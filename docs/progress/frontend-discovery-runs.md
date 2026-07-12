# Session：frontend-discovery-runs

## 基本信息

- 状态：completed
- 负责人/Session：frontend_system_workspace
- 开始时间：2026-07-12 09:38（Asia/Shanghai）
- 完成时间：2026-07-12 09:43（Asia/Shanghai）
- 修改范围：`apps/web/src/**`、`docs/progress/frontend-discovery-runs.md`

## 目标

- 完成单业务系统内“场景发现 → 候选核验 → 场景运行 → 运行详情/步骤重试”的前端链路。
- 明确 code-only、一句话、PRD、mixed 四种发现输入，候选展示 P0、置信度、来源和人工核验要求。
- 支持运行到指定步骤、输入变量、attempt/assertion 详情和单步骤 retry，并保持系统角色隔离。

## Red

- 测试：扩展 ApiClient 测试覆盖 discovery/candidate/scenario/run/retry 路径与 body；增加系统内运行详情路由测试；新增页面测试覆盖四种输入、候选证据与权限、运行状态、截止步骤、attempt/assertion/retry 和输入变量 JSON 校验。
- 失败原因：客户端 discovery/run 方法不存在；discovery/runs 页面模块不存在；`runs/:runId` 被通配路由接管。
- 命令与摘要：`npm test -- --run src/api/client.test.ts src/routes.test.tsx src/pages/discovery-runs.test.tsx`，3 个测试文件均按预期失败。

## Green

- 最小实现：新增领域 DTO 和类型化客户端；DiscoveryPage、CandidateQueue、RunsPage、RunDetailPage 接入真实接口；所有列表具备 loading/error/empty/ready，mutation 具备 submitting/success/error；刷新失败保留旧 state；新增系统内 run detail 路由。
- 命令与摘要：首轮针对性测试 3 files / 15 tests；scenario promotion 小接力后全量测试 9 files / 38 tests；`npm run build`（TypeScript + Vite）成功。

## 改动文件

- `apps/web/src/api/types.ts`
- `apps/web/src/api/client.ts`
- `apps/web/src/api/client.test.ts`
- `apps/web/src/routes.tsx`
- `apps/web/src/routes.test.tsx`
- `apps/web/src/pages/discovery.tsx`
- `apps/web/src/pages/runs.tsx`
- `apps/web/src/pages/discovery-runs.test.tsx`
- `apps/web/src/styles.css`
- `docs/progress/frontend-discovery-runs.md`

## 契约决策

- Discovery：`GET|POST /systems/{systemId}/discoveries`；`GET .../{discoveryId}/candidates`；`POST .../{candidateId}/accept|reject`，核验 body 为 `{ note }`。
- 发现类型与后端一致：`code`、`prompt`、`prd`、`mixed`。code/mixed 提交前读取当前系统 API operations 并作为 operations 输入，不拼接跨系统资产。
- Execution/Run Record：`POST|GET /systems/{systemId}/scenario-runs`、`GET .../{runId}`、`POST .../{runId}/steps/{stepId}/retry`；执行 body 支持 `stopAfterStepId`、`inputVariables`。
- Scenario：接入 `GET /systems/{systemId}/scenarios`、`GET .../scenarios/{scenarioId}`；运行页优先用详情中的 current version 和 ordered steps 生成选择器，详情部分失败时保留手工 scenario/version/step ID fallback。
- Promotion：`POST /systems/{systemId}/discoveries/{discoveryId}/candidates/{candidateId}/promote` 必须发送空 JSON 对象；仅 owner/maintainer 可见，`created=false` 明确提示幂等返回原场景。
- 权限：owner/maintainer 可创建发现、核验、执行和 retry；reviewer 仅核验；runner 仅执行和 retry；viewer 只读。

## 已知限制

- 候选聚合需要先查 discoveries 再逐项查 candidates；部分 candidate 请求失败时会保留其他成功候选，但目前没有单独展示“部分来源加载失败”警告。
- accepted candidate 可发布为带首版和 ordered steps 的场景草稿；当前尚未在发布成功后自动跳转场景详情/编排页。
- 场景列表可用时提供场景、版本和截止步骤选择；environment 仍需手工填写，场景详情部分失败时回退到手工 version/step ID。
- 当前运行接口是同步响应，页面未增加轮询/SSE；运行时间较长时只能看到 submitting。
- attempt 页面展示断言与错误，但请求/响应快照尚未展开，以避免在脱敏契约确认前展示潜在敏感内容。
- 前端角色隐藏不替代后端 RBAC；错误刷新不会清空旧数据，但没有统一 stale-data 标记。

## 下一接力点

- 增加场景详情/编排路由，发布成功后跳转新场景，并将 environment 改成授权环境选择器。
- Candidate 聚合改为后端系统级待办查询或带 partialErrors 的批量接口，消除 N+1 和静默部分失败。
- 增加运行 SSE/轮询、取消、从失败步骤继续；详情增加已脱敏 request/response snapshot 展开区。
- 增加真实浏览器交互测试，覆盖动态输入、accept/reject note、invalid inputVariables、stopAfterStepId 和 retry 409。

## 小接力：scenario promotion

- Red：先扩展客户端契约测试和视图测试；`promoteCandidate` 缺失、accepted 候选无发布按钮、运行表单仍要求手输版本/步骤，共 3 个新增行为失败。
- Green：补齐 Scenario/Version/Step/Promotion DTO、candidate promote 与 scenario detail 客户端；accepted 候选仅 owner/maintainer 显示“发布为场景”，共享 submission guard 防重复，按 `created` 区分首次发布与幂等结果；运行页批量读取 scenario detail 并优先展示 current version/step selectors。
- 验证：小接力针对性测试 2 files / 14 tests；最终全量 9 files / 38 tests；最终 TypeScript + Vite build 成功。
