# 代码源管理与安全扫描选择器

更新时间：2026-07-12（Asia/Shanghai）

## 本轮完成

- 新增系统范围代码源 API：
  - `GET /api/v1/systems/{systemId}/code-sources`
  - `POST /api/v1/systems/{systemId}/code-sources`
  - `PUT /api/v1/systems/{systemId}/code-sources/{sourceId}`
- 所有系统成员可读取；仅 owner/maintainer 可创建和更新；非成员按跨系统隔离规则返回 404。
- 支持 Git 与本地目录代码源、默认引用、包含/排除路径、外部凭据引用和启停状态。
- 严格拒绝未知字段、非规范 UUID、路径穿越、Git URL 内嵌凭据以及明文 credential 值；`credentialRef` 只允许受支持的外部引用 scheme。
- 新增系统设置代码源管理界面；扫描和场景发现页面改为真实代码源下拉，不再要求用户手填 UUID。
- `code`/`mixed` 发现必须选择 active 代码源，`prompt`/`prd` 不提交无关代码源。
- 扫描创建会在服务端验证代码源属于当前系统且 active，避免只依赖数据库外键产生不明确的 500。

## 扫描目录信任边界

- 浏览器不再提交 `repositoryRoot`；运行接口只接受严格空对象 `{}`。
- 运行时根据已保存 scan 的 `codeSourceId` 再次读取 system-scoped 代码源，防止跨系统和停用状态绕过。
- 只有 active local 代码源可直接执行；Git 代码源在受控 checkout workspace 未实现前明确返回 409，页面也不展示误导性的执行按钮。
- 新增 `SCANNER_ALLOWED_ROOTS` CSV 服务端配置，空值默认 deny-all。
- 本地目标与允许根均执行绝对路径、`EvalSymlinks` 和真实目录检查，再通过 `filepath.Rel` 验证目录边界；越界、不存在、无有效根或符号链接逃逸统一不可运行，Analyzer 不会被调用。
- Analyzer 只接收解析后的真实目标目录。

## 验证

- API：`go test -count=1 ./...` 通过。
- API：`go vet ./...` 通过。
- Web：19 个测试文件、96 项测试通过。
- Web：TypeScript 检查和 `/bizdevops/` 生产构建通过。
- Python DB/腾讯云部署静态契约：22 项通过。

## 腾讯云试运行部署

- API/Worker 已切换到 `20260712164546-functional`；Web 在修复系统设置样式契约后切换到 `20260712170231-settings-layout`。API、Worker、Nginx、MariaDB 均 active。
- 由于公网仍无可信 TLS，API 继续使用 `AUTH_MODE=development`，Basic Auth 保持启用，正式 JWT 登录未公开切换。
- 最新 Web bundle 不含 development 用户 ID；Nginx 在受 Basic Auth 保护的 `/bizdevops/api/` location 覆盖注入固定试运行身份，避免客户端携带或伪造身份 Header。
- `deploy/tencent/api.env.example` 只新增空的 `SCANNER_ALLOWED_ROOTS=`，未写入真实服务器路径或秘密。
- 远端当前未配置扫描允许根，因此本地扫描按设计拒绝执行；配置时必须由服务器运维者设置最小化目录范围。
- Git 源当前可登记、选择并创建追踪记录，但不能执行；后续需实现受控 clone/fetch、host allowlist、凭据 Provider、版本固定和工作区清理。
- 远端 smoke test：系统、代码源、环境和成员 API 均为 200；未认证 `/bizdevops/` 为 401；原 `/`、`/api`、`/ai-data/` 分别保持 200/404/200。
- 样式故障根因是新设置组件使用的页面、卡片和表单 class 未在主样式表定义；新增直接读取 CSS 的契约测试，防止只验证 HTML 文案而漏掉视觉退化。
- 回滚目标：bin `20260712031205`、Web `20260712033413-identity-hotfix`，Nginx 已保存带 release 标识的备份。
