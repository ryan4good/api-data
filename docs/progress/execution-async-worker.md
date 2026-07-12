# Session：execution-async-worker

## 基本信息

- 状态：completed
- 负责人/Session：`scenario-import-review`
- 开始时间：2026-07-12
- 完成时间：2026-07-12
- 修改范围：`apps/api/internal/modules/execution/**`、`apps/api/internal/modules/runrecord/**`、本进度文件

## 目标

- 在不改变既有同步执行入口和数据库迁移的前提下，增加 system-scoped QueueService、异步 Worker、MySQL 原子 lease、heartbeat、过期恢复、取消、执行超时和 retry-key 幂等。

## Red

- 先新增 Queue/Worker 领域测试，覆盖 system-scoped retry key 幂等、双 worker 不重复领取、过期 running 重排、queued/running 取消、执行超时和 worker 完成不得覆盖并发取消。
- 首次目标测试按预期编译失败：`QueueService`、`EnqueueCommand`、`LeaseNext`、`RequeueExpired`、`Worker` 等契约尚不存在。
- 新增 MySQL sqlmock 测试，固定 `BEGIN -> scoped SELECT ... FOR UPDATE SKIP LOCKED -> scoped row lock -> conditional UPDATE -> COMMIT`，以及 heartbeat 必须同时校验 system/run/status/lease owner。
- 新增异步 HTTP 测试，按预期因 `RegisterAsyncRoutes` 不存在而编译失败；随后补齐独立 enqueue/cancel 入口。

## Green

- 新增 `AsyncRepository`、QueueService、Worker 和 Memory/MySQL 实现。
- enqueue 使用 `system_id + retryKey` 幂等；Memory 在锁内查重，MySQL 事务先锁 business system 行，再查 summary retry key/插入，避免同系统并发重复创建。
- MySQL 领取使用 `FOR UPDATE SKIP LOCKED`，再对同一 system/run 做条件更新；多 worker 不会同时领取同一 queued run。
- lease owner/expiry、retry key、execution timeout 保存在既有 summary JSON；heartbeat 仅允许当前 owner 更新 running run。
- worker 每次领取前将 lease 已过期的 running run 重排为 queued，支持进程崩溃/重启后的再次领取。
- queued/running 可取消为 cancelled；worker 在步骤前后检查持久化状态，不会用后续完成结果覆盖并发取消。
- worker 对整个 run 使用 context timeout；超时 attempt 保留，run 状态落 `timed_out`，最终状态更新使用 `context.WithoutCancel` 确保持久化。
- retry key 按业务系统隔离；相同 key 返回已有 run 和 `existing=true`。
- 增加异步 HTTP：enqueue 返回 202，新建之外的幂等命中返回 200；取消返回 200；权限沿用 owner/maintainer/runner。
- 增加可运行 `cmd/worker`：解析独立环境配置、连接 MySQL、组合 MySQLRepository/MySQLStepProvider/HTTPExecutor 或 DisabledExecutor，并在 signal context 下循环执行 RunOnce。
- worker 在无任务或任意迭代错误后等待 poll interval，避免空队列/故障忙循环；SIGINT/SIGTERM 会取消等待并退出。
- execution 目标测试：21/21 通过；`go test ./...`、`go vet ./...` 通过。

## Worker 进程配置

- 必填：`MYSQL_DSN`、`WORKER_SYSTEM_ID`、`WORKER_ID`。
- `WORKER_POLL_INTERVAL`：默认 `1s`，必须为正 duration。
- `WORKER_LEASE_DURATION`：默认 `30s`。
- `WORKER_HEARTBEAT_INTERVAL`：默认 `10s`，且必须短于 lease duration。
- `HTTP_EXECUTOR_ALLOWED_HOSTS`：逗号分隔 hostname；为空时使用 DisabledExecutor，执行安全失败，不开放网络。
- `HTTP_EXECUTOR_ALLOW_PRIVATE`：默认 false；仅在明确受控内网开启。
- MySQL DSN 会通过官方 driver 解析并强制 `parseTime=true`，不会把 DSN 写入配置错误。

## HTTP 契约

- 原同步入口保持不变：`POST /api/v1/systems/{systemId}/scenario-runs`。
- 异步入队：`POST /api/v1/systems/{systemId}/scenario-run-jobs`。
- body：scenarioId、scenarioVersionId、environmentId、retryKey，及可选 stopAfterStepId、executionTimeoutMs、inputVariables。
- 取消：`POST /api/v1/systems/{systemId}/scenario-runs/{runId}/cancel`。
- owner/maintainer/runner 可 enqueue/cancel；非成员 404，其他成员 403。

## MySQL Lease 契约

- candidate：`WHERE system_id=? AND status='queued' ORDER BY created_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`。
- lease update：必须同时匹配 system_id、run id 和原 queued 状态。
- heartbeat：必须匹配 system_id、run id、running 状态和 summary 中的 lease owner。
- expiry/requeue：只处理目标 system 下 running 且 leaseExpiresAt 已过期的记录，并清除 lease 元数据。
- cancel：事务锁住 system-scoped run，仅允许 queued/running 条件更新为 cancelled。

## 改动文件

- `apps/api/internal/modules/execution/async.go`
- `apps/api/internal/modules/execution/async_http.go`
- `apps/api/internal/modules/execution/mysql_async.go`
- `apps/api/internal/modules/execution/domain.go`
- `apps/api/internal/modules/execution/memory_repository.go`
- `apps/api/internal/modules/execution/mysql_repository.go`
- `apps/api/internal/modules/execution/async_test.go`
- `apps/api/internal/modules/execution/async_http_test.go`
- `apps/api/internal/modules/execution/mysql_async_test.go`
- `apps/api/cmd/worker/config.go`
- `apps/api/cmd/worker/main.go`
- `apps/api/cmd/worker/main_test.go`
- `docs/progress/execution-async-worker.md`

## 迁移限制与风险

- 现有表没有 lease_owner、lease_expires_at、retry_key、execution_timeout 或 revision 列，本轮按要求将其保存在 summary JSON，不修改 migration。
- JSON retry key 没有数据库唯一索引；当前通过锁 `business_systems` 的 system 行串行化同系统 enqueue。吞吐增长后应新增 `(system_id,retry_key)` 唯一键。
- JSON lease expiry 使用 UTC RFC3339Nano 字符串比较；后续应迁移为独立 DATETIME(3) 索引列，否则大规模 requeue 会扫描 JSON。
- worker 崩溃后可重新领取，但无法保证外部 HTTP 副作用 exactly-once；下游接口仍需业务幂等键。已有 attempt 会保留，新 worker 会生成递增 attempt。
- MySQL LeaseNext 当前返回 run 主记录；生产恢复若需精确跳过已成功步骤，应在领取后加载 attempts，并基于步骤业务幂等策略决定 resume/retry。
- heartbeat 使用进程内 ticker；后续应记录 heartbeat 失败并主动取消当前执行，避免 lease 丢失后继续产生副作用。
- worker 当前按单个 `WORKER_SYSTEM_ID` 运行；多业务系统需部署多个 worker 实例或后续增加授权 system 分片/调度层。

## 下一接力点

- root 组合 MySQLRepository、MySQLStepProvider、HTTPExecutor/DisabledExecutor，启动循环调用 Worker.RunOnce。
- 增加正式 migration：retry_key 唯一索引、lease owner/expiry/revision、cancel_requested、worker heartbeat/error 字段。
- 增加真实 MySQL 双 worker E2E、进程崩溃恢复、lease 丢失中断和 HTTP 下游幂等测试。
