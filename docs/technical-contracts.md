# 技术契约：React + Go + MySQL

本文锁定 MVP 的数据边界与交换协议。MySQL 版本为 8.0+，字符集为 `utf8mb4`；时间统一存 UTC `DATETIME(3)`，API 使用 RFC 3339；ID 为应用生成的 UUID 字符串。

## 系统隔离与授权

`business_systems` 是租户边界。除平台用户和系统本身外，所有业务资源都必须携带非空 `system_id`。Repository 查询必须显式传入 `system_id`，禁止仅按资源 ID 查询。跨资源关系使用 `(system_id, id)` 复合外键，数据库直接拒绝跨系统关联。

权限由平台角色和系统角色共同决定：

| 系统角色 | 主要能力 |
|---|---|
| owner | 成员管理、系统归档、全部维护能力 |
| maintainer | 代码源、环境、扫描、场景发布与执行 |
| reviewer | 场景证据核验、跨系统 API 确认与审核 |
| runner | 发起执行、取消、重试并查看完整运行结果 |
| viewer | 只读资产与运行记录 |

所有写操作、权限变化、导入、发布和执行都写 `audit_logs`。审计记录随系统保留，不对业务删除做级联。

## 模块与数据所有权

| Go 模块 | 拥有的数据 | 对外职责 |
|---|---|---|
| identity | `users` | 身份、平台角色、账号状态 |
| systems | `business_systems`, `system_members` | 系统生命周期、成员和 RBAC |
| sources | `code_sources`, `scan_runs` | 代码源配置、扫描任务状态 |
| apiassets | `api_operations` | 统一 API 资产、证据、核验状态 |
| discovery | `scenario_discoveries`, `scenario_candidates` | 多来源场景发现、候选证据与人工审核 |
| scenarios | `scenario_imports`, `scenarios`, `scenario_versions`, `scenario_steps` | 导入、版本、编辑、发布 |
| environments | `environments`, `connectors` | 环境变量、连接器和 secret reference |
| runner | `scenario_runs`, `scenario_step_runs`, `scenario_assertion_results` | 执行状态机、步骤快照、断言结果 |
| audit | `audit_logs` | 只追加审计查询 |

模块不能直接修改其他模块拥有的表。同步交互走 application service；扫描和执行等长任务通过 job/event 接口解耦，MVP 可先由 MySQL 任务状态轮询实现。

## 核心状态机

- ScanRun：`queued → running → succeeded|failed|cancelled`。
- ScenarioDiscovery：`queued → running → ready|failed|cancelled`。
- ScenarioCandidate：`pending → accepted|rejected`；accepted 候选可关联转正后的场景，但候选证据保持不可变。
- ScenarioImport：`uploaded → validating → ready|failed → applied`；`ready` 只代表可编辑，不代表可发布。
- Scenario：`draft → active → archived`。发布只改变 `current_version_id`，历史版本不可修改。
- ScenarioRun：`queued → running → passed|failed|cancelled|timed_out`。
- StepRun：`pending → running → passed|failed|skipped|timed_out`。

状态变更采用条件更新，例如 `UPDATE ... WHERE id=? AND system_id=? AND status='queued'`；受影响行数为 0 视为并发冲突。运行和版本记录只追加，HTTP request/response 落库前必须脱敏。

## API Operation 契约

`operation_key` 是同一系统内稳定键，建议格式为 `METHOD /normalized/{path}`。扫描器用它做 upsert，并用 `content_hash` 判断定义是否变化。`request_schema`、`response_schemas` 使用 JSON Schema；`code_evidence` 至少包含仓库 ref、文件、起止行和识别器版本。低置信结果必须为 `unverified`。

删除或未识别 API 不删除历史数据：把资产标记为 `removed`，已发布场景仍能依据 operation key 和版本快照展示。场景执行前检查其 API 引用是否仍可绑定。

## Scenario Bundle 1.0

权威 Schema 位于 `contracts/scenario-bundle.schema.json`，示例位于 `contracts/examples/scenario-bundle.example.json`。Bundle 是无损交换格式，包含：

- 系统稳定键和场景稳定键；
- 版本、变量、步骤依赖、超时和失败策略；
- API 引用、连接器引用、HTTP 请求；
- 变量提取、断言和需要复核的脚本；
- 只允许在明确的 `extensions` 中携带厂商信息。

变量引用语法为 `{{variableKey}}`。敏感变量禁止包含 `value`，必须使用 `secretRef`；导出永不解析 secret。导入方先做 JSON Schema 校验，再检查 step key 唯一性、依赖存在且无环、变量引用可解析、API/connector 可绑定。Schema 校验通过不等于语义校验通过。

Postman Collection 2.1 是兼容输入而非平台权威模型，详细规则见 `contracts/postman-2.1-mapping.md`。

## 运行快照与可复现性

ScenarioRun 固定引用 `scenario_version_id` 和 `environment_id`。开始执行时解析变量并生成输入快照；每个 StepRun 保存脱敏后的 request/response、提取变量、耗时和错误。断言逐条保存，步骤失败原因不能只存在日志文本中。

重试为同一 `scenario_step_id` 增加 `attempt_no`，不得覆盖前次结果。`failurePolicy=stop` 时后续依赖步骤为 skipped；`continue` 仅允许不依赖失败输出的步骤继续。

## 数据库约束说明

- 可选的复合租户外键没有 `ON DELETE SET NULL`：MySQL 会尝试同时清空非空 `system_id`。删除被引用对象前应由 application service 清除引用或归档对象。
- `scenarios.current_version_id` 与 `scenario_versions` 构成受控环，创建场景时为空，创建版本后再发布；删除版本前必须先取消当前版本引用。
- JSON 字段存可演进文档，稳定查询维度保持为普通列并建立系统前缀索引。
- secret 只存外部 secret manager 的 reference；`credential_ref`、`secret_ref` 不得包含实际凭据。

## 可执行验证

```powershell
python db/tests/validate_migration.py
python contracts/tests/validate_scenario_bundle.py
```

第一项检查必需表、tenant 列/索引、复合 tenant 外键和 MySQL `SET NULL` 限制；第二项使用 Draft 2020-12 validator 校验 Schema 本身和标准示例，并补充 step key 唯一性检查。生产 CI 还应在临时 MySQL 8.0 实例实际执行 up/down migration。
