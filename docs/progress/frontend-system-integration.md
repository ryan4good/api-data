# Session：frontend-system-integration

## 基本信息

- 状态：completed
- 负责人 Session：`/root/frontend_system_integration`
- 开始时间：2026-07-11
- 修改范围：`apps/web/**`、`docs/progress/frontend-system-integration.md`

## 目标

- 按 `contracts/openapi/system-api.yaml` 接入授权业务系统列表与详情。
- 系统列表呈现 `myRole`，覆盖 loading / error / empty / ready，并可进入系统工作空间。
- API transport 可替换，测试不访问真实后端。

## 进度登记

### 1. API client 与系统列表视图测试（Red）

- 状态：completed
- 测试：为 `listSystems`、注入 transport、授权系统列表四态、角色和工作空间链接新增测试。
- 失败原因：现有 client 不接受 `transport` 且没有 `listSystems`；契约类型缺少 `code`、`myRole` 等必填字段；`SystemsPageView` 尚不存在。
- Red 命令：`npm test -- --run`
- 实际摘要：2 个测试文件失败、5 个测试失败；失败点分别为 transport 未被使用、`listSystems` 不存在、`SystemsPageView` 未导出。

### 2. API client 与授权系统列表（Green）

- 状态：completed
- 最小实现：补齐契约类型；client 支持 `transport` 注入并提供 `listSystems`；列表接入 API，渲染 loading / error / empty / ready、`myRole` 与工作空间链接。
- Green 命令：`npm test -- --run`
- 实际摘要：4 个测试文件、10 个测试全部通过。

### 3. 工作空间详情视图（Red）

- 状态：completed
- 测试：新增工作空间详情 loading / error / ready 与角色展示测试。
- 失败原因：`SystemContextView` 尚不存在，现有侧栏仍按 URL 文本硬编码名称。
- Red 命令：`npm test -- --run`
- 实际摘要：1 个测试文件失败、2 个测试失败，其余 10 个测试保持通过。

### 4. 工作空间授权详情（Green）

- 状态：completed
- 最小实现：`SystemLayout` 按路由 `systemId` 调用 `getSystem`，侧栏显示真实名称、code、`myRole` 和 active/archived 状态；覆盖 loading / error / ready。
- Green 命令：`npm test -- --run`、`npm run build`
- 实际摘要：5 个测试文件、12 个测试全部通过；TypeScript 编译与 Vite 生产构建通过。

## Red

- 已运行 Red：`2 failed | 2 passed` 测试文件，`5 failed | 5 passed` 测试。
- 第二轮 Red：`1 failed | 4 passed` 测试文件，`2 failed | 10 passed` 测试。

## Green

- 第一轮 Green：`4 passed` 测试文件，`10 passed` 测试。
- 最终 Green：`5 passed` 测试文件，`12 passed` 测试；生产构建成功（34 modules transformed）。

## 改动文件

- `apps/web/src/api/client.test.ts`
- `apps/web/src/api/client.ts`
- `apps/web/src/api/types.ts`
- `apps/web/src/layouts/SystemLayout.test.tsx`
- `apps/web/src/layouts/SystemLayout.tsx`
- `apps/web/src/pages/global.tsx`
- `apps/web/src/pages/systems.test.tsx`
- `apps/web/src/styles.css`
- `docs/progress/frontend-system-integration.md`

## 契约决策

- `BusinessSystem` 严格采用 OpenAPI 字段：`id/code/name/status/myRole/createdAt/updatedAt`，仅 `description` 可选。
- 列表路由使用不可变 `id`，展示使用 `code`；不再沿用旧骨架的 `slug`。
- `myRole` 仅接受契约枚举，并在 UI 映射中文角色名称。
- transport 签名保持与 `fetch` 一致，默认浏览器 fetch，测试显式注入 mock。
- `GET /systems` 的返回数据即当前用户授权范围，前端不做二次权限筛选。
- `GET /systems/{systemId}` 的 404 同时代表不存在或越权；前端统一显示“工作空间不可用”，避免泄露资源存在性。

## 已知限制

- 当前没有运行时 OpenAPI schema 校验，依赖后端遵守 Envelope 与字段契约。
- 错误态暂未提供重试按钮；重新进入路由会重新请求。
- 系统内各功能页的 eyebrow 仍是骨架期“订单中心”静态文本，后续应由系统详情上下文统一提供。
- 新建业务系统按钮尚未接入 `POST /systems`，不在本竖切范围。

## 下一接力点

- 抽取 System Workspace Context，让系统内页面复用当前系统详情，移除静态“订单中心”文案，并避免多个子页面重复请求。
- 为列表与详情错误态增加可访问的重试操作；需要时再接入创建系统流程。

## 完成信息

- 状态：completed
- 完成时间：2026-07-11
- 修改边界检查：仅修改 `apps/web/**` 与本进度文件；未改动后端、数据库或契约文件。
