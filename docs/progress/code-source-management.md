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

## 部署说明

- 本轮未切换腾讯云公网 release，现有试运行服务保持不变。
- `deploy/tencent/api.env.example` 只新增空的 `SCANNER_ALLOWED_ROOTS=`，未写入真实服务器路径或秘密。
- 以后部署新 API 时，必须由服务器运维者设置最小化允许根；未设置时本地扫描按设计拒绝执行。
- Git 源当前可登记、选择并创建追踪记录，但不能执行；后续需实现受控 clone/fetch、host allowlist、凭据 Provider、版本固定和工作区清理。
