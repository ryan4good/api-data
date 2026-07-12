# Session：frontend-management-overview

## 基本信息

- 状态：completed
- 负责人/Session：frontend_system_workspace
- 开始时间：2026-07-12 10:16（Asia/Shanghai）
- 完成时间：2026-07-12 10:20（Asia/Shanghai）
- 修改范围：`apps/web/src/**`、`docs/progress/frontend-management-overview.md`

## 目标

- 以服务端授权聚合替换 Dashboard 静态统计，展示授权系统数、API 资产、P0 候选、场景、近 24h 运行和风险提示。
- 支持后端返回系统列表内的筛选和 workspace drill-down，不在前端抓取/汇总其他模块数据。
- 将单系统管理摘要接入现有系统 workspace，并隔离其加载失败。

## Red

- 测试：ApiClient 固定两个 management GET；新增管理视图测试覆盖 loading/error/empty、授权范围、后端 aggregate、筛选/drill-down、partial 指标和单系统摘要。
- 失败原因：`getManagementOverview` / `getManagementSystemOverview` 不存在；`management` 页面模块不存在。
- 命令与摘要：`npm test -- --run src/api/client.test.ts src/pages/management-overview.test.tsx`，客户端新增用例失败且页面 suite 因模块缺失失败。

## Green

- 最小实现：新增 Management DTO/client；Dashboard 路由切换到真实 management projection；筛选只过滤 response.systems；总指标始终直接显示 response aggregate；单系统摘要作为独立组件嵌入 workspace；所有计数字段可选并支持 partial。
- 命令与摘要：针对性测试 3 files / 19 tests；全量测试 10 files / 44 tests；最终 `npm run build`（TypeScript + Vite）成功。

## 改动文件

- `apps/web/src/api/types.ts`
- `apps/web/src/api/client.ts`
- `apps/web/src/api/client.test.ts`
- `apps/web/src/pages/management.tsx`
- `apps/web/src/pages/management-overview.test.tsx`
- `apps/web/src/pages/system.tsx`
- `apps/web/src/routes.tsx`
- `apps/web/src/styles.css`
- `docs/progress/frontend-management-overview.md`

## 契约决策

- 固定端点：`GET /api/v1/management/overview`、`GET /api/v1/management/systems/{systemId}/overview`。
- 全局 DTO：`accessScope`、四个可选 aggregate count、`runs24h`、`risks`、`systems`、`partial/unavailableMetrics/generatedAt`。
- 系统 DTO：`systemId/code/name/myRole`、三个可选 count、`runs24h/riskCount/risks`、partial 信息。
- `accessScope=platform|platform_admin` 显示“平台管理员全局范围”；`authorized` 显示“仅授权系统”。前端不根据本地角色扩大范围，完全信任后端已裁剪 response。
- 数字缺失显示 `—`，绝不转换为 0；partial 显示明确警告。
- 筛选是对 response.systems 的展示过滤，不改变服务端 aggregate 卡片，也不请求/拼接未经授权的系统。

## 已知限制

- 两个 management endpoint 当前是待实现契约，后端 DTO 字段需按本记录对齐或在联调时增加显式 adapter。
- 系统筛选当前为前端单选，仅适用于已返回列表；大量系统后应由 management endpoint 支持 query/cursor，仍由后端执行授权过滤。
- 风险仅展示后端返回文本和等级，没有确认/指派/跳转风险实体的写契约。
- 单系统 management 请求失败时独立显示错误并保留原 workspace；目前没有重试按钮和 stale/generatedAt 提示。
- `platform_admin` 能力只由后端 `accessScope` 表达，前端不会自行判断或尝试全量查询。

## 下一接力点

- 后端按此 DTO 落地读模型与 HTTP 契约测试，重点验证普通成员只能看到 membership 范围、platform_admin 才能获得全局 projection。
- 前后端集成测试使用两个权限不同的用户，验证系统数、rows 和 aggregate 同时隔离，避免只裁列表未裁总数。
- 增加生成时间、stale 标记、分页筛选和风险详情链接；所有扩展继续由 management 读模型提供。
