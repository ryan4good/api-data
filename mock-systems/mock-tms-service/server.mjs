import { createServer } from "../lib/http-utils.mjs";

const state = {
  shipmentSeq: 1,
  shipments: new Map()
};

function ok(data) {
  return { success: true, data };
}

export function startTms(port = 3103) {
  return createServer({
    name: "mock-tms-service",
    port,
    routes: [
      {
        method: "POST",
        pattern: /^\/api\/shipment\/create$/,
        handler: ({ body }) => {
          const shipmentNo = `SHIP_AUTO_${String(state.shipmentSeq++).padStart(4, "0")}`;
          const shipment = {
            shipmentNo,
            orderNo: body.orderNo,
            outboundNo: body.outboundNo,
            status: "SHIPMENT_CREATED"
          };
          state.shipments.set(shipmentNo, shipment);
          return ok(shipment);
        }
      },
      {
        method: "GET",
        pattern: /^\/api\/shipment\/detail\/(?<shipmentNo>[^/]+)$/,
        handler: ({ params }) => {
          const shipment = state.shipments.get(params.shipmentNo);
          if (!shipment) return { statusCode: 404, body: { success: false, error: "shipment not found" } };
          return ok(shipment);
        }
      }
    ]
  });
}

if (import.meta.url === `file://${process.argv[1]}`) {
  startTms();
}
