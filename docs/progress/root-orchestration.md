# Session：root-orchestration

## 基本信息

- 状态：completed
- 修改范围：共享契约、总体文档、集成验证和进度汇总

## 已完成内容

- 确认 React + TypeScript、Go、MySQL 8.0 技术栈。
- 完成模块边界和依赖规则。
- 建立多 Session 台账与交接模板。
- 建立 System & Access OpenAPI 0.1 契约。
- 启动后端、前端、MySQL 三个互不覆盖的并行 Session。

## 当前契约决策

- System API 以 `contracts/openapi/system-api.yaml` 为准。
- 未授权系统详情返回 404，避免泄漏系统存在性。
- 业务系统响应必须返回当前用户的 `myRole`。
- 开发流程强制 Red → Green → Refactor。

## 已知限制

- 当前机器没有全局 Go；后端 Session 使用临时 Go runtime 验证。
- Docker daemon 此前未运行，真实 MySQL 执行能力由当前 MySQL Session 再次检查。

## 集成 Green

- Go：`go test ./...` 与 `go vet ./...` 通过。
- Web：5 个测试文件、12 个测试通过；生产构建通过。
- MySQL：18 张表、17 个 scoped foreign keys；5 个隔离契约测试通过。
- Contracts：Scenario Bundle 与 System OpenAPI 验证通过。
- 实际 MySQL 容器测试未执行，原因是 Docker daemon 未启动；强制 CI 命令已登记。

### 第二轮真实数据库 Green

- 本地服务识别为 MariaDB 12.2.2，作为 MySQL 协议兼容环境使用；迁移 SQL 未为 MariaDB 特化。
- 真实完成 up → seed → 8 项静态/隔离验证 → role enum → down，临时库已清理。
- Go MySQL Repository 已替换空内存装配，保留未配置 DSN 时的开发回退。
- System API 真实数据库 E2E 1/1 通过，覆盖用户系统隔离、404、403、Owner 更新和持久化回读。
- E2E Red 发现并修复成员更新响应错误回显 userId 的问题。
- Go 全量测试和 vet 再次通过。

### 第三轮能力 Green

- 生产 Bearer JWT 已落地：校验 HS256、issuer、audience、`exp` 与 UUID `sub`；开发身份头仅在显式 `AUTH_MODE=development` 时启用。
- 新增 `/api/v1/me`，JWT 模式不提供默认密钥；真实数据库 E2E 已显式切换开发鉴权并再次通过。
- Go/Gin AST 扫描器已支持五种 HTTP 方法、源码定位、稳定 JSON 输出和任务状态机；真实 OMS 样例识别 3 个 API。
- Scenario Bundle 1.0 / Postman 2.1 转换器已支持嵌套请求、变量、脚本复核、密钥脱敏、未绑定变量和结构化导入报告。
- Go `test ./...`、Go `vet ./...`、Web 12 项测试与生产构建全部通过。

### 第四轮系统工作流 Green

- 扫描任务、API Operation 和场景导入草稿已提供 system-scoped Memory/MySQL Repository 与应用服务。
- Scanner/Importer HTTP 已接入成员隔离和角色矩阵，配置 MySQL 时共享 Server 使用真实持久化实现。
- React 单系统工作区展示真实扫描/API/导入数据，并分别处理 loading/error/empty/ready；当前为 7 个测试文件、22 项测试。
- 真实数据库 E2E 增加三类资源隔离、Postman 上传、Viewer 禁止发布与 Owner 发布。
- 浏览器验证 Owner/Viewer 页面与操作可见性通过，无控制台错误；临时 QA 进程与数据库已清理。
- 严格 UUID E2E 发现并修正开发 seed 中无效的 version/variant 位。

### 第五轮场景闭环 Green

- React 已支持创建/运行扫描、上传 Postman、确认脚本和应用导入，并覆盖重复提交、非法 JSON 与刷新失败保留。
- 场景发现可在没有 PRD 时仅用代码 API Operation 生成 P0 候选，并支持一句话/PRD/mixed 增强。
- 候选必须人工 accepted/rejected；成员读取、发起和核验角色权限均为 system-scoped。
- 执行 Service 支持 stopAfterStepId、失败即停、单步 retry 与 attempt 历史，运行快照和错误已脱敏。
- MySQLStepProvider、Execution/RunRecord Repository 与 HTTP 已接入共享 Server；无 connector executor 时安全返回 503。
- 真实数据库 E2E 已覆盖代码-only P0、人工核验、执行安全失败和 run/attempt 查询。

### 第六轮真实执行 Green

- accepted candidate 可原子发布为 Scenario/Version/Steps，并支持幂等重复发布。
- HTTP executor 强制 host allowlist、SSRF/redirect/DNS/IP 双重检查、timeout/体积限制和脱敏。
- React 已覆盖四类发现输入、P0 核验/发布、场景版本/步骤选择、stop-after、run detail 与单步 retry。
- Server 仅在显式配置 allowed hosts 时启用 HTTP executor；默认仍安全禁用。
- 真实 MySQL E2E 使用临时本地业务 API，完整验证“代码资产 → P0 → accept → promote → HTTP step → assertion → passed run”。

### 第七轮运营能力 Green

- 异步 Worker 支持 queue、lease、heartbeat、过期重排、取消、超时与 retry key，并提供独立可运行命令。
- Environment/Secret Resolver 只保存外部 reference，新增 `000002_environment_secrets` migration，真实数据库隔离通过。
- 管理总览由后端按成员授权或 platform admin 范围聚合，React 不跨系统自行汇总。
- 真实 E2E 覆盖 secret reference 角色可见性、member/outsider/admin 管理范围。
- 浏览器验证管理 Dashboard、系统筛选和单系统摘要，无 console warning/error。

## 下一接力点

- 将扫描器接入 system-scoped API 和 MySQL，形成扫描任务、API Operation 与版本差异。
- 将 importer 接入导入草稿、环境/secret 绑定、人工核验和发布门禁。
- 完成真实用户目录与登录签发，并补齐 React 浏览器 E2E。
- 完成扫描创建/导入核验 mutation 页面，再推进场景发现、发布、执行器和运行详情。
