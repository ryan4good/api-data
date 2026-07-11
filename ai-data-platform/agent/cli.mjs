#!/usr/bin/env node
import { executePlan } from "../executor/executor.mjs";
import { parseIntent } from "./parser.mjs";

function printResult(result) {
  console.log(`识别场景：${result.sceneName}`);
  console.log(`目标阶段：${result.targetStage}`);
  console.log("");

  for (const log of result.logs) {
    const label = log.status === "SUCCESS" ? "成功" : log.status === "SKIPPED" ? "跳过" : "失败";
    const detail = log.status === "FAILED" ? ` - ${log.error}` : "";
    console.log(`[${label}] ${log.name}${detail}`);
  }

  console.log("");
  console.log("结果：");
  console.log(`success: ${result.success}`);
  if (result.context.orderNo) console.log(`orderNo: ${result.context.orderNo}`);
  if (result.context.outboundNo) console.log(`outboundNo: ${result.context.outboundNo}`);
  if (result.context.shipmentNo) console.log(`shipmentNo: ${result.context.shipmentNo}`);
  if (result.context.skuCode) console.log(`skuCode: ${result.context.skuCode}`);
  if (result.error) console.log(`error: ${result.error}`);
}

async function main() {
  const [command, ...rest] = process.argv.slice(2);
  if (command !== "run") {
    console.log('Usage: npm run run -- "我要造一笔出库单，做到分配库存完成"');
    process.exitCode = command ? 1 : 0;
    return;
  }

  const text = rest.join(" ").trim();
  if (!text) {
    throw new Error("Please provide a natural language data request.");
  }

  const plan = parseIntent(text);
  const result = await executePlan(plan);
  printResult(result);
  process.exitCode = result.success ? 0 : 2;
}

main().catch((error) => {
  console.error(error.message);
  process.exitCode = 1;
});
