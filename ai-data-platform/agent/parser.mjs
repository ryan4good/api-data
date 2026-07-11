export function parseIntent(text) {
  const normalized = text.trim();
  const isOrder = /销售订单|订单|oms|order/i.test(normalized);
  const scene = isOrder ? "oms_order_to_wms_allocated" : "wms_outbound_allocated";
  const params = {};

  const qtyMatch = normalized.match(/(?:数量|qty|件数)\s*([0-9]+)/i) || normalized.match(/([0-9]+)\s*(?:件|个|pcs)/i);
  if (qtyMatch) {
    params.qty = Number(qtyMatch[1]);
  }

  const skuMatch = normalized.match(/(?:SKU|sku)\s*[:：]?\s*([A-Za-z0-9_-]+)/);
  if (skuMatch) {
    params.skuCode = skuMatch[1];
  }

  if (/不要.*补库存|不.*自动补库存|库存不足/.test(normalized)) {
    params.noAutoInventory = true;
    params.inventoryQty = 0;
  }

  if (/华东/.test(normalized)) {
    params.warehouseCode = "WH_EAST";
  }

  let targetStage;
  if (/库存不足/.test(normalized)) {
    targetStage = isOrder ? "INVENTORY_ALLOCATED" : "ALLOCATED";
  } else if (/未分配/.test(normalized)) {
    targetStage = isOrder ? "OUTBOUND_CREATED" : "AUDITED";
  } else if (/运输单|运单/.test(normalized)) {
    targetStage = isOrder ? "SHIPMENT_CREATED" : "SHIPPED";
  } else if (/出库完成|已出库|发货完成/.test(normalized)) {
    targetStage = isOrder ? "SHIPMENT_CREATED" : "SHIPPED";
  } else if (/分配库存|库存.*分配|已分配/.test(normalized)) {
    targetStage = isOrder ? "INVENTORY_ALLOCATED" : "ALLOCATED";
  } else if (/审核/.test(normalized)) {
    targetStage = "AUDITED";
  } else if (/生成出库单|出库单已创建/.test(normalized)) {
    targetStage = isOrder ? "OUTBOUND_CREATED" : "CREATED";
  } else if (/创建|造一笔|来一笔/.test(normalized)) {
    targetStage = isOrder ? "ORDER_CREATED" : "CREATED";
  }

  return {
    scene,
    targetStage,
    params,
    rawText: text
  };
}
