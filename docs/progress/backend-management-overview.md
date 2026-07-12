# Session：backend-management-overview

## 基本信息

- 状态：completed
- 范围：管理总览聚合 API、成员范围和平台管理员范围

## Red

- 先定义 Memory Repository 和 HTTP 隔离测试；领域、Repository 和路由尚不存在，目标测试应先编译失败。
- 增加 MySQL membership scope/sqlmock 后，测试因 Repository/SQL 契约不存在而编译失败。
- 前后端对齐测试首次失败：后端曾返回 `scope/totals/apiCount`，前端固定契约要求 `accessScope/apiAssetCount` 等顶层聚合字段。

## Green

- Memory/MySQL Repository：普通成员只聚合 system_members 授权系统，platform admin 聚合全部 active 系统。
- 聚合 API 资产、P0 pending、场景、24h succeeded/failed/running 和结构化风险。
- `GET /api/v1/management/overview` 与 system detail；未授权详情 404。
- 契约对齐 React DTO：`authorized/platform_admin`、`apiAssetCount`、`p0CandidateCount`、`runs24h.succeeded`、risks。
- 真实 MySQL E2E 覆盖 Alice 单系统、Outsider 空总览、Admin 两系统和 Alice 越权 WMS 404。
- 已注册 shared Server 并使用共享 MySQL pool。

## 下一接力点

- 增加分页/趋势时间序列、审计事件和指标缓存。
- 为 platform auditor 定义只读全局范围。
