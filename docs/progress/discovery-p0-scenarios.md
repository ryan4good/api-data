# Session：discovery-p0-scenarios

## 基本信息

- 状态：completed
- 负责人/Session：scanner-persistence-api
- 开始时间：2026-07-12
- 修改范围：`apps/api/internal/modules/discovery/**`、`docs/progress/discovery-p0-scenarios.md`

## Red

- 先新增规则/service 测试：无 PRD 纯代码生成 P0、prompt/mixed 证据、查询+写入+结果查询三步组合、跨系统隔离、人工核验门禁。
- 先新增 sqlmock 测试：discovery+candidates 同事务、失败 rollback、list/review 强制 system scope。
- 先新增 HTTP 测试：成员读、owner/maintainer 发起、reviewer 核验、非成员 404、只读角色 403、严格输入。
- `go test ./internal/modules/discovery` 编译失败，明确缺少领域、Service、Memory/MySQL repository 和 HTTP 路由符号，确认测试先进入 Red。

## Green

- 领域与状态：实现 `code/prd/prompt/mixed`、discovery ready、candidate pending/accepted/rejected、P0/confidence/sourceRefs/requiresReview/steps。
- 确定性 P0 规则：POST/PUT/PATCH/DELETE 作为动作锚点；订单/支付/认证/登录提高置信度；按资源首段匹配前置集合查询和结果详情查询，形成“前置查询 → 写操作 → 结果查询”。
- 无 PRD 路径：code 类型只使用 scanner `APIOperation` 即可生成候选；prompt/PRD 可在没有 operation 时为关键业务或明确写意图生成待编排 intent 候选。
- 人工门禁：候选初始必须 pending 且 requiresReview；只允许一次 accepted/rejected 决策；`AllCandidatesReviewed` 在任一 pending 时返回 false。accepted 仅表示核验通过，本轮不写 scenarios 表。
- system scope：Memory/MySQL 所有 list/review 都显式使用 systemID；MySQL discovery+candidates 同事务写入，失败整批 rollback；candidate bundle 落步骤/优先级/source refs。
- HTTP：提供 root 可注册的 `RegisterSystemRoutes`，覆盖 discovery 创建/列表、candidate 列表、accept/reject；成员读，owner/maintainer 发起，reviewer/owner/maintainer 核验，非成员 404。
- 边界：规范 UUID、严格 JSON、未知字段/多 JSON 值拒绝、1MiB body 限制、统一 httpresponse 和 405。
- 验证：`go test ./internal/modules/discovery`、`go vet ./internal/modules/discovery`、`go test ./...`、`go vet ./...` 全部通过。

## 改动文件

- `apps/api/internal/modules/discovery/domain.go`
- `apps/api/internal/modules/discovery/repository.go`
- `apps/api/internal/modules/discovery/memory_repository.go`
- `apps/api/internal/modules/discovery/mysql_repository.go`
- `apps/api/internal/modules/discovery/service.go`
- `apps/api/internal/modules/discovery/http.go`
- `apps/api/internal/modules/discovery/service_test.go`
- `apps/api/internal/modules/discovery/mysql_repository_test.go`
- `apps/api/internal/modules/discovery/http_test.go`
- `docs/progress/discovery-p0-scenarios.md`

## 契约决策

- discovery 的 `ready` 表示“发现计算完成、候选可供核验”，不是“场景已发布”；场景发布前必须额外检查 `AllCandidatesReviewed`，只提升 accepted 候选。
- 候选业务字段嵌入既有 `candidate_bundle` JSON，不改 migration；evidence 保存 source refs，confidence 使用表原生列。
- operation key 排序后再执行规则，保证同一输入的候选顺序和步骤选择确定；数据库 ID/时间除外。
- 非成员先于资源读取返回 system 404；review 路由先验证 candidate 确实属于 URL 中的 discovery，再执行 system-scoped review。
- MySQL review 使用 `WHERE system_id/id/review_status=pending` compare-and-set，重复人工决策返回 conflict。

## 已知限制

- 当前为透明启发式规则，不是语义模型：资源匹配使用 URL 首段，复杂跨域链路、嵌套路由和补偿流程需后续规则/模型增强。
- HTTP 创建目前接收 operation 快照；生产组合层应优先按 systemID 从 scanner repository 读取资产，避免把客户端上传的 operation 当权威资产。
- PRD/prompt-only intent 候选没有可执行 API operation，必须人工补全步骤后才可发布运行。
- 本轮不创建 scenarios/scenario_versions，也没有发布 endpoint；accepted/rejected 仅完成核验状态。
- discovery 计算为同步；大量资产应改为 queued + worker，并持久化 running/failed 过程。

## 下一接力点

- root 注入 `discovery.NewMySQLRepository(db)`、`discovery.NewService(repo)`、system role reader 并调用 `discovery.RegisterSystemRoutes`。
- 增加 scanner operation provider，由服务端按 system scope 装载资产，而非依赖 HTTP body。
- 实现 accepted candidate 到 scenario/version/steps 的发布事务，并把“全部已决策”作为发布前置条件。
- 加入真实 MySQL HTTP E2E，验证同一 candidate 并发 review 只有一次成功、跨系统全链路不可见。
