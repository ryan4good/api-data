import { createServer } from "../lib/http-utils.mjs";

const state = {
  skuSeq: 1,
  outboundSeq: 1,
  skus: new Map(),
  inventory: new Map(),
  outbounds: new Map()
};

function nextCode(prefix, seq) {
  return `${prefix}_${String(seq).padStart(4, "0")}`;
}

function stockKey(warehouseCode, skuCode) {
  return `${warehouseCode}:${skuCode}`;
}

function ok(data) {
  return { success: true, data };
}

export function startWms(port = 3102) {
  return createServer({
    name: "mock-wms-service",
    port,
    routes: [
      {
        method: "POST",
        pattern: /^\/api\/sku\/create$/,
        handler: ({ body }) => {
          const skuCode = body.skuCode || nextCode("SKU_AUTO", state.skuSeq++);
          const sku = { skuCode, skuName: body.skuName || "Auto SKU", skuType: body.skuType || "NORMAL" };
          state.skus.set(skuCode, sku);
          return ok(sku);
        }
      },
      {
        method: "POST",
        pattern: /^\/api\/inventory\/add$/,
        handler: ({ body }) => {
          const warehouseCode = body.warehouseCode || "WH_TEST";
          const skuCode = body.skuCode;
          if (!skuCode) {
            return { statusCode: 400, body: { success: false, error: "skuCode is required" } };
          }
          const key = stockKey(warehouseCode, skuCode);
          const current = state.inventory.get(key) || 0;
          const qty = Number(body.qty ?? 100);
          state.inventory.set(key, current + qty);
          return ok({ warehouseCode, skuCode, availableQty: state.inventory.get(key) });
        }
      },
      {
        method: "POST",
        pattern: /^\/api\/outbound\/create$/,
        handler: ({ body }) => {
          const outboundNo = nextCode("OUT_AUTO", state.outboundSeq++);
          const order = {
            outboundNo,
            sourceOrderNo: body.sourceOrderNo || "",
            warehouseCode: body.warehouseCode || "WH_TEST",
            ownerCode: body.ownerCode || "OWNER_TEST",
            skuCode: body.skuCode,
            qty: Number(body.qty ?? 1),
            status: "CREATED"
          };
          if (!order.skuCode) {
            return { statusCode: 400, body: { success: false, error: "skuCode is required" } };
          }
          state.outbounds.set(outboundNo, order);
          return ok(order);
        }
      },
      {
        method: "POST",
        pattern: /^\/api\/outbound\/audit$/,
        handler: ({ body }) => {
          const order = state.outbounds.get(body.outboundNo);
          if (!order) return { statusCode: 404, body: { success: false, error: "outbound not found" } };
          order.status = "AUDITED";
          return ok(order);
        }
      },
      {
        method: "POST",
        pattern: /^\/api\/outbound\/allocate$/,
        handler: ({ body }) => {
          const order = state.outbounds.get(body.outboundNo);
          if (!order) return { statusCode: 404, body: { success: false, error: "outbound not found" } };
          if (order.status !== "AUDITED") {
            return { statusCode: 409, body: { success: false, error: `cannot allocate from ${order.status}` } };
          }
          const key = stockKey(order.warehouseCode, order.skuCode);
          const available = state.inventory.get(key) || 0;
          if (available < order.qty) {
            order.status = "ALLOCATE_FAILED";
            return {
              statusCode: 409,
              body: { success: false, error: "inventory not enough", data: { availableQty: available, needQty: order.qty } }
            };
          }
          state.inventory.set(key, available - order.qty);
          order.status = "ALLOCATED";
          return ok({ ...order, allocatedQty: order.qty });
        }
      },
      {
        method: "POST",
        pattern: /^\/api\/outbound\/ship$/,
        handler: ({ body }) => {
          const order = state.outbounds.get(body.outboundNo);
          if (!order) return { statusCode: 404, body: { success: false, error: "outbound not found" } };
          if (order.status !== "ALLOCATED") {
            return { statusCode: 409, body: { success: false, error: `cannot ship from ${order.status}` } };
          }
          order.status = "SHIPPED";
          return ok(order);
        }
      },
      {
        method: "GET",
        pattern: /^\/api\/outbound\/detail\/(?<outboundNo>[^/]+)$/,
        handler: ({ params }) => {
          const order = state.outbounds.get(params.outboundNo);
          if (!order) return { statusCode: 404, body: { success: false, error: "outbound not found" } };
          return ok(order);
        }
      },
      {
        method: "GET",
        pattern: /^\/api\/outbound\/by-source-order\/(?<sourceOrderNo>[^/]+)$/,
        handler: ({ params }) => {
          const order = [...state.outbounds.values()].find((item) => item.sourceOrderNo === params.sourceOrderNo);
          if (!order) return { statusCode: 404, body: { success: false, error: "outbound not found" } };
          return ok(order);
        }
      }
    ]
  });
}

if (import.meta.url === `file://${process.argv[1]}`) {
  startWms();
}
