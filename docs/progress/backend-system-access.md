# Backend System/Access 进度登记

## 范围与协作约束

- 负责路径：`apps/api/**` 与本文件。
- 契约来源：`contracts/openapi/system-api.yaml`、`docs/technical-contracts.md`。
- 开发方法：每个独立内容严格执行 Red → Green；此文件保存可供后续 session 接力的命令、决策与限制。

## 进度

### 1. 领域类型与内存 Repository

- 状态：Green
- Red 测试：`internal/modules/system/repository_test.go`
- Red 命令：`C:\Users\ryanf\AppData\Local\Temp\go1.22.12\go\bin\go.exe test ./internal/modules/system`
- Red 结果：待实现的 `BusinessSystem`、`Member`、`NewMemoryRepository` 等符号导致编译失败，符合预期。
- Green 命令：同上。
- Green 结果：`ok bizdevops/apps/api/internal/modules/system`。
- 完成文件：`access/role.go`、`system/domain.go`、`system/repository.go`、`system/memory_repository.go`、`system/repository_test.go`。
- 决策：授权查询由 Repository 显式携带 `userID`；系统内资源查询继续显式携带 `systemID`。禁用成员不进入授权范围；内存实现只用于测试/开发，并保证列表顺序稳定。

### 2. 开发身份中间件与授权服务

- 状态：Green
- Red 测试：`internal/modules/access/access_test.go`
- Red 命令：`C:\Users\ryanf\AppData\Local\Temp\go1.22.12\go\bin\go.exe test ./internal/modules/access`
- Red 结果：`ActorFromContext`、`DevelopmentIdentity`、`NewAuthorizer` 等符号未定义，编译失败。
- Green 命令：同上。
- Green 结果：`ok bizdevops/apps/api/internal/modules/access`。
- 完成文件：`access/identity.go`、`access/authorizer.go`、`access/access_test.go`。
- 决策：成员管理最低角色为 `owner`，与技术契约中“owner 负责成员管理”一致。身份占位统一只由中间件读取 `X-Dev-User-ID`，业务 handler 不直接读取请求头。
- 限制：该请求头当前被完全信任，仅用于本地开发/测试，不具备签名、令牌校验、用户状态校验或防伪能力；生产组合必须替换为真实身份提供方。

### 3. System/Member HTTP API

- 状态：Green
- Red 测试：`internal/modules/system/http_test.go`
- Red 命令：`C:\Users\ryanf\AppData\Local\Temp\go1.22.12\go\bin\go.exe test ./internal/modules/system`
- Red 结果：测试向 `Register` 传入 Repository，而占位实现只接受 mux，编译失败（`too many arguments in call to Register`）。
- Green 命令：同上。
- Green 结果：`ok bizdevops/apps/api/internal/modules/system`。
- 完成文件：`system/module.go`、`system/http_test.go`，并更新 `httpapi/server.go`、`httpapi/server_test.go` 完成组合。
- 已实现：
  - `GET /api/v1/systems`：只返回当前用户的 active membership，并输出契约字段 `myRole`。
  - `GET /api/v1/systems/{systemId}`：仅授权范围内可见；不存在和范围外统一返回 404，避免枚举系统。
  - `GET /api/v1/systems/{systemId}/members`：仅 owner。
  - `POST /api/v1/systems/{systemId}/members`：仅 owner，严格 JSON、1 MiB body 上限、UUID 与五种角色校验，响应符合 `SystemMember`。
- 决策：OpenAPI 的成员更新使用 `POST`；角色枚举固定为 `owner/maintainer/reviewer/runner/viewer`；错误继续使用统一 `{ "error": ... }` envelope；Repository 异常不对外泄漏细节。

## 最终验证

- 格式化：`gofmt -w internal/modules/access internal/modules/system internal/httpapi/server.go internal/httpapi/server_test.go`，完成。
- 全量测试：`C:\Users\ryanf\AppData\Local\Temp\go1.22.12\go\bin\go.exe test ./...`，全部通过。
- 静态检查：`C:\Users\ryanf\AppData\Local\Temp\go1.22.12\go\bin\go.exe vet ./...`，通过、无输出。

## 文件清单

- 新增：
  - `apps/api/internal/modules/access/role.go`
  - `apps/api/internal/modules/access/identity.go`
  - `apps/api/internal/modules/access/authorizer.go`
  - `apps/api/internal/modules/access/access_test.go`
  - `apps/api/internal/modules/system/domain.go`
  - `apps/api/internal/modules/system/repository.go`
  - `apps/api/internal/modules/system/memory_repository.go`
  - `apps/api/internal/modules/system/repository_test.go`
  - `apps/api/internal/modules/system/http_test.go`
- 修改：
  - `apps/api/internal/modules/system/module.go`
  - `apps/api/internal/httpapi/server.go`
  - `apps/api/internal/httpapi/server_test.go`
  - `docs/progress/backend-system-access.md`

## 限制与下一接力点

- 当前 HTTP 组合使用空的内存 Repository，因此服务启动后需携带 `X-Dev-User-ID`，但系统列表默认为空；这是测试/开发实现，不是生产持久化。
- `X-Dev-User-ID` 仅做非空检查并被信任；真实认证接入时应替换 composition 中间件，验证 token、用户状态与审计身份，业务 handler 无需改为直接读 header。
- 新增成员时由于本竖切没有 identity 用户目录，`displayName` 暂以 `userId` 回填、`email` 省略、`status` 置为 active。接入 users Repository 后应解析真实展示信息，并拒绝不存在/禁用的平台用户。
- 本轮未实现 OpenAPI 中的 `POST /systems`，因为任务范围指定的是列表、详情、成员列表/更新；下一竖切可按平台级创建权限和审计契约补齐。
- 下一 session 建议：先为 MySQL Repository 写集成测试（显式 `system_id` / `user_id` 范围与 disabled membership），再实现持久化并在 `httpapi.New` 注入；随后为成员变更接入 append-only audit 服务。
