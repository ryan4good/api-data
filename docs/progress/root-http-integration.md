# Session：root-http-integration

## 基本信息

- 状态：completed
- 范围：共享 HTTP 注册、MySQL 装配、真实数据库与浏览器回归

## Red

- `go test ./internal/httpapi` 首次编译失败：`NewWithRepositories` 尚不存在，证明扫描/导入 Repository 未接入共享 Server。
- `go test ./cmd/server` 首次编译失败：`selectWorkflowRepositories` / `openWorkflowMySQL` 尚不存在，证明配置 MySQL 时工作流能力仍会停留在内存。
- 真实 MySQL E2E 首次返回 scans 400；定位为开发 seed 的 UUID version/variant 位无效，而新 system-scoped 路由执行严格 UUID 校验。
- Web client 新增显式开发身份测试后首次失败，证明浏览器开发环境尚不能安全、显式地提供本地用户身份。

## Green

- 新增 `httpapi.NewWithRepositories`，统一注册 System、Scanner 和 Importer 路由。
- Server 配置 DSN 时装配 Scanner/Importer MySQL Repository；无 DSN 才使用内存实现。
- 开发 seed 改为有效 v4 UUID，未放宽生产 UUID 校验。
- Web 仅在 Vite DEV 且显式设置 `VITE_DEV_USER_ID` 时发送 `X-Dev-User-ID`；生产构建不注入。
- 本地 MariaDB 真实执行 migration/seed/scoped reads/role enum/down 通过。
- E2E 覆盖系统成员隔离、三类资源空态、Postman 上传、Viewer 发布 403、Owner applied，1/1 通过。
- Go `test ./...`、Go `vet ./...`、Web 22 项测试与生产构建、8 项 DB 隔离契约及 2 个合同验证全部通过。
- 浏览器验证 Owner 与 Viewer 工作区：系统名/角色正确、空态真实、Viewer 写操作隐藏、无 console warning/error；临时进程和 QA 数据库已清理。

## 改动文件

- `apps/api/internal/httpapi/server.go`、`server_test.go`
- `apps/api/cmd/server/main.go`、`main_test.go`
- `apps/web/src/api/client.ts`、`client.test.ts`
- `db/seeds/000001_development.sql`
- `db/tests/run_mysql_integration.py`
- `tests/e2e/test_system_api_mysql.py`
- `apps/api/internal/modules/system/mysql_integration_test.go`

## 下一接力点

- 扫描 POST 目前接收服务端本地路径；生产需改为受控 workspace/source connector，禁止任意路径访问。
- 前端仍缺扫描创建、Postman 上传、核验/apply 的真实表单和 mutation 状态。
- 场景发现、场景发布、执行器与运行详情仍是后续主链路。
