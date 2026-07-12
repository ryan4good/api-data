# Backend MySQL Repository 进度登记

## 基本信息

- 状态：Green
- Session：`backend_mysql_repository`
- 日期：2026-07-11
- 修改范围：`apps/api/**` 与本文件
- 方法：TDD，逐项执行 Red → Green

## 已完成内容

### 1. MySQL Repository 与系统隔离查询

- 状态：Green
- Red 测试：`apps/api/internal/modules/system/mysql_repository_test.go`
- Red 命令：`go test ./internal/modules/system`
- Red 证据：`MySQLRepository`、`NewMySQLRepository`、查询常量均不存在，测试编译失败。
- Green：使用 `database/sql` 与 `go-sql-driver/mysql` 实现：
  - 按认证 `user_id` 查询授权系统列表。
  - 按 `user_id + system_id` 查询授权系统详情；无结果统一映射为未找到。
  - 按 `system_id + user_id` 查询成员角色。
  - 按 `system_id` 查询成员列表，并 join `users` 返回展示名、邮箱和用户状态。
  - 按 `system_id + user_id` upsert 成员角色，成员主键由应用生成 UUID。
- Green 命令：`go test ./internal/modules/system`
- Green 结果：通过。

### 2. MYSQL_DSN 条件装配、启动 Ping 与优雅关闭

- 状态：Green
- Red 测试：`apps/api/cmd/server/main_test.go`
- Red 命令：`go test ./cmd/server`
- Red 证据：`selectSystemRepository` 与 `openSystemMySQL` 未定义，测试编译失败。
- Green：
  - 未配置 `MYSQL_DSN` 时继续使用内存 Repository。
  - 配置 `MYSQL_DSN` 时创建 MySQL Repository，应用连接池参数并在启动 HTTP server 前执行 `PingContext`。
  - Ping 失败时关闭连接并阻止服务启动。
  - 服务退出时调用 Repository `Close`；HTTP server 本身仍按既有超时优雅关闭。
  - `httpapi.NewWithSystemRepository` 提供明确的组合根注入点，`httpapi.New` 保持内存实现兼容现有测试。
- Green 命令：`go test ./cmd/server ./internal/httpapi ./internal/modules/system`
- Green 结果：通过。

### 3. 成员更新响应回读真实用户资料

- 状态：Green
- Red 测试：`TestMySQLRepositoryUpsertsMemberWithBothScopeKeys`
- Red 命令：`go test ./internal/modules/system -run TestMySQLRepositoryUpsertsMemberWithBothScopeKeys`
- Red 证据：原 `UpsertMember` 只返回 error，且不存在按 `system_id + user_id` 回读成员的 `getMemberSQL`，无法返回 `users.display_name`。
- Green：Repository 的 `UpsertMember` 返回持久化后的完整 `Member`；MySQL 实现在 upsert 后按双作用域键 join `users` 回读，HTTP handler 返回真实 `displayName/email/status`，不再以 userId 伪造 displayName。
- Green 命令：`go test ./internal/modules/system ./cmd/server ./internal/httpapi`
- Green 结果：通过。

## SQL 与接口契约决策

- 授权系统 list/get SQL 与 `db/queries/business_systems.scoped.sql` 保持同一列、join、过滤和参数顺序；应用始终从认证上下文传入 `user_id`。
- 系统详情与角色查询显式同时绑定 `system_id`、`user_id`，避免仅凭资源 ID 读取跨系统数据。
- 成员列表显式绑定 `system_id`；HTTP 层在调用前先通过同一 Repository 以 `system_id + actor user_id` 校验 owner。
- 成员 upsert 与回读均显式绑定 `system_id + target user_id`；数据库唯一键 `(system_id, user_id)` 保证幂等更新。
- `sql.ErrNoRows` 对授权详情和角色查询映射为 `found=false`，其他数据库错误保留 cause 并只由 HTTP 层输出统一内部错误。
- 目标数据库契约仍为 MySQL 8.0；SQL mock 不依赖 MariaDB 特性。

## 验证

- 格式化：`gofmt`，完成。
- 全量测试：`go test ./...`，通过。
- 静态检查：`go vet ./...`，通过。
- 可选真实库测试：`TestMySQLRepositoryAgainstDevelopmentSeed` 仅在设置 `MYSQL_INTEGRATION_DSN` 时运行，要求 DSN 指向已迁移并加载 development seed 的临时/本地库。
- 本 Session 曾连接本地 `127.0.0.1:3306`，服务器可达，但永久库 `bizdevops` 当时不存在；集成编排应使用数据库 Session 创建并清理的临时库。凭据未写入仓库。
- 后续 `system_e2e` Session 已使用本实现连接本地 MariaDB 12.2.2 临时库完成真实 System API E2E 1/1：授权隔离、越权 404/403、owner 成员更新和持久化回读均通过，API 进程与临时库已清理。目标生产契约仍为 MySQL 8.0。

## 改动文件

- `apps/api/go.mod`
- `apps/api/go.sum`
- `apps/api/cmd/server/main.go`
- `apps/api/cmd/server/main_test.go`
- `apps/api/internal/httpapi/server.go`
- `apps/api/internal/modules/system/repository.go`
- `apps/api/internal/modules/system/memory_repository.go`
- `apps/api/internal/modules/system/repository_test.go`
- `apps/api/internal/modules/system/module.go`
- `apps/api/internal/modules/system/mysql_repository.go`
- `apps/api/internal/modules/system/mysql_repository_test.go`
- `apps/api/internal/modules/system/mysql_integration_test.go`
- `docs/progress/backend-mysql-repository.md`

## 已知限制与下一接力点

- `MYSQL_INTEGRATION_DSN` 必须包含 `parseTime=true`，否则 MySQL DATETIME 无法扫描到 `time.Time`。
- 本轮成员 upsert 只管理成员关系与角色，目标用户必须已存在于 `users`；外键错误当前映射为统一 500，后续可增加领域错误并映射 404/422。
- 成员 upsert 与随后回读是两条已提交语句；若后续要求强一致的写后读快照，可改为事务实现。
- `RoleForUser` 遵循当前 `system_members` 契约，尚未 join `users.status`；真实认证接入时应统一阻止 disabled 用户获得 actor 身份。
- 下一 Session 可直接通过 `system.OpenMySQLRepository` + `httpapi.NewWithSystemRepository` 建立临时已迁移库 E2E，验证 Alice 只能访问 OMS、owner 成员更新以及 GET members 持久化结果。
