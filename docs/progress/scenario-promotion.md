# Session：scenario-promotion

## 基本信息

- 状态：completed
- 负责人/Session：scanner-persistence-api
- 开始时间：2026-07-12
- 修改范围：`apps/api/internal/modules/discovery/**`、`apps/api/internal/modules/scenario/**`、`docs/progress/scenario-promotion.md`

## Red

- 先新增 Memory/Service 测试：仅 accepted 可发布、跨系统不可见、重复发布幂等、步骤顺序/依赖、详情 bundle 保真。
- 先新增 sqlmock：候选 `FOR UPDATE`、scenario/version/steps/current version/candidate promotion 同一事务，所有查询显式 system scope。
- 先新增 HTTP：owner/maintainer 发布、成员读取、非成员 404、viewer 写入 403、严格 UUID/JSON。
- `go test ./internal/modules/scenario` 编译失败，明确缺少 PromotionCandidate、Repository、Service、Memory/MySQL 和 HTTP 路由，确认先 Red。

## Green

- 实现正式场景领域：Scenario、Version、Step、Detail、PromotionResult；发布后的场景初始为 draft，版本 source 为 generated、versionNo=1。
- 门禁：候选必须 accepted 且 URL 中 system/discovery/candidate scope 全匹配；pending/rejected 返回 not accepted；非本系统表现为 not found。
- 幂等：候选已有 `promoted_scenario_id` 时读取并返回原 scenario/version/steps，`created=false`，不会创建第二份场景。
- bundle/步骤保真：candidate priority、sourceRefs、requiresReview 和原始步骤写入 `bundle_document`；步骤按原顺序 1-based 落库，后一节点依赖前一节点，保留 API operation ref 和 method/path request config。
- MySQL 原子事务：candidate `FOR UPDATE` → scenario → version → ordered steps → scenario.current_version_id → candidate.promoted_scenario_id → commit；任一步失败 rollback。所有 SQL 查询/更新显式携带 system_id。
- MemoryRepository 提供同语义实现，覆盖跨系统隔离和重复调用。
- HTTP：`POST /systems/{systemId}/discoveries/{discoveryId}/candidates/{candidateId}/promote`，以及 scenario 列表/详情；owner/maintainer 发布、成员读取、非成员 404；严格 UUID、空 JSON 对象、1MiB 和统一响应。
- 验证：`go test ./internal/modules/scenario`、`go vet ./internal/modules/scenario`、`go test ./...`、`go vet ./...` 全部通过。

## 改动文件

- `apps/api/internal/modules/scenario/domain.go`
- `apps/api/internal/modules/scenario/repository.go`
- `apps/api/internal/modules/scenario/memory_repository.go`
- `apps/api/internal/modules/scenario/mysql_repository.go`
- `apps/api/internal/modules/scenario/http.go`
- `apps/api/internal/modules/scenario/promotion_test.go`
- `apps/api/internal/modules/scenario/mysql_repository_test.go`
- `apps/api/internal/modules/scenario/http_test.go`
- `docs/progress/scenario-promotion.md`

## 契约决策

- `accepted` 是正式发布的硬门禁；promotion 不接受 pending/rejected，也不会隐式修改 review 状态。
- scenario 使用 candidate key 作为 `scenario_key`，正式状态先落 draft，后续人工确认/启用流程再转 active。
- candidate 行锁和 `promoted_scenario_id IS NULL` compare-and-set 共同保证同一候选并发发布只有一个事务成功；后续请求走幂等读取。
- 先插入 scenario（current version 为空），再插入 version/steps，最后更新 current_version，满足现有循环外键结构。
- HTTP 只依赖窄接口 `ScenarioHTTPService` 和 RoleReader，由 root 选择 MySQL、身份中间件和注册顺序。

## 已知限制

- 不同候选若生成相同 `scenario_key`，会触发数据库 system/key 唯一约束；后续需定义合并、版本升级或显式改名策略。
- 当前发布只创建首版，尚未支持 accepted candidate 合并到已有 scenario 的新版本。
- MemoryRepository 接收独立 PromotionCandidate 快照，不与 discovery MemoryRepository 共享原子状态；生产 MySQLRepository 直接锁定真实 candidate 表，具备完整事务语义。
- 当前依赖按线性前一步生成；复杂 DAG 应由人工编辑或后续编排规则提供明确 dependsOn。
- 本轮不注册 shared server，也未增加真实 MySQL HTTP E2E。

## 下一接力点

- root 构造 `scenario.NewMySQLRepository(db)`、`scenario.NewService(repo)`，调用 `scenario.RegisterSystemRoutes(mux, service, systemRepo)`。
- 增加真实 MySQL 并发发布 E2E，验证两个请求一个 created、一个幂等返回同一 scenario。
- 前端 candidate accepted 后显示“发布场景”，并跳转正式 scenario 详情/编排页。
- 增加场景新版本、草稿编辑和 active 发布工作流。
