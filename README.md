# AI Data MVP

> 当前正在重构为 API Data Intelligence Platform。新架构采用 React + TypeScript、Go 和 MySQL 8.0，并强制使用 Red → Green → Refactor 的 TDD 流程。原有 Node.js MVP 暂时保留，用于行为对照和迁移验收。

## 重构工程

```text
apps/web/       React + TypeScript 管理控制台
apps/api/       Go HTTP API 与领域模块
db/             MySQL 8.0 迁移和结构验证
contracts/      Scenario Bundle Schema 与 Postman 映射
docs/           产品、架构、契约和线框文档
```

关键文档：

- `docs/refactoring-blueprint.md`：产品与重构方案。
- `docs/technical-architecture.md`：模块边界和依赖规则。
- `docs/technical-contracts.md`：MySQL、领域和交换协议。
- `contracts/scenario-bundle.schema.json`：原生场景交换格式。

验证命令：

```bash
cd apps/web && npm test -- --run && npm run build
cd apps/api && go test ./... && go vet ./...
python db/tests/validate_migration.py
python -m unittest db.tests.test_system_isolation_contract -v
python contracts/tests/validate_scenario_bundle.py
python contracts/tests/validate_system_api.py
```

跨 Session 进度和接力入口统一登记在 `docs/progress/README.md`。

## 原 Node.js MVP

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
