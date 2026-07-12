# Session：execution-step-control

## 基本信息

- 状态：completed
- 负责人/Session：`scenario-import-review`
- 开始时间：2026-07-12
- 完成时间：2026-07-12
- 修改范围：`apps/api/internal/modules/execution/**`、`apps/api/internal/modules/runrecord/**`、本进度文件

## 目标

- 基于现有 `scenario_runs`、`scenario_step_runs`、`scenario_assertion_results` 表实现 system-scoped 顺序执行、指定步骤停止、失败停止、单步重试、运行记录查询、脱敏和 HTTP/RBAC。

## Red

- 先新增 execution Service 测试，固定完整成功、`stopAfterStepId` 部分完成、断言失败立即停止、单步 retry 只新增目标 attempt、未执行步骤不可重试、快照/错误脱敏。
- 首次执行目标测试按预期编译失败：`Step`、`StepResult`、`Repository`、`Service` 等领域契约尚不存在。
- 再新增 MySQL sqlmock 测试，固定 attempt + assertion 原子事务，以及 run/attempt/assertion 的逐层 `system_id` scope；按预期因 MySQLRepository/SQL 契约不存在而编译失败。
- 最后新增 execution/runrecord HTTP 测试，固定 runner 权限矩阵、成员读取、非成员 404、严格 JSON/1 MiB；按预期因 `RegisterSystemRoutes` 不存在而编译失败。

## Green

- 实现执行领域、可插拔 `StepProvider` / `StepExecutor`、Service、并发安全 MemoryRepository 和 MySQLRepository。
- 场景步骤按 position 顺序执行；完整成功为 `status=passed,outcome=succeeded`；指定步骤停止为 `status=passed,outcome=partial`；断言失败或 executor error 立即停止并标记 failed。
- retry 只执行目标步骤，使用递增 `attemptNo` 保留历史，不重新执行其他步骤；未执行步骤返回 `ErrStepNotExecuted`。
- 保存请求/响应摘要、提取变量、耗时、固定长度错误、断言预期/实际/消息；敏感 header/body/变量及错误或断言消息中的已知 secret 统一替换为 `[REDACTED]`。
- MySQL attempt 与 assertion 在同一事务提交/回滚；所有 INSERT/SELECT/UPDATE 显式携带 `system_id`。
- runrecord QueryService 复用 execution Repository 的 system-scoped List/Get。
- execution HTTP：owner/maintainer/runner 可运行和 retry；runrecord HTTP：任意系统成员可读；非成员统一 `404 system_not_found`。
- 增加生产 MySQLStepProvider，按 `system_id + scenario_version_id` 和 position/id 稳定顺序读取启用步骤，并映射 request/extractors/assertions/timeout/failure policy。
- 增加安全默认 DisabledExecutor：绝不访问网络或执行脚本，持久化 failed run/attempt 后返回 `ErrExecutorNotConfigured`；HTTP 映射为 `503 executor_not_configured`，不制造假成功。
- execution/runrecord 目标测试：15/15 通过；`go test ./...`、`go vet ./...` 通过。

## 状态与结果契约

- 数据库迁移已有 run status ENUM：`queued/running/passed/failed/cancelled/timed_out`，没有 `succeeded/partial`。
- 因此数据库 `scenario_runs.status` 保持 `passed/failed`；用户侧完成语义写入 summary JSON 中的 `outcome=succeeded|partial|failed`，避免修改既有迁移。
- `stopAfterStepId` 必须属于当前版本；仅持久化实际执行的步骤，不为后续步骤制造 skipped attempt。
- 失败优先于 stop-after：目标步骤失败时 run 为 failed，而不是 partial。
- retry 成功后，如果原运行尚有从未执行步骤则 outcome 为 partial；所有唯一步骤均已执行才为 succeeded。

## HTTP 契约

- `POST /api/v1/systems/{systemId}/scenario-runs`：同步执行，成功 201；body 包含 scenarioId、scenarioVersionId、environmentId，可选 stopAfterStepId/inputVariables。
- `POST /api/v1/systems/{systemId}/scenario-runs/{runId}/steps/{stepId}/retry`：单步重试，成功 200。
- `GET /api/v1/systems/{systemId}/scenario-runs`：成员运行记录列表。
- `GET /api/v1/systems/{systemId}/scenario-runs/{runId}`：成员运行详情，包含全部 attempts/assertions。
- 稳定错误：`400 invalid_request`、`404 system_not_found/run_not_found/step_not_found`、`409 step_not_executed`、`403 forbidden`。

## 改动文件

- `apps/api/internal/modules/execution/domain.go`
- `apps/api/internal/modules/execution/service.go`
- `apps/api/internal/modules/execution/memory_repository.go`
- `apps/api/internal/modules/execution/mysql_repository.go`
- `apps/api/internal/modules/execution/mysql_step_provider.go`
- `apps/api/internal/modules/execution/disabled_executor.go`
- `apps/api/internal/modules/execution/http.go`
- `apps/api/internal/modules/execution/service_test.go`
- `apps/api/internal/modules/execution/mysql_repository_test.go`
- `apps/api/internal/modules/execution/production_boundaries_test.go`
- `apps/api/internal/modules/execution/http_test.go`
- `apps/api/internal/modules/runrecord/service.go`
- `apps/api/internal/modules/runrecord/http.go`
- `apps/api/internal/modules/runrecord/service_test.go`
- `apps/api/internal/modules/runrecord/http_test.go`
- `docs/progress/execution-step-control.md`

## 已知限制

- StepExecutor 是边界接口；本 Session 用 fake executor 验证编排，并提供不会访问外网的 DisabledExecutor 作为安全生产默认值。真实 HTTP/脚本执行器尚未实现，未显式注入时执行入口稳定返回 503，同时保留失败记录。
- 当前 HTTP 是同步执行入口；生产环境应改为创建 queued run 后投递 worker，并使用相同 Service/Repository 状态契约。
- retry 的 attemptNo 由读取到的历史计算，并受数据库唯一键保护；极端并发重复 retry 中一个请求可能收到唯一键冲突，后续可在 run 行锁内分配 attemptNo 并增加幂等键。
- 输入/输出持久化的是脱敏值；重试需要的 secret 应由后续环境/connector secret resolver 在 StepExecutor 内重新绑定，不能从运行记录反向恢复。
- MySQL ListRuns 当前逐 run 加载 attempts/assertions，适用于详情量较小的 MVP；大列表需改摘要分页，详情再单独加载。
- 本 Session 按范围未修改 shared server，旧 placeholder 注册仍保留；根 Session 需注册两个新 `RegisterSystemRoutes`。

## 下一接力点

- 根 Session 注入场景版本 StepProvider、MySQLRepository、实际 connector-aware StepExecutor，并注册 execution/runrecord 路由。
- 增加真实 MySQL E2E，验证 JSON summary/outcome、attempt 唯一键、断言事务回滚和跨系统 404。
- 增加 queued worker、取消/超时、分布式 retry 幂等键与 audit log。
