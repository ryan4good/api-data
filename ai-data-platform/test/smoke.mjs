const baseUrl = process.argv[2] || process.env.BASE_URL || "http://127.0.0.1:3120";

async function request(path, options = {}) {
  const response = await fetch(`${baseUrl}${path}`, {
    headers: { "content-type": "application/json" },
    ...options
  });
  const text = await response.text();
  let body;
  try {
    body = text ? JSON.parse(text) : {};
  } catch {
    body = { raw: text };
  }
  if (!response.ok) {
    throw new Error(`${options.method || "GET"} ${path} failed: ${response.status} ${JSON.stringify(body)}`);
  }
  return body;
}

function assert(condition, message) {
  if (!condition) throw new Error(message);
}

const status = await request("/api/status");
assert(status.ok, "status should be ok");
assert(status.scenarioCount >= 2, "scenario count should be >= 2");

const scenarios = await request("/api/scenarios");
assert(scenarios.some((item) => item.scene === "wms_outbound_allocated"), "wms scenario missing");
assert(scenarios.some((item) => item.scene === "oms_order_to_wms_allocated"), "oms->wms scenario missing");

const scan = await request("/api/scan", { method: "POST" });
assert(scan.candidateScenes.length >= 2, "candidate scenes missing");
assert(scan.nodes.length >= 10, "scan nodes too few");

const confirmed = await request("/api/candidates/candidate_oms_order_to_wms_allocated/confirm", { method: "POST" });
assert(confirmed.scene === "confirmed_oms_order_to_wms_allocated", "confirm candidate failed");

const run = await request("/api/execute", {
  method: "POST",
  body: JSON.stringify({
    plan: {
      scene: "confirmed_oms_order_to_wms_allocated",
      targetStage: "INVENTORY_ALLOCATED",
      params: {
        warehouseCode: "WH_TEST",
        ownerCode: "OWNER_TEST",
        customerCode: "CUST_TEST",
        qty: 2
      }
    }
  })
});
assert(run.success, "execution should succeed");
assert(run.context.orderNo, "orderNo missing");
assert(run.context.outboundNo, "outboundNo missing");
assert(run.logs.length >= 7, "execution logs too few");

const logs = await request("/api/logs");
assert(logs.length > 0, "logs missing");
const detail = await request(`/api/logs/${encodeURIComponent(logs[0].file)}`);
assert(detail.logs?.length > 0, "log detail missing steps");

console.log(JSON.stringify({
  ok: true,
  baseUrl,
  scenarios: scenarios.length,
  candidates: scan.candidateScenes.length,
  nodes: scan.nodes.length,
  orderNo: run.context.orderNo,
  outboundNo: run.context.outboundNo,
  logSteps: detail.logs.length
}, null, 2));
