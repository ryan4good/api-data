# AI Data MVP

一个多业务系统 AI 造数平台 MVP。当前版本用 mock OMS/WMS/TMS 验证闭环：

```text
自然语言需求 -> Agent 解析 -> 场景选择 -> 跨系统接口编排 -> 状态校验 -> 结果日志
```

## 快速运行

启动 mock 业务系统：

```bash
npm run start:mock
```

另开一个终端执行造数：

```bash
npm run run -- "我要造一笔出库单，做到分配库存完成"
npm run run -- "我要造一笔销售订单，并让出库单分配库存完成"
```

扫描示例 Go 代码并生成候选场景：

```bash
npm run scan
```

一键演示：

```bash
npm run demo
```

## 目录

```text
mock-systems/              mock OMS/WMS/TMS 服务
sample-repos/              用于验证扫描能力的 Go 风格示例代码
ai-data-platform/
  agent/                   自然语言入口
  connectors/              系统连接器配置
  domains/                 业务术语
  executor/                执行引擎
  logs/                    执行日志
  scanner/                 代码扫描器雏形
  scenarios/               场景 DSL
  system-registry/         系统注册
docs/                      验收用例
```
