# Session：frontend-workspace-data

## 基本信息

- 状态：completed
- 负责人/Session：frontend_system_workspace
- 开始时间：2026-07-11 23:24（Asia/Shanghai）
- 完成时间：2026-07-11 23:27（Asia/Shanghai）
- 修改范围：`apps/web/src/**`、`docs/progress/frontend-workspace-data.md`

## 目标

- 接入当前业务系统的扫描记录、API 资产和场景导入记录，不使用模拟数量。
- 三类资源并行加载且分别呈现 loading/error/empty/ready；单个请求失败不能清空其他成功资源。
- 保留既有角色操作可见性；POST 契约未明确前不产生写请求。

## Red

- 测试：扩展 `api/client.test.ts` 锁定三个系统级 URL 与 `systemId` 编码；新增 `workspace-data.test.tsx` 覆盖独立加载态、部分失败、真实数量/列表摘要和并行资源加载。
- 失败原因：ApiClient 尚无 `listScans`；工作区忽略 resources；`loadWorkspaceResources` 未实现。
- 命令与摘要：`npm test -- --run src/api/client.test.ts src/pages/workspace-data.test.tsx`，确认 4 个新增行为失败、既有 3 个测试通过。

## Green

- 最小实现：新增 ScanRun/ApiOperation/ScenarioImport DTO；ApiClient 增加三个 GET；工作区通过 `Promise.allSettled` 并行加载并保留成功分支；每类资源展示加载、错误、空数据、真实 count 和最多三条摘要。
- 命令与摘要：针对性测试 3 files / 13 tests；全量测试 7 files / 22 tests；`npm run build`（TypeScript + Vite）成功。

## 改动文件

- `apps/web/src/api/types.ts`
- `apps/web/src/api/client.ts`
- `apps/web/src/api/client.test.ts`
- `apps/web/src/pages/system.tsx`
- `apps/web/src/pages/workspace-data.test.tsx`
- `apps/web/src/styles.css`
- `docs/progress/frontend-workspace-data.md`

## 契约决策

- 固定读取端点：`GET /systems/{id}/scans`、`GET /systems/{id}/api-operations`、`GET /systems/{id}/scenario-imports`，均使用现有 `/api/v1` base URL 和 `{ data: [...] }` envelope。
- 列表响应按数组处理；服务端若把空列表编码为 `null`，前端容错为空数组。
- 扫描和导入入口仍受 `myRole` 能力矩阵控制；本 session 未调用任何 POST，因为 code source、文件上传形式和幂等契约尚未确认。

## 已知限制

- `Promise.allSettled` 保证部分失败隔离，但当前会在三项均完成后一次更新最终摘要；慢资源不会导致已成功数据丢失，但会延后最终展示。
- 只展示最多三条摘要，不含分页；总数当前是本次返回数组长度，后端引入分页后应读取 `meta.total`。
- 尚无固定场景列表和运行列表端点，因此这两张卡继续显示明确空状态，没有模拟数据。
- 前端角色可见性不替代后端 GET/POST 权限校验。

## 下一接力点

- 后端 HTTP 契约测试确认三类 DTO 字段与空数组行为，并补前后端集成测试。
- 明确扫描创建的 `codeSourceId/sourceRef`、导入上传的 multipart/JSON、文件限制及幂等键后，先补失败测试再实现表单 POST。
- 引入分页契约时将列表状态扩展为 `items + total + nextCursor`，避免把首屏长度误作总数。
