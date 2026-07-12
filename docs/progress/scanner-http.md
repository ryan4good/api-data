# Session：scanner-http

## 基本信息

- 状态：completed
- 负责人/Session：scanner-persistence-api
- 开始时间：2026-07-11
- 修改范围：`apps/api/internal/modules/scanner/**`、`docs/progress/scanner-http.md`

## 目标

- 提供由 root 显式组合的业务系统级扫描 HTTP 路由。
- 使用 actor 与 system member role 做资源隐藏和读写权限隔离。

## Red

- 先新增 HTTP 测试，覆盖四个固定端点、viewer 读取、owner/maintainer 写入、非成员 404、只读成员 403、严格 JSON、1MiB、UUID、空 repositoryRoot、service/role reader 错误映射。
- 命令：`go test ./internal/modules/scanner`。
- 失败摘要：编译期缺少 `ScannerHTTPService`、`RegisterSystemRoutes`，且 `CreateScanInput` 尚未承载 language/framework，确认测试先进入 Red。

## Green

- 新增 `ScannerHTTPService` 和 `RegisterSystemRoutes`，由 root 注入 service、role reader 并选择身份中间件，不修改共享 server。
- 固定端点：
  - `GET /api/v1/systems/{systemId}/scans`
  - `POST /api/v1/systems/{systemId}/scans`
  - `POST /api/v1/systems/{systemId}/scans/{scanId}/run`
  - `GET /api/v1/systems/{systemId}/api-operations`
- RBAC：任意已找到的系统成员可读；owner/maintainer 可创建和运行；非成员统一返回 404 `system_not_found`；其他成员写入返回 403。
- 输入边界：system/source/scan 使用规范 UUID；JSON 最大 1MiB、拒绝未知字段和多个 JSON 值；repositoryRoot trim 后必须非空。
- 输出边界：所有成功/失败及 405 都使用 `httpresponse` envelope；create 返回 201，run 同步完成返回 200，scan not found 返回 404，非法状态返回 409。
- 创建扫描输入及持久化补齐 language/framework。
- 验证：`go test ./internal/modules/scanner`、`go vet ./internal/modules/scanner`、`go test ./...`、`go vet ./...` 全部通过。

## 改动文件

- `apps/api/internal/modules/scanner/http.go`
- `apps/api/internal/modules/scanner/http_test.go`
- `apps/api/internal/modules/scanner/service.go`
- `docs/progress/scanner-http.md`

## 契约决策

- HTTP 层只依赖窄接口 `ScannerHTTPService` 和 `access.RoleReader`，便于 root 使用 MySQL 或 memory 组合，也便于处理器单测隔离。
- 权限检查发生在读取请求体和调用 service 之前，非成员即使猜中资源 ID 也只能观察到 system 404。
- 读取权限由 role reader 的 `found` 表示成员关系；写权限再检查 owner/maintainer。
- run 当前为同步操作：service 返回 nil 即代表任务已转 succeeded，因此响应状态为 succeeded。
- 路由使用显式 method dispatch，避免 Go ServeMux 默认纯文本 405 破坏统一 API envelope。

## 已知限制

- `repositoryRoot` 当前只检查非空，尚未限制允许扫描的根目录、符号链接逃逸或进程可访问路径。部署前必须增加 server-side workspace allowlist，并将客户端路径转换为受控 code source/workspace，而不能信任任意本地路径。
- run 同步占用 HTTP 请求；大仓库应改为投递 worker 后返回 202，并由 scan status 查询进度。
- 本 session 不注册共享 server、不选择 JWT/development middleware；root 组合层负责统一身份认证。
- 尚未增加真实 MySQL HTTP E2E，本轮以 handler 隔离测试和 repository/service 既有测试覆盖。

## 下一接力点

- root 在共享 server 中构造 `scanner.NewMySQLRepository(db)`、`scanner.NewService(repo, scanner.AnalyzerFunc(scanner.Analyze))`，再调用 `scanner.RegisterSystemRoutes(mux, service, systemRepository)`。
- 加入 workspace allowlist/代码源解析器后再开放生产 run endpoint。
- 前端接入 scan 创建、列表、运行和 API operation 资产列表，并处理 403/404/409。
