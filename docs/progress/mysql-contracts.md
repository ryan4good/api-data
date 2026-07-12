# Session：mysql-contracts

## 基本信息

- 状态：completed
- 修改范围：`db/**`、`contracts/**`、`docs/technical-contracts.md`

## 完成内容

- MySQL 初始迁移 18 张表。
- 17 个系统隔离复合外键。
- Scenario Bundle Schema、示例与 Postman 映射。
- Discovery 与 Candidate 持久化。

## TDD 证据

- Red：复合外键 `SET NULL` 错误、Schema 缺失、角色不一致、Discovery 表缺失。
- Green：数据库结构验证和 Scenario Schema 验证通过。

## 下一接力点

- 在真实 MySQL 8.0 中执行 up/down migration，并验证用户无法跨系统读取资源。

