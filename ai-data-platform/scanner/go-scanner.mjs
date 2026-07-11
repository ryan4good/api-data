import { readdir, readFile, writeFile, mkdir, stat } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { rootDir, platformDir, readJsonDir } from "../lib/fs.mjs";

async function walk(dir) {
  const entries = await readdir(dir, { withFileTypes: true });
  const files = [];
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...await walk(fullPath));
    } else if (entry.isFile() && entry.name.endsWith(".go")) {
      files.push(fullPath);
    }
  }
  return files;
}

function lineOf(content, index) {
  return content.slice(0, index).split(/\r?\n/).length;
}

function scanRoutes(content, file, system) {
  const routes = [];
  const pattern = /r\.(GET|POST|PUT|DELETE|PATCH)\("([^"]+)",\s*([^)]+)\)/g;
  let match;
  while ((match = pattern.exec(content))) {
    routes.push({
      type: "api",
      system,
      method: match[1],
      path: match[2],
      handler: match[3].trim(),
      evidence: { file, line: lineOf(content, match.index) }
    });
  }
  return routes;
}

function scanStatuses(content, file, system) {
  const statuses = [];
  const pattern = /([A-Za-z0-9_]*Status[A-Za-z0-9_]*)\s*=\s*"([^"]+)"/g;
  let match;
  while ((match = pattern.exec(content))) {
    statuses.push({
      type: "status",
      system,
      symbol: match[1],
      value: match[2],
      evidence: { file, line: lineOf(content, match.index) }
    });
  }
  return statuses;
}

function scanServiceTransitions(content, file, system) {
  const transitions = [];
  const functionPattern = /func\s+\([^)]*\)\s+([A-Za-z0-9_]+)\([^)]*\)[\s\S]*?\n}/g;
  let fn;
  while ((fn = functionPattern.exec(content))) {
    const body = fn[0];
    const statusPattern = /Status\s*=\s*([A-Za-z0-9_]+)/g;
    let status;
    while ((status = statusPattern.exec(body))) {
      transitions.push({
        type: "transition",
        system,
        function: fn[1],
        statusSymbol: status[1],
        evidence: { file, line: lineOf(content, fn.index + status.index) }
      });
    }
  }
  return transitions;
}

function inferCandidateScenes(nodes) {
  const apiNodes = nodes.filter((node) => node.type === "api");
  const statusNodes = nodes.filter((node) => node.type === "status");

  const wmsApis = apiNodes.filter((node) => node.system === "wms");
  const omsApis = apiNodes.filter((node) => node.system === "oms");

  const scenes = [];
  if (wmsApis.some((api) => api.path.includes("/outbound/allocate"))) {
    scenes.push({
      scene: "candidate_wms_outbound_allocated",
      name: "候选：出库单分配库存完成",
      confidence: 0.82,
      systems: ["wms"],
      evidence: wmsApis
        .filter((api) => /sku|inventory|outbound/.test(api.path))
        .map((api) => ({ stepHint: api.handler, api: `${api.method} ${api.path}`, evidence: api.evidence })),
      statuses: statusNodes.filter((node) => node.system === "wms").map((node) => node.value)
    });
  }

  if (omsApis.some((api) => api.path.includes("/order/create")) && wmsApis.some((api) => api.path.includes("/outbound/create"))) {
    scenes.push({
      scene: "candidate_oms_order_to_wms_allocated",
      name: "候选：销售订单生成出库单并分配库存",
      confidence: 0.76,
      systems: ["oms", "wms"],
      evidence: [
        ...omsApis.map((api) => ({ stepHint: api.handler, api: `${api.method} ${api.path}`, evidence: api.evidence })),
        ...wmsApis
          .filter((api) => /outbound\/create|outbound\/allocate/.test(api.path))
          .map((api) => ({ stepHint: api.handler, api: `${api.method} ${api.path}`, evidence: api.evidence }))
      ],
      statuses: statusNodes.filter((node) => ["oms", "wms"].includes(node.system)).map((node) => `${node.system}:${node.value}`)
    });
  }

  return scenes;
}

export async function scanCodebase() {
  const systems = await readJsonDir(path.join(platformDir, "system-registry"));
  const nodes = [];

  for (const system of systems) {
    if (system.language !== "go" || !system.repo) continue;
    const repoPath = path.join(rootDir, system.repo);
    try {
      await stat(repoPath);
    } catch {
      nodes.push({
        type: "scan_warning",
        system: system.code,
        message: `repo not found: ${system.repo}`
      });
      continue;
    }
    const files = await walk(repoPath);
    for (const file of files) {
      const content = await readFile(file, "utf8");
      const relativeFile = path.relative(rootDir, file).replaceAll("\\", "/");
      nodes.push(...scanRoutes(content, relativeFile, system.code));
      nodes.push(...scanStatuses(content, relativeFile, system.code));
      nodes.push(...scanServiceTransitions(content, relativeFile, system.code));
    }
  }

  const candidateScenes = inferCandidateScenes(nodes);
  const output = {
    scannedAt: new Date().toISOString(),
    nodes,
    candidateScenes
  };

  const outputPath = path.join(platformDir, "scanner", "output", "candidate-scenes.json");
  await mkdir(path.dirname(outputPath), { recursive: true });
  await writeFile(outputPath, `${JSON.stringify(output, null, 2)}\n`, "utf8");
  return { output, outputPath };
}

async function main() {
  const { output, outputPath } = await scanCodebase();

  console.log(`Scanned nodes: ${output.nodes.length}`);
  console.log(`Candidate scenes: ${output.candidateScenes.length}`);
  for (const scene of output.candidateScenes) {
    console.log(`- ${scene.name} (${scene.confidence})`);
  }
  console.log(`Output: ${path.relative(rootDir, outputPath)}`);
}

if (process.argv[1] && fileURLToPath(import.meta.url) === path.resolve(process.argv[1])) {
  main().catch((error) => {
    console.error(error.message);
    process.exit(1);
  });
}
