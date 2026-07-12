# Session：frontend-workflow-mutations

## 基本信息

- 状态：completed
- 负责人/Session：frontend_system_workspace
- 开始时间：2026-07-12 09:18（Asia/Shanghai）
- 完成时间：2026-07-12 09:22（Asia/Shanghai）
- 修改范围：`apps/web/src/**`、`docs/progress/frontend-workflow-mutations.md`

## 目标

- 在单业务系统工作区接入扫描创建/执行、场景 JSON 上传、脚本确认和导入应用五类真实写操作。
- 展示提交中、成功和错误状态，避免重复提交；写入后刷新对应资源，刷新失败不得清空已有数据。
- 按 `myRole` 隔离写入口：owner/maintainer 管理扫描和导入，reviewer 可确认脚本，viewer 保持只读。

## Red

- 测试：ApiClient 新增五类写请求契约；新增 workflow mutation 测试覆盖非法 JSON、pending 期间重复提交、刷新失败保留旧数据、角色入口和提交反馈。
- 失败原因：`createScan` 等客户端方法不存在；`workflow-mutations` 操作模块不存在。
- 命令与摘要：`npm test -- --run src/api/client.test.ts src/pages/workflow-mutations.test.tsx`，客户端新增用例失败且 mutation suite 因模块缺失失败。

## Green

- 最小实现：新增类型化写请求；独立 WorkflowMutationPanel 提供扫描创建、创建后执行、JSON 文本上传、脚本确认和应用按钮；submission guard 防止未完成请求被重复触发；所有写成功后只刷新相关列表，刷新失败保留当前快照。
- 命令与摘要：核心相关测试 4 files / 20 tests；全量测试 8 files / 29 tests；`npm run build`（TypeScript + Vite）成功。

## 改动文件

- `apps/web/src/api/types.ts`
- `apps/web/src/api/client.ts`
- `apps/web/src/api/client.test.ts`
- `apps/web/src/pages/system.tsx`
- `apps/web/src/pages/workflow-mutations.tsx`
- `apps/web/src/pages/workflow-mutations.test.tsx`
- `apps/web/src/styles.css`
- `docs/progress/frontend-workflow-mutations.md`

## 契约决策

- 扫描：`POST /systems/{systemId}/scans`；`POST /systems/{systemId}/scans/{scanId}/run`。
- 导入：`POST /systems/{systemId}/scenario-imports`；结合当前后端系统隔离路由，确认和应用使用 `POST /systems/{systemId}/scenario-imports/{importId}/confirm-scripts|apply`。
- confirm/apply 不发送空 JSON body；后端接口不读取请求体。其他请求统一由 ApiClient 发送 JSON envelope。
- owner/maintainer：扫描创建/执行、上传、确认、应用；reviewer：确认脚本；runner/viewer：本操作区无写能力。场景执行权限仍由原有执行入口负责。

## 已知限制

- 场景文件当前通过 textarea 粘贴并在浏览器中先做 JSON 对象校验，尚未提供文件选择器、大小提示和客户端 schema 校验；服务端仍是最终校验者。
- 扫描采用“两步式”：先创建，再对本次创建的任务输入服务端可访问的 `repositoryRoot` 并执行；页面还不能选择历史 queued scan。
- 刷新请求失败时保留旧数据并继续显示写操作结果，但当前没有单独的“刷新失败”告警；成功文案只说明已请求刷新，不声称列表一定更新。
- Apply 表示应用导入结果，不等同于正式场景版本发布；正式发布仍需 Scenario 模块契约。
- 前端权限隐藏与重复提交保护不能替代后端 RBAC、幂等和并发控制。

## 下一接力点

- 增加真实浏览器交互测试，覆盖表单输入、按钮双击、HTTP 409/403、创建成功但刷新失败的组合状态。
- 场景上传增加文件选择、1 MiB 限制提示和导入 conversion report 展示；确认脚本前展示脚本风险明细。
- 扫描页接入 code source 下拉和历史 queued scan，禁止用户手输不受控的服务端路径。
- Scenario 模块明确 draft/apply/publish 边界后，再增加正式发布操作，避免把 importer apply 误称为发布。
