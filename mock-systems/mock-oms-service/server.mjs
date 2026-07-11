import { createServer } from "../lib/http-utils.mjs";

const state = {
  orderSeq: 1,
  orders: new Map()
};

function ok(data) {
  return { success: true, data };
}

function nextOrderNo() {
  return `SO_AUTO_${String(state.orderSeq++).padStart(4, "0")}`;
}

export function startOms(port = 3101) {
  return createServer({
    name: "mock-oms-service",
    port,
    routes: [
      {
        method: "POST",
        pattern: /^\/api\/order\/create$/,
        handler: ({ body }) => {
          const orderNo = body.orderNo || nextOrderNo();
          const order = {
            orderNo,
            customerCode: body.customerCode || "CUST_TEST",
            skuCode: body.skuCode,
            qty: Number(body.qty ?? 1),
            status: "ORDER_CREATED",
            outboundNo: ""
          };
          if (!order.skuCode) {
            return { statusCode: 400, body: { success: false, error: "skuCode is required" } };
          }
          state.orders.set(orderNo, order);
          return ok(order);
        }
      },
      {
        method: "POST",
        pattern: /^\/api\/order\/mark-wms-pushed$/,
        handler: ({ body }) => {
          const order = state.orders.get(body.orderNo);
          if (!order) return { statusCode: 404, body: { success: false, error: "order not found" } };
          order.status = "WMS_PUSHED";
          order.outboundNo = body.outboundNo || order.outboundNo;
          return ok(order);
        }
      },
      {
        method: "GET",
        pattern: /^\/api\/order\/detail\/(?<orderNo>[^/]+)$/,
        handler: ({ params }) => {
          const order = state.orders.get(params.orderNo);
          if (!order) return { statusCode: 404, body: { success: false, error: "order not found" } };
          return ok(order);
        }
      }
    ]
  });
}

if (import.meta.url === `file://${process.argv[1]}`) {
  startOms();
}
