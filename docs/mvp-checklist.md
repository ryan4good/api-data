# MVP 验收清单

访问入口：

```text
http://124.223.28.126:8080/ai-data/
```

## 控制台

- 场景列表能展示正式场景。
- 候选场景能展示置信度、系统、证据数量。
- 点击“扫描”能刷新候选场景。
- 点击“确认为正式场景”能把候选场景保存进正式场景列表。
- 系统资产能展示系统、连接器和运行状态。

## 执行

- 输入“我要造一笔出库单，做到分配库存完成”，点击“解析意图”，能生成 WMS 执行计划。
- 输入“我要造一笔销售订单，并让出库单分配库存完成”，能生成 OMS -> WMS 跨系统执行计划。
- 修改 `qty` 后执行，结果能按新参数运行。
- 执行成功后能返回 `orderNo`、`outboundNo` 或 `skuCode`。
- 库存不足场景能失败并展示失败步骤。

## 日志

- 执行日志列表能显示最近运行记录。
- 点击日志能加载完整步骤详情。
- 步骤详情能展开请求、响应、耗时和错误。
- “复制结果”能复制当前结果 JSON。
- “重新执行”能按当前计划再次运行。

## 后端 API

```text
GET  /ai-data/api/status
GET  /ai-data/api/scenarios
GET  /ai-data/api/scenarios/:scene
GET  /ai-data/api/systems
GET  /ai-data/api/connectors
GET  /ai-data/api/candidates
POST /ai-data/api/scan
POST /ai-data/api/candidates/:scene/confirm
POST /ai-data/api/preview
POST /ai-data/api/execute
GET  /ai-data/api/logs
GET  /ai-data/api/logs/:file
```

## 自动验收

本地：

```bash
npm run test:smoke
```

云端：

```bash
npm run test:smoke -- http://124.223.28.126:8080/ai-data
```

## 服务

云端服务：

```text
systemctl status ai-data-mvp.service
```

部署目录：

```text
/opt/ai-data-mvp
```
