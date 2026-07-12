# 多 Session 开发进度台账

本目录用于跨 Session 接力。每个并行 Session 只维护自己的登记文件，主 Session 负责汇总状态和处理跨模块冲突。

## 状态总览

| Session | 当前内容 | 状态 | 进度文件 | 下一接力点 |
|---|---|---|---|---|
| root-orchestration | 模块拆分、统一契约、TDD 与集成 | completed | `root-orchestration.md` | 启动下一轮持久化联调 |
| frontend-shell | React+TS 路由、布局、API Client | completed | `frontend-shell.md` | 接入真实系统 API |
| go-backend-shell | Go 服务与领域模块骨架 | completed | `go-backend-shell.md` | 实现 System/Access 竖切 |
| mysql-contracts | MySQL 初始迁移、Scenario Schema | completed | `mysql-contracts.md` | 验证真实 MySQL 隔离 |
| backend-system-access | System/Access API 与内存 Repository | completed | `backend-system-access.md` | 实现 MySQL Repository |
| frontend-system-integration | 业务系统列表与 API 联调 | completed | `frontend-system-integration.md` | 对接真实 Go 服务与鉴权 |
| mysql-system-isolation | MySQL 开发环境与隔离测试 | completed | `mysql-system-isolation.md` | 启动 Docker 后执行真实集成 |
| backend-mysql-repository | Go System/Member MySQL Repository | completed | `backend-mysql-repository.md` | 接入真实身份认证 |
| local-db-integration | 本地数据库迁移与隔离验证 | completed | `local-db-integration.md` | 在 CI 补跑 MySQL 8.0 |
| system-e2e | Go + DB 真实系统隔离 E2E | completed | `system-e2e.md` | 扩展到前端浏览器 E2E |
| auth-session | Bearer 身份认证与开发模式隔离 | completed | `auth-session.md` | 接入真实用户目录与登录签发 |
| scanner-go-gin | Go/Gin API 与状态流转扫描 | completed | `scanner-go-gin.md` | 持久化扫描任务与 API Operation |
| scenario-importer | Scenario/Postman JSON 导入转换 | completed | `scenario-importer.md` | 接入系统级导入草稿与人工核验 |
| scanner-persistence-api | 扫描任务与 API Operation Repository/服务 | completed | `scanner-persistence-api.md` | 接入 HTTP 与共享数据库装配 |
| scenario-import-review | 场景导入草稿与人工核验服务 | completed | `scenario-import-review.md` | 接入 HTTP 与 RBAC |
| frontend-system-workspace | 系统级资产与场景工作区页面 | completed | `frontend-system-workspace.md` | 接入扫描与导入 API 数据 |
| scanner-http | 扫描/API 资产 HTTP 与 RBAC | completed | `scanner-http.md` | 前端创建扫描与受控 workspace |
| scenario-import-http | 导入/核验 HTTP 与 RBAC | completed | `scenario-import-http.md` | 前端上传、核验与发布 |
| frontend-workspace-data | 工作区扫描/导入真实数据接入 | completed | `frontend-workspace-data.md` | 接入 mutation 与分页 meta |
| root-http-integration | 共享路由、MySQL 装配与浏览器 E2E | completed | `root-http-integration.md` | 场景发现与执行主链路 |
| frontend-workflow-mutations | 扫描、导入、核验与发布操作页面 | completed | `frontend-workflow-mutations.md` | 接入场景发现与运行操作 |
| discovery-p0-scenarios | 代码/需求驱动的 P0 场景发现 | completed | `discovery-p0-scenarios.md` | accepted candidate 发布为场景 |
| execution-step-control | 截止步骤执行、单步重试与运行记录 | completed | `execution-step-control.md` | 接入 connector-aware executor |
| root-scenario-integration | 场景发现/执行共享装配与真实 E2E | completed | `root-scenario-integration.md` | 场景发布、真实执行器与前端闭环 |
| scenario-promotion | accepted candidate 发布为场景版本 | completed | `scenario-promotion.md` | 场景版本编辑与审计 |
| connector-http-executor | 受控 HTTP 步骤执行器 | completed | `connector-http-executor.md` | 环境/secret resolver 与 worker |
| frontend-discovery-runs | 场景发现、核验、执行与运行详情页面 | completed | `frontend-discovery-runs.md` | 分页、实时日志与审计 |
| root-end-to-end-scenario | 发布到真实 HTTP 执行的完整 E2E | completed | `root-end-to-end-scenario.md` | 异步 worker 与管理总览 |

## 下一批可接力 Session

| 建议 Session | 内容 | 前置进度 |
|---|---|---|
| scanner-persistence-api | 扫描任务、Operation 持久化与 system-scoped HTTP API | `scanner-go-gin.md`、`mysql-system-isolation.md` |
| scenario-import-review | 导入上传、环境/密钥绑定、人工核验与发布门禁 | `scenario-importer.md`、`auth-session.md` |
| frontend-system-e2e | 启动 Go + React，验证列表、角色和系统工作空间 | `system-e2e.md`、`frontend-system-integration.md` |
| identity-directory | 用户目录、登录签发与密钥轮换 | `auth-session.md` |

## 强制交接规则

每个 Session 完成一个可独立验收的内容后，必须更新自己的进度文件，至少记录：

1. 目标和范围。
2. 状态：`planned / in_progress / blocked / completed`。
3. Red：失败测试、失败原因和执行命令。
4. Green：通过测试、构建结果和执行命令。
5. 改动文件。
6. 新增或修改的接口/数据契约。
7. 已知限制和未完成内容。
8. 下一 Session 的建议入口。

禁止只登记“已完成”，也禁止把聊天记录当作唯一交接信息。

## 当前统一契约

- API 前缀：`/api/v1`。
- 成功响应：`{"data": ...}`。
- 失败响应：`{"error":{"code":"...","message":"...","details":...}}`。
- ID：UUID 字符串。
- 时间：API 使用 RFC 3339，数据库使用 UTC `DATETIME(3)`。
- 系统角色：`owner / maintainer / reviewer / runner / viewer`。
- 系统级 Repository 必须显式接收 `system_id` 或授权用户上下文。
- 开发流程：Red → Green → Refactor。
