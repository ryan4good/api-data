import { startOms } from "./mock-oms-service/server.mjs";
import { startWms } from "./mock-wms-service/server.mjs";
import { startTms } from "./mock-tms-service/server.mjs";

startOms();
startWms();
startTms();

console.log("Mock systems are ready. Press Ctrl+C to stop.");
