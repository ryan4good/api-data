# MVP 验收用例

## 运行方式

启动 mock 系统：

```bash
npm run start:mock
```

执行命令：

```bash
npm run run -- "我要造一笔出库单，做到分配库存完成"
```

## 用例

1. 我要造一笔出库单
   - 期望：创建 WMS 出库单，返回 `outboundNo`。

2. 我要造一笔出库单，做到分配库存完成
   - 期望：执行到 `ALLOCATED`，返回 `outboundNo` 和 `skuCode`。

3. 我要造一笔数量10的出库单，做到分配库存完成
   - 期望：参数 `qty=10`，状态 `ALLOCATED`。

4. 我要造一笔已审核但未分配库存的出库单
   - 期望：状态 `AUDITED`，不执行库存分配。

5. 我要造一笔出库完成的单据
   - 期望：执行到 `SHIPPED`。

6. 我要造一笔库存不足的出库单
   - 期望：分配库存步骤失败，错误为 `inventory not enough`。

7. 我要造一笔 SKU:SKU_FIXED_001 的出库单，做到分配库存完成
   - 期望：跳过创建 SKU，使用指定 SKU，并自动补库存。

8. 我要造一笔销售订单
   - 期望：创建 OMS 销售订单，返回 `orderNo`。

9. 我要造一笔销售订单，并让出库单分配库存完成
   - 期望：创建 OMS 订单、WMS 出库单，执行到 WMS `ALLOCATED`。

10. 我要造一笔已创建运输单的订单
    - 期望：执行 OMS -> WMS -> TMS，返回 `shipmentNo`。

11. 扫描 sample Go 代码
    - 命令：`npm run scan`
    - 期望：生成 `ai-data-platform/scanner/output/candidate-scenes.json`，包含 WMS 和 OMS->WMS 候选场景。
