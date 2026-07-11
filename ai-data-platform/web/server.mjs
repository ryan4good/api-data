import http from "node:http";
import path from "node:path";
import { readFile, readdir } from "node:fs/promises";
import { startOms } from "../../mock-systems/mock-oms-service/server.mjs";
import { startWms } from "../../mock-systems/mock-wms-service/server.mjs";
import { startTms } from "../../mock-systems/mock-tms-service/server.mjs";
import { parseIntent } from "../agent/parser.mjs";
import { executePlan } from "../executor/executor.mjs";
import { platformDir, readJson, readJsonDir, writeJson } from "../lib/fs.mjs";
import { loadScenario } from "../executor/loaders.mjs";
import { scanCodebase } from "../scanner/go-scanner.mjs";

const port = Number(process.env.PORT || 3120);
const withMock = process.argv.includes("--with-mock");

if (withMock) {
  startOms();
  startWms();
  startTms();
}

function send(res, statusCode, body, contentType = "application/json; charset=utf-8") {
  res.writeHead(statusCode, {
    "content-type": contentType,
    "cache-control": "no-store",
    "access-control-allow-origin": "*",
    "access-control-allow-methods": "GET,POST,OPTIONS",
    "access-control-allow-headers": "content-type"
  });
  if (typeof body === "string" || body instanceof Uint8Array) {
    res.end(body);
  } else {
    res.end(JSON.stringify(body, null, 2));
  }
}

function contentTypeFor(filePath) {
  if (filePath.endsWith(".css")) return "text/css; charset=utf-8";
  if (filePath.endsWith(".js")) return "application/javascript; charset=utf-8";
  if (filePath.endsWith(".json")) return "application/json; charset=utf-8";
  return "application/octet-stream";
}

async function bodyJson(req) {
  let raw = "";
  for await (const chunk of req) raw += chunk;
  return raw ? JSON.parse(raw) : {};
}

function publicPlan(scenario, targetStage) {
  const stage = scenario.stages.find((item) => item.code === targetStage);
  const stopIndex = scenario.steps.findIndex((step) => step.code === stage?.stopAfterStep);
  const main = scenario.steps.slice(0, stopIndex + 1);
  const verify = scenario.steps.filter((step) => step.code.startsWith("verify_"));
  const steps = [...main, ...verify.filter((step) => !main.some((item) => item.code === step.code))];
  return {
    scene: scenario.scene,
    sceneName: scenario.name,
    targetStage,
    systems: scenario.systems,
    steps: steps.map((step) => ({
      code: step.code,
      name: step.name,
      system: step.system,
      method: step.method,
      path: step.path
    }))
  };
}

async function scenarioSummaries() {
  const scenarios = await readJsonDir(path.join(platformDir, "scenarios"));
  return scenarios.map((scenario) => ({
    scene: scenario.scene,
    name: scenario.name,
    description: scenario.description,
    systems: scenario.systems,
    defaultTargetStage: scenario.defaultTargetStage,
    stages: scenario.stages,
    steps: scenario.steps?.map((step) => ({
      code: step.code,
      name: step.name,
      system: step.system,
      method: step.method,
      path: step.path
    })),
    sourceCandidate: scenario.sourceCandidate,
    confirmedAt: scenario.confirmedAt
  }));
}

function redactedConnector(connector) {
  return {
    ...connector,
    auth: connector.auth?.type ? { type: connector.auth.type } : { type: "unknown" }
  };
}

async function loadCandidates() {
  const outputPath = path.join(platformDir, "scanner", "output", "candidate-scenes.json");
  try {
    return await readJson(outputPath);
  } catch {
    return { scannedAt: null, nodes: [], candidateScenes: [] };
  }
}

async function confirmCandidate(candidateId) {
  const scanOutput = await loadCandidates();
  const candidate = scanOutput.candidateScenes.find((item) => item.scene === candidateId);
  if (!candidate) {
    const error = new Error(`candidate not found: ${candidateId}`);
    error.statusCode = 404;
    throw error;
  }

  const templateScene = candidateId.includes("oms")
    ? "oms_order_to_wms_allocated"
    : "wms_outbound_allocated";
  const template = await loadScenario(templateScene);
  const sceneId = candidateId.replace(/^candidate_/, "confirmed_");
  const confirmed = {
    ...template,
    scene: sceneId,
    name: candidate.name.replace(/^候选：/, "已确认："),
    description: `${template.description} 该场景由代码扫描候选场景确认生成。`,
    sourceCandidate: candidate.scene,
    confidence: candidate.confidence,
    evidence: candidate.evidence,
    statuses: candidate.statuses,
    confirmedAt: new Date().toISOString()
  };

  await writeJson(path.join(platformDir, "scenarios", `${sceneId}.json`), confirmed);
  return confirmed;
}

async function handleApi(req, res, url) {
  if (req.method === "GET" && url.pathname === "/api/scenarios") {
    send(res, 200, await scenarioSummaries());
    return;
  }

  if (req.method === "GET" && url.pathname.startsWith("/api/scenarios/")) {
    const scene = decodeURIComponent(url.pathname.slice("/api/scenarios/".length));
    send(res, 200, await loadScenario(scene));
    return;
  }

  if (req.method === "GET" && url.pathname === "/api/systems") {
    const systems = await readJsonDir(path.join(platformDir, "system-registry"));
    send(res, 200, systems);
    return;
  }

  if (req.method === "GET" && url.pathname === "/api/connectors") {
    const connectors = await readJsonDir(path.join(platformDir, "connectors"));
    send(res, 200, connectors.map(redactedConnector));
    return;
  }

  if (req.method === "GET" && url.pathname === "/api/status") {
    const [scenarios, systems, connectors, scanOutput] = await Promise.all([
      scenarioSummaries(),
      readJsonDir(path.join(platformDir, "system-registry")),
      readJsonDir(path.join(platformDir, "connectors")),
      loadCandidates()
    ]);
    send(res, 200, {
      ok: true,
      env: process.env.NODE_ENV || "development",
      port,
      withMock,
      scenarioCount: scenarios.length,
      systemCount: systems.length,
      connectorCount: connectors.length,
      candidateCount: scanOutput.candidateScenes.length,
      nodeCount: scanOutput.nodes.length,
      scannedAt: scanOutput.scannedAt,
      uptimeSeconds: Math.round(process.uptime())
    });
    return;
  }

  if (req.method === "POST" && url.pathname === "/api/preview") {
    const { text, params = {}, targetStage } = await bodyJson(req);
    const intent = parseIntent(text || "");
    intent.params = { ...intent.params, ...params };
    if (targetStage) {
      intent.targetStage = targetStage;
    }
    const scenario = await loadScenario(intent.scene);
    send(res, 200, { intent, plan: publicPlan(scenario, intent.targetStage || scenario.defaultTargetStage) });
    return;
  }

  if (req.method === "POST" && url.pathname === "/api/execute") {
    const payload = await bodyJson(req);
    const intent = payload.plan || parseIntent(payload.text || "");
    intent.params = { ...intent.params, ...(payload.params || {}) };
    if (payload.targetStage) {
      intent.targetStage = payload.targetStage;
    }
    const result = await executePlan(intent);
    send(res, result.success ? 200 : 409, result);
    return;
  }

  if (req.method === "GET" && url.pathname === "/api/candidates") {
    send(res, 200, await loadCandidates());
    return;
  }

  if (req.method === "POST" && url.pathname === "/api/scan") {
    const { output } = await scanCodebase();
    send(res, 200, output);
    return;
  }

  if (req.method === "POST" && url.pathname.startsWith("/api/candidates/") && url.pathname.endsWith("/confirm")) {
    const candidateId = decodeURIComponent(url.pathname.slice("/api/candidates/".length, -"/confirm".length));
    const confirmed = await confirmCandidate(candidateId);
    send(res, 200, confirmed);
    return;
  }

  if (req.method === "GET" && url.pathname === "/api/logs") {
    const logDir = path.join(platformDir, "logs");
    const files = (await readdir(logDir).catch(() => []))
      .filter((file) => file.endsWith(".json"))
      .sort()
      .reverse()
      .slice(0, 50);
    const logs = [];
    for (const file of files) {
      const item = await readJson(path.join(logDir, file));
      logs.push({
        file,
        success: item.success,
        scene: item.scene,
        sceneName: item.sceneName,
        targetStage: item.targetStage,
        context: item.context,
        error: item.error,
        finishedAt: item.finishedAt
      });
    }
    send(res, 200, logs);
    return;
  }

  if (req.method === "GET" && url.pathname.startsWith("/api/logs/")) {
    const file = decodeURIComponent(url.pathname.slice("/api/logs/".length));
    if (!/^[A-Za-z0-9_.-]+\.json$/.test(file)) {
      send(res, 400, { success: false, error: "invalid log file" });
      return;
    }
    send(res, 200, await readJson(path.join(platformDir, "logs", file)));
    return;
  }

  send(res, 404, { success: false, error: "api not found" });
}

const server = http.createServer(async (req, res) => {
  try {
    if (req.method === "OPTIONS") {
      send(res, 200, { ok: true });
      return;
    }

    const url = new URL(req.url, `http://localhost:${port}`);
    if (url.pathname.startsWith("/api/")) {
      await handleApi(req, res, url);
      return;
    }

    if (url.pathname.startsWith("/vendor/")) {
      const file = decodeURIComponent(url.pathname.slice("/vendor/".length));
      if (!/^[A-Za-z0-9_.-]+$/.test(file)) {
        send(res, 400, { success: false, error: "invalid asset path" });
        return;
      }
      const asset = await readFile(path.join(platformDir, "web", "vendor", file));
      send(res, 200, asset, contentTypeFor(file));
      return;
    }

    const html = await readFile(path.join(platformDir, "web", "prototype.html"), "utf8");
    send(res, 200, html, "text/html; charset=utf-8");
  } catch (error) {
    send(res, error.statusCode || 500, { success: false, error: error.message });
  }
});

server.listen(port, "0.0.0.0", () => {
  console.log(`AI data console listening on http://0.0.0.0:${port}`);
  if (withMock) {
    console.log("Mock systems are running with the console.");
  }
});
