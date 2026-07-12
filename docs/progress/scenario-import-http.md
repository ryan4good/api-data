# Session：scenario-import-http

## 基本信息

- 状态：completed
- 负责人/Session：`scenario-import-review`
- 开始时间：2026-07-11
- 完成时间：2026-07-11
- 修改范围：`apps/api/internal/modules/importer/**`、本进度文件

## 目标

- 为 importer 应用服务提供可由根 Session 注册的 system-scoped HTTP/RBAC 竖切，不修改共享 server/config。

## Red

- 先新增 HTTP 测试，固定上传、列表、详情、脚本确认和应用五类路由的成功契约。
- 测试覆盖所有角色：成员可读；owner/maintainer 可上传和应用；owner/maintainer/reviewer 可确认脚本；其余成员写入返回 403；非成员统一返回 404。
- 测试覆盖严格 JSON、未知字段/尾随 JSON、1 MiB 上限、raw document 不回传、敏感原值不进入错误、失败草稿和脚本门禁的稳定 409。
- 首次执行 `go test ./internal/modules/importer` 按预期编译失败：`RegisterSystemRoutes` 尚不存在。

## Green

- 新增 `RegisterSystemRoutes(mux, service, roleReader)`，只注册 importer 的 system-scoped 路由，由共享 server 负责组合身份中间件。
- 每个 handler 防御性读取 `access.ActorFromContext`，并通过 `access.RoleReader` 先校验系统成员身份，再解析 body 或访问 importer Service。
- 上传使用 `http.MaxBytesReader` 限制 1 MiB、`DisallowUnknownFields` 且只接受单个 JSON 值；`fileName` 必填且不超过数据库列的 512 字节。
- HTTP view 不包含 `raw_document`；错误响应仅返回固定 code/message，不拼接 decoder、数据库或导入文档错误。
- importer 目标测试：20/20 通过；`go test ./...`、`go vet ./...` 通过。

## 路由契约

- `POST /api/v1/systems/{systemId}/scenario-imports`：owner/maintainer，成功 201。
- `GET /api/v1/systems/{systemId}/scenario-imports`：任意成员，成功 200。
- `GET /api/v1/systems/{systemId}/scenario-imports/{importId}`：任意成员，成功 200。
- `POST .../{importId}/confirm-scripts`：owner/maintainer/reviewer，成功 200。
- `POST .../{importId}/apply`：owner/maintainer，成功 200。
- 非成员：`404 system_not_found`；权限不足：`403 forbidden`；未认证：`401 authentication_required`。
- 输入错误：`400 invalid_request`；导入不存在：`404 scenario_import_not_found`。
- 门禁冲突：`409 import_has_errors`、`409 scripts_unreviewed`、`409 import_not_ready`。

## 改动文件

- `apps/api/internal/modules/importer/http.go`
- `apps/api/internal/modules/importer/http_test.go`
- `docs/progress/scenario-import-http.md`

## 已知限制

- 本 Session 按约束未注册共享 server；根 Session 需要构造 importer Service/MySQLRepository，并在身份中间件内调用 `RegisterSystemRoutes`。
- 上传 Options 暂以 `systemId` 同时作为导入 Bundle 的 system key/name；后续共享编排层可从业务系统资料补入稳定 code/name。
- 当前人工确认仍是整份导入的全部脚本确认，不是逐脚本批注。
- HTTP `apply` 只推进 importer 的 `applied` 状态；创建 scenario/version/steps 仍由后续发布事务负责。

## 下一接力点

- 根 Session 接入共享 server，并补真实 JWT + MySQL 的端到端测试。
- 接入业务系统资料，将 system code/name 传给 Upload Options。
- 在应用成功时调用场景版本落库服务并记录 audit log。

## 并行验证说明

- 首次全量测试曾撞上 scanner 并行 Session 的 Red 阶段；通知对方完成 Green 后再次执行，全量测试与 vet 最终均通过。
