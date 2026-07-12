# Session：frontend-system-workspace

## 基本信息

- 状态：completed
- 负责人/Session：frontend_system_workspace
- 开始时间：2026-07-11 23:18（Asia/Shanghai）
- 完成时间：2026-07-11 23:20（Asia/Shanghai）
- 修改范围：`apps/web/src/**`、`docs/progress/frontend-system-workspace.md`

## 目标

- 将单业务系统首页改造成可理解的工作区，覆盖代码扫描与 API 资产、场景导入与人工核验、场景列表与编排、执行与结果四组入口。
- 始终展示当前授权系统和 `myRole`，按角色隐藏不允许的操作。
- 不伪造跨系统或单系统统计数据，后端资产接口尚未落地时明确展示空状态。

## Red

- 测试：新增 `apps/web/src/pages/system-workspace.test.tsx`，先覆盖加载/错误/空数据、当前系统、入口路径及五类角色能力；随后增加模块页上下文与按钮权限回归。
- 失败原因：第一轮 `SystemWorkspaceView` 未实现，5/5 失败；第二轮 `SystemModulePageView` 未实现，新增用例失败。
- 命令与摘要：`npm test -- --run src/pages/system-workspace.test.tsx`；两轮均在实现前确认失败。

## Green

- 最小实现：`SystemLayout` 通过 Outlet 透传已授权系统状态；工作区使用真实 `BusinessSystem`，展示四组空状态和系统内链接；角色能力矩阵控制扫描、导入、核验、编辑、执行按钮；各模块页也使用当前系统上下文并隐藏越权按钮。
- 命令与摘要：`npm test -- --run src/pages/system-workspace.test.tsx`（6/6）；`npm test -- --run`（6 files / 18 tests）；`npm run build`（TypeScript + Vite 成功）。

## 改动文件

- `apps/web/src/layouts/SystemLayout.tsx`
- `apps/web/src/pages/system.tsx`
- `apps/web/src/pages/system-workspace.test.tsx`
- `apps/web/src/styles.css`
- `docs/progress/frontend-system-workspace.md`

## 契约决策

- 继续使用现有 `GET /api/v1/systems/:systemId` 返回的 `BusinessSystem.myRole` 作为前端可见性依据，不新增未经后端确认的统计接口。
- 权限矩阵：`owner`、`maintainer` 可扫描/导入/核验/编辑/执行；`reviewer` 仅增加核验操作；`runner` 仅增加执行操作；`viewer` 只读。
- 所有链接带当前 `systemId`，资产、场景和执行信息不得回退到全局模拟数据。

## 已知限制

- 前端按钮隐藏只改善交互，不能代替后端逐接口鉴权。
- 扫描、API、场景和运行列表接口尚未形成可用契约，因此当前展示明确空状态；没有编造数量、成功率或最近活动。
- 目前“查看场景列表”复用 `discovery` 路由；后续可在场景查询接口落地时拆成独立列表路由。
- `SystemLayout` 的错误状态当前无页面内重试按钮，可在 ApiClient 增加可取消/重试策略后一并补齐。

## 下一接力点

- 后端提供系统范围的扫描、API、场景摘要和运行列表接口后，先补 ApiClient 契约测试，再将四个空状态分别升级为 loading/empty/error/ready 数据视图。
- 将后端返回的 capability/permission 集合替代前端静态角色推导，避免未来自定义角色导致前后端漂移。
- 场景列表落地独立路由，并把编排器的单步骤编辑、执行至某一步和导入报告接入对应入口。
