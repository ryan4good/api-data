# Session：scanner-persistence-api

## 基本信息

- 状态：completed
- 负责人/Session：scanner-persistence-api
- 开始时间：2026-07-11
- 修改范围：`apps/api/internal/modules/scanner/**`、`docs/progress/scanner-persistence-api.md`

## 目标

- 为扫描任务和 API Operation 提供强制 `systemID` 作用域的 Repository、内存/MySQL 实现与应用服务。
- 扫描执行严格遵守 `queued -> running -> succeeded|failed`，并事务化、幂等持久化扫描结果。

## Red

- 先新增 service/memory 测试：跨系统不可见、成功状态链、相同 operation key 的幂等更新、分析失败落失败终态、跨系统运行返回 not found。
- 先新增 sqlmock 测试：scan 与 operation 查询均携带 `system_id`；operation 批量 upsert 使用事务，单条失败整批 rollback。
- 命令：`go test ./internal/modules/scanner`。
- 失败摘要：编译期明确缺少 `MySQLRepository`、`APIOperation`、SQL 常量以及 Service/Repository 等待实现符号，确认测试先进入 Red。

## Green

- 领域与仓储契约：新增 `ScanRun`、`APIOperation`、`Repository`，所有读写方法显式要求 `systemID`，operation 写入同时要求 `scanID`。
- 内存实现：复合 scope key 隔离多业务系统；状态转换执行 compare-and-transition；operation 以 `(systemID, operationKey)` 幂等覆盖，保留原资产 ID/创建时间。
- MySQL 实现：scan 查询/更新均使用 `system_id`；operation 批量 upsert 使用事务和 `ON DUPLICATE KEY UPDATE`，同步最新 `scan_run_id`、代码证据和 `content_hash`。
- 应用服务：创建任务固定为 queued；运行前原子切到 running；AST 分析和持久化成功后切 succeeded；分析或 operation 持久化失败切 failed 并记录错误；生成稳定的 `METHOD path` operation key 和 SHA-256 content hash。
- 验证命令与摘要：
  - `go test ./internal/modules/scanner`：通过。
  - `go vet ./internal/modules/scanner`：通过。
  - `go test ./...`：全量通过（并行 importer session 完成 Green 后复验）。
  - `go vet ./...`：全量通过。
  - `go test -race ./internal/modules/scanner`：本机 Go 环境未启用 cgo，race 构建无法启动；普通测试已覆盖并发安全实现。

## 改动文件

- `apps/api/internal/modules/scanner/repository.go`
- `apps/api/internal/modules/scanner/memory_repository.go`
- `apps/api/internal/modules/scanner/mysql_repository.go`
- `apps/api/internal/modules/scanner/service.go`
- `apps/api/internal/modules/scanner/service_test.go`
- `apps/api/internal/modules/scanner/mysql_repository_test.go`
- `docs/progress/scanner-persistence-api.md`

## 契约决策

- Repository 不提供无作用域的 Get/List；调用方必须传 `systemID`，错误系统读取 scan 表现为 not found，避免泄露资源存在性。
- 状态更新使用 `(system_id, id, expected_status)` compare-and-transition，防止重复执行或终态回退。
- API 资产唯一性沿用迁移中的 `(system_id, operation_key)`；新扫描命中同一方法+路径时更新来源 scan、证据和 content hash，不产生重复资产。
- content hash 覆盖 method/path/handler/source file/line；代码证据保留 handler 与源码位置，供人工核验。
- operation 批量写入必须全成或全败；内存实现也先校验整批 scope 后再修改，保持相同语义。

## 已知限制

- 本 session 未注册共享 HTTP 路由；root 可直接组合 `Service.Create`、`Service.Run`、`Service.ListScans`、`Service.ListOperations`，并在 HTTP 层完成成员权限校验。
- `scan_runs` 引用的 business system、code source、request user 必须由上游预先存在；外键错误按 repository error 返回。
- 当前 operation 映射只落代码证据及核心 method/path，schema、tag、security 等增强字段等待扫描器后续能力。
- 并发触发同一 queued scan 时由 MySQL expected-status 更新拒绝第二次转换；内存实现同样在互斥锁内比较状态。

## 下一接力点

- 在共享 server 中注入 MySQL scanner repository 与 service，并注册 system-scoped HTTP endpoint；调用前复用 access 模块校验业务系统成员权限。
- 增加真实 MySQL E2E：创建 source/scan、执行样例仓库、验证重复扫描只保留一份 operation 资产及跨系统不可见。
- 增加异步 worker/队列和取消状态；当前 `Run` 是同步入口。
