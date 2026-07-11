import path from "node:path";
import { performance } from "node:perf_hooks";
import { platformDir, writeJson } from "../lib/fs.mjs";
import { getPath, isTruthyTemplate, renderValue } from "./template.mjs";
import { loadConnectors, loadScenario } from "./loaders.mjs";

function selectSteps(scenario, targetStage) {
  const stage = scenario.stages.find((item) => item.code === targetStage);
  if (!stage) {
    throw new Error(`Target stage ${targetStage} is not available for ${scenario.scene}`);
  }
  const stopIndex = scenario.steps.findIndex((step) => step.code === stage.stopAfterStep);
  if (stopIndex < 0) {
    throw new Error(`Stop step ${stage.stopAfterStep} is missing in ${scenario.scene}`);
  }
  const mainSteps = scenario.steps.slice(0, stopIndex + 1);
  const verifySteps = scenario.steps.filter((step) => step.code.startsWith("verify_"));
  return [...mainSteps, ...verifySteps.filter((step) => !mainSteps.some((main) => main.code === step.code))];
}

function extractOutputs(outputs = {}, response) {
  const extracted = {};
  for (const [name, expression] of Object.entries(outputs)) {
    extracted[name] = getPath(response, expression);
  }
  return extracted;
}

function assertVerify(verify = {}, response, scope) {
  const errors = [];
  for (const [expression, expectedTemplate] of Object.entries(verify)) {
    const actual = getPath(response, expression);
    const expected = renderValue(expectedTemplate, scope);
    if (String(actual) !== String(expected)) {
      errors.push(`${expression} expected ${expected}, got ${actual}`);
    }
  }
  if (errors.length) {
    throw new Error(`Verification failed: ${errors.join("; ")}`);
  }
}

function ensureSafe(connector, method, pathValue) {
  const key = `${method} ${pathValue}`;
  if (connector.safety?.blacklist?.includes(key)) {
    throw new Error(`Blocked by safety blacklist: ${key}`);
  }
}

async function callHttp({ connector, method, pathValue, body }) {
  ensureSafe(connector, method, pathValue);
  const url = `${connector.baseUrl}${pathValue}`;
  const init = { method, headers: { "content-type": "application/json" } };
  if (method !== "GET") {
    init.body = JSON.stringify(body ?? {});
  }
  const response = await fetch(url, init);
  const text = await response.text();
  let json;
  try {
    json = text ? JSON.parse(text) : {};
  } catch {
    json = { raw: text };
  }
  if (!response.ok || json.success === false) {
    const reason = json.error || response.statusText;
    const error = new Error(reason);
    error.response = json;
    error.statusCode = response.status;
    throw error;
  }
  return json;
}

export async function executePlan(plan, options = {}) {
  const env = options.env || "test";
  const scenario = await loadScenario(plan.scene);
  const connectors = await loadConnectors(env);
  const params = { ...scenario.defaultParams, ...plan.params };
  const targetStage = plan.targetStage || scenario.defaultTargetStage;
  const context = {};
  if (params.skuCode) {
    context.skuCode = params.skuCode;
  }
  const logs = [];
  const startedAt = new Date().toISOString();
  const steps = selectSteps(scenario, targetStage);

  for (const step of steps) {
    const scope = { params, context, targetStage, verifyOutboundStatus: "ALLOCATED" };
    if (step.skipWhen && isTruthyTemplate(step.skipWhen, scope)) {
      logs.push({ step: step.code, name: step.name, status: "SKIPPED", reason: "skipWhen matched" });
      continue;
    }

    const connector = connectors.get(step.system);
    if (!connector) {
      throw new Error(`Connector not found for system ${step.system} in ${env}`);
    }

    const pathValue = renderValue(step.path, scope);
    const body = renderValue(step.body ?? {}, scope);
    const started = performance.now();
    const entry = {
      step: step.code,
      name: step.name,
      system: step.system,
      request: { method: step.method, path: pathValue, body },
      status: "RUNNING"
    };
    logs.push(entry);

    try {
      const response = await callHttp({ connector, method: step.method, pathValue, body });
      const durationMs = Math.round(performance.now() - started);
      Object.assign(context, extractOutputs(step.outputs, response));
      assertVerify(step.verify, response, { params, context, targetStage, verifyOutboundStatus: "ALLOCATED" });
      Object.assign(entry, { status: "SUCCESS", durationMs, response });
    } catch (error) {
      const durationMs = Math.round(performance.now() - started);
      Object.assign(entry, {
        status: "FAILED",
        durationMs,
        error: error.message,
        response: error.response
      });
      const result = {
        success: false,
        scene: scenario.scene,
        sceneName: scenario.name,
        targetStage,
        params,
        context,
        logs,
        startedAt,
        finishedAt: new Date().toISOString(),
        error: error.message
      };
      await persistResult(result);
      return result;
    }
  }

  const result = {
    success: true,
    scene: scenario.scene,
    sceneName: scenario.name,
    targetStage,
    params,
    context,
    logs,
    startedAt,
    finishedAt: new Date().toISOString()
  };
  await persistResult(result);
  return result;
}

async function persistResult(result) {
  const stamp = new Date().toISOString().replace(/[:.]/g, "-");
  const fileName = `${stamp}_${result.scene}_${result.success ? "success" : "failed"}.json`;
  await writeJson(path.join(platformDir, "logs", fileName), result);
}
