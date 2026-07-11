import { startOms } from "../mock-systems/mock-oms-service/server.mjs";
import { startWms } from "../mock-systems/mock-wms-service/server.mjs";
import { startTms } from "../mock-systems/mock-tms-service/server.mjs";
import { executePlan } from "./executor/executor.mjs";
import { parseIntent } from "./agent/parser.mjs";

const servers = [startOms(), startWms(), startTms()];

async function run(text) {
  console.log(`\n> ${text}`);
  const result = await executePlan(parseIntent(text));
  console.log(`${result.success ? "SUCCESS" : "FAILED"} ${result.sceneName} ${result.targetStage}`);
  console.log(result.context);
  if (result.error) console.log(result.error);
}

try {
  await new Promise((resolve) => setTimeout(resolve, 200));
  await run("我要造一笔出库单，做到分配库存完成");
  await run("我要造一笔销售订单，并让出库单分配库存完成");
} finally {
  for (const server of servers) {
    server.close();
  }
}
