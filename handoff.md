# BizDevOps 平台接力说明

更新时间：2026-07-12（Asia/Shanghai）

## 1. 当前状态

腾讯云试运行部署已经完成并稳定运行：

- Web：`/bizdevops/`
- API：仅监听 `127.0.0.1:18080`
- API systemd：`bizdevops-api.service`
- OMS Worker：`bizdevops-worker@20000000-0000-4000-8000-000000000001.service`
- 当前公网入口仍为 HTTP `:8080`，Nginx Basic Auth 保持启用。
- 当前线上 API 仍为 `AUTH_MODE=development`，Web release 为 `20260712033413-identity-hotfix`。
- 当前线上修复已确保生产 bundle 发送试运行 `X-Dev-User-ID`；正式登录代码尚未切换到公网服务。

正式登录/JWT 代码、前端登录页、Cookie 会话、部署模板和测试已经完成。由于入口尚无 TLS，按安全边界不得把正式密码登录切到公网 HTTP；下一阶段必须先完成 HTTPS/TLS。

## 2. Git 与本轮变更

- 仓库：`git@github.com:ryan4good/api-data.git`
- 分支：`feature/platform-refactor-e2e`
- 远端：`origin/feature/platform-refactor-e2e`
- 最近已推送提交：
  - `1377d8e feat: deploy BizDevOps Tencent trial`
  - `2a07418 fix: preserve trial identity in production build`
  - `38e6f0f feat: add secure JWT login flow`
  - `ad15fbb feat: complete system overview with real data`
  - `f3ccec5 feat: complete system operational pages`
  - `99ef5ad feat: refine scenario and operations workflows`

本轮正式认证变更包括：

- API：邮箱/密码登录、bcrypt、JWT 签发、`/auth/me`、`/auth/logout`。
- API：Bearer 与 HttpOnly Cookie 双通道；JWT 模式拒绝 development Header。
- API：未知账号执行 dummy bcrypt，错误密码/未知账号/disabled 用户统一 401。
- API：用户状态每次认证重新检查，disabled 用户已签发 token 也会失效。
- Web：`/login`、受保护路由、真实用户展示、退出、401 会话清理。
- Web：只使用 HttpOnly Cookie，不将 token 写入 localStorage/sessionStorage，也不主动发送 Bearer。
- 部署：JWT/Cookie env 模板、Nginx 登录限流、TLS 切换与回滚约束。
- 集成修复：过期/轮换前 Cookie 不再阻止重新登录或退出清 Cookie。
- 单系统概览：场景和运行卡片已接真实 scoped API，不再硬编码空状态。
- 管理摘要：新增成员、环境、代码源和最近运行指标；缺失值保持未知而非伪造 0。
- 代码扫描页：真实列表、创建与执行，写操作只对 owner/maintainer 展示。
- API 资产页：真实资产表格与 method/status/path 筛选，无虚假写按钮。
- 系统设置页：真实环境与 external secret reference 管理，Viewer 引用位置脱敏。
- 场景编排器：指定场景可加载并保存不可变人工修订版本，owner/maintainer 可写，其他角色严格只读。
- 全局运行记录：跨授权系统并发聚合真实运行，支持部分失败与真实运行详情。
- 系统成员管理：仅 owner 读取成员目录并添加已有用户或更新系统角色。
- 工作区场景记录已链接到版本编辑器；无空白创建契约时提供诚实的候选核验/导入引导。
- 已删除未使用的静态假仪表盘和无后端契约的“新建业务系统”按钮。
- 代码源管理：系统成员可查看，owner/maintainer 可新增和更新 Git/本地代码源；凭据只允许外部引用。
- 扫描与场景发现已使用真实代码源下拉，不再要求手填 UUID。
- 扫描执行不再接受浏览器提交的服务器路径，只从 system-scoped 代码源解析。
- 本地扫描受 `SCANNER_ALLOWED_ROOTS`、真实目录和符号链接边界约束；空 allowlist 默认拒绝全部执行。
- Git 代码源在受控 checkout workspace 未实现前只可登记和创建追踪记录，不能直接执行。

敏感文件 `docs/tencent.txt` 已由 `.gitignore` 精确忽略。绝不能输出内容、暂存或提交。所有 shell 命令仍必须按 `C:\Users\ryanf\.codex\RTK.md` 以 `rtk` 开头。

## 3. 正式登录 HTTP 契约

### 登录

`POST /api/v1/auth/login`

请求：

```json
{"email":"user@example.com","password":"..."}
```

成功响应 envelope 的 `data` 包含：

- `accessToken`：保留给非浏览器 API 客户端。
- `tokenType`：`Bearer`。
- `expiresIn`：秒。
- `user`：`id/email/displayName/platformRole`。

浏览器同时收到 JWT Cookie。Cookie 必须为 HttpOnly、SameSite=Strict；公网切换时必须 Secure。

### 当前用户与退出

- `GET /api/v1/auth/me`：Bearer 或 Cookie JWT；再次确认数据库用户仍为 active。
- `POST /api/v1/auth/logout`：清 Cookie，返回 `{"data":null}`。
- login/logout 是旧 Cookie 恢复端点：即使请求带过期或已轮换 token，也必须能重新登录或清 Cookie。

## 4. 为什么使用 Cookie 而不是浏览器 Bearer

Nginx Basic Auth 与 Bearer JWT 都使用 `Authorization` Header。浏览器无法在同一个请求里同时发送 `Basic ...` 和 `Bearer ...`，因此“双层 Basic + 浏览器 Bearer”必然失败。

正式方案是：

- Nginx Basic Auth 暂时继续使用 `Authorization: Basic ...`。
- 浏览器 JWT 使用 HttpOnly Cookie。
- 后端仍兼容 Bearer，供没有 Nginx Basic 的非浏览器客户端或后续正式网关使用。

这解决 Header 冲突，但不解决 HTTP 明文传输。因此公网切换仍被 TLS 阻塞。

## 5. 已通过验证

### 本地

- API：`go test -count=1 ./...` 通过。
- API：`go vet ./...` 通过。
- Web：19 个测试文件、96 项测试通过。
- Web：`VITE_BASE_PATH=/bizdevops/` 且不设置 `VITE_DEV_USER_ID` 的生产构建通过。
- 部署契约覆盖 JWT 必需配置、安全 Cookie、禁止 Web Storage token、登录限流、TLS 门禁和回滚。
- 场景人工修订覆盖 RBAC、跨系统隔离、严格 JSON、依赖 DAG、MySQL 事务/CAS 和 `requestConfig` JSON 对象序列化。
- Python DB/部署静态契约：22 项通过。
- 代码源覆盖严格 JSON、RBAC/跨系统隔离、Git/本地互斥、外部凭据引用、重名冲突和 MySQL system scope。
- 扫描目录覆盖服务端 allowlist、缺省 deny-all、路径边界和 symlink 逃逸。

### 远端真实 MariaDB staging

未切换公网服务，而是在 `127.0.0.1:18081` 启动一次性 staging API 完成：

- 错误密码返回统一 401。
- 正确密码登录成功并设置 HttpOnly/SameSite=Strict Cookie。
- Cookie `/auth/me` 成功。
- Bearer `/auth/me` 兼容成功。
- Cookie 管理总览成功，RBAC 数据路径可用。
- JWT 模式拒绝 `X-Dev-User-ID`。
- 带失效 Cookie 的 logout 仍能清理会话。
- logout 后 `/auth/me` 返回 401。

staging unit、env、Cookie jar、临时 release 已清理，测试用户原 password hash 已恢复，公网 `18080` 服务未改变。

### 远端真实系统概览 staging

新 Linux amd64 API 还在 `127.0.0.1:18082` 做过一次性真实 MariaDB 验收：

- 系统摘要返回 `memberCount/environmentCount/codeSourceCount/lastRunAt`。
- 计数字段为数字；无运行时 `lastRunAt` 明确为 null。
- 场景列表和运行列表 scoped endpoint 均返回数组 envelope。
- staging unit、env 和 release 已清理，公网服务未切换。

### 当前线上回归

- API 与 OMS Worker active。
- `127.0.0.1:18080` 仅回环监听。
- Nginx 与 MariaDB active，`nginx -t` 通过。
- 原 `/` 为 200、`/api` 为 404、`/ai-data/` 为 200。
- 当前 `/bizdevops/` 仍受 Basic Auth 保护并运行 development identity release。

## 6. 安全边界

在以下条件全部满足前，不得切换正式登录公网服务，也不得移除 Basic Auth：

1. `/bizdevops/` 已有浏览器信任的 HTTPS/TLS，不是自签名警告页。
2. `AUTH_COOKIE_SECURE=true`。
3. `AUTH_COOKIE_PATH=/bizdevops/`，不得把 JWT Cookie 发送给同源其他应用。
4. JWT signing key 至少 32 个随机字节，只存远端受限 env。
5. 试运行账号密码随机生成，数据库只保存 bcrypt hash。
6. `/bizdevops/api/v1/auth/login` Nginx 限流生效并经过 `nginx -t`。
7. HTTPS 下完成登录、Cookie、退出、过期、disabled、越权和跨系统隔离验收。
8. 日志确认无密码、token、Cookie、DSN 或 signing key 泄漏。

当前没有 443 监听，也没有已知可用域名/证书。下一接手者不得猜测域名或部署自签名证书冒充完成；需要用户提供域名/DNS 控制，或明确选择可信 TLS 终止方案。

## 7. 下一步推荐顺序

1. 确认试运行域名及 DNS 控制方式。
2. 在不影响 80 端口现有 Go 服务的前提下，为 BizDevOps 配置可信 HTTPS；优先独立域名/端口或现有网关 TLS 终止。
3. 安装 `deploy/tencent/nginx-bizdevops-http.conf` 到 Nginx `http` context include，并确认登录限流 zone 生效。
4. 生成随机 JWT signing key、试运行登录密码和 bcrypt hash；秘密仅通过 Paramiko stdin/SFTP 传输。
5. 交叉编译新 API、构建不含 `VITE_DEV_USER_ID` 的 Web，上传新 versioned release。
6. 先在 HTTPS + Basic Auth 双层状态验收 Cookie JWT。
7. 验收全部通过后，另一次原子变更移除 `/bizdevops/` 与 API 的 Basic Auth；失败立即恢复。
8. 补浏览器 E2E：登录、刷新会话、退出、错误密码、过期 Cookie、跨系统越权。

## 8. 腾讯云部署与回滚原则

- SSH 只通过 Paramiko 读取本地凭据，严格校验 handoff 已记录的 host key；不得 AutoAddPolicy。
- 所有上传先到 root-only staging，再进入版本化 `/opt/bizdevops/releases/<release>`。
- API/Web 使用符号链接原子切换；切换前记录旧目标。
- 修改 Nginx 前备份站点、server snippet 和 http-context rate-limit 文件。
- 只有 `nginx -t` 成功后才能 reload。
- API 切换失败：恢复旧 env、旧 binary symlink，重启 BizDevOps API。
- Web 切换失败：恢复 `/var/www/bizdevops` 旧 symlink。
- Nginx 失败：恢复全部备份，`nginx -t` 后 reload。
- 数据库 password hash 切换失败：通过 stdin 恢复旧 hash；不得把 hash 或明文密码写入日志。
- 不得停止、覆盖或重启 stock-analyzer、ai-data-mvp、rent-platform，也不得触碰 80 端口现有 Go 程序。
- 独立试运行数据库不自动删除；删除必须另获用户授权。

## 9. 相关文档

- `docs/progress/tencent-trial-deployment.md`
- `docs/progress/authentication-jwt-login.md`
- `docs/progress/system-overview-real-data.md`
- `docs/progress/system-operational-pages.md`
- `docs/progress/functional-refinement.md`
- `docs/progress/code-source-management.md`
- `docs/progress/root-end-to-end-scenario.md`
- `docs/progress/root-operations-integration.md`
- `deploy/tencent/README.md`

## 10. 产品后续

TLS 与正式登录上线后，优先继续：

1. audit event 写入器和管理端审计页。
2. Vault/AWS/GCP SecretProvider adapter。
3. 多 Worker 并发 E2E、容量限制、指标与告警。
4. 密码重置、管理员用户管理、多因素认证或外部 IdP/OIDC。
5. 为平台设置和新建业务系统设计 platform admin/auditor 服务端授权与写接口，再替换当前诚实占位说明。
6. 实现 Git 代码源的受控 clone/fetch workspace、仓库 host allowlist、外部凭据 Provider 和清理策略。
