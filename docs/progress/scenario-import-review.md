# Session：scenario-import-review

## 基本信息

- 状态：completed
- 负责人/Session：`scenario-import-review`
- 开始时间：2026-07-11
- 完成时间：2026-07-11
- 修改范围：`apps/api/internal/modules/importer/**`、本进度文件

## 目标

- 基于现有 `scenario_imports` 表，为转换器补齐 system-scoped 导入草稿、幂等上传、列表/详情、脚本人工核验和应用门禁。

## Red

- 先新增 Service/MemoryRepository 测试，覆盖同系统 hash 幂等、跨系统隔离、转换失败草稿、脚本核验门禁、列表/详情 scope 与并发唯一键竞争。
- 先新增 MySQL sqlmock 测试，覆盖查询必须携带 `system_id`，以及核验/应用必须 `BEGIN -> SELECT ... FOR UPDATE -> scoped UPDATE -> COMMIT/ROLLBACK`。
- 首次执行 `go test ./internal/modules/importer` 按预期编译失败：`Repository`、`Service`、`StoredConversion`、`MySQLRepository`、状态和门禁错误尚不存在。

## Green

- 实现 `Service`、`Repository`、并发安全 `MemoryRepository` 与 `MySQLRepository`。
- 上传按原始文档 SHA-256 在业务系统内幂等；并发唯一键竞争后会按相同 scope/hash 读取胜者。
- 转换成功进入 `ready`；支持格式但转换有 errors 时保存为 `failed`，错误消息仅使用固定文本，不拼接原始文档或敏感值。
- 转换结果和 `Review` 元数据共同存入 `conversion_report` JSON，不修改现有迁移。
- 脚本存在时必须先确认；应用在事务锁内重验 errors、status 和人工核验状态，再迁移为 `applied`。
- `go test -v ./internal/modules/importer`：13/13 通过。
- `go test ./...`：通过。
- `go vet ./...`：通过。

## 改动文件

- `apps/api/internal/modules/importer/application.go`
- `apps/api/internal/modules/importer/memory_repository.go`
- `apps/api/internal/modules/importer/mysql_repository.go`
- `apps/api/internal/modules/importer/service_test.go`
- `apps/api/internal/modules/importer/mysql_repository_test.go`
- `docs/progress/scenario-import-review.md`

## 契约决策

- 所有 Repository 读写都显式接收 `systemID`；详情不存在和跨系统访问统一返回 `ErrNotFound`。
- `conversion_report` 保存 `{result, review}`；`review.scriptsConfirmed/reviewedBy/reviewedAt` 是当前核验凭证。
- 应用门禁：转换报告有 errors 或状态为 `failed` 返回 `ErrImportHasErrors`；有脚本未确认返回 `ErrScriptsUnreviewed`；非 `ready/applied` 返回 `ErrNotReady`。
- `Apply` 对已 `applied` 记录幂等；首次应用使用 scoped 行锁和条件更新，避免核验与应用并发绕过门禁。
- 原始上传文档仅作为 `raw_document` 参数持久化；不进入服务错误、数据库错误包装或日志。

## 已知限制

- 本 Session 未注册 HTTP 路由；根 Session 可直接注入 Service，或在 importer 包内另加 handler 后接入共享 server。
- `applied` 当前代表导入门禁已通过，尚未在同一事务内创建 `scenarios/scenario_versions/scenario_steps`；后续场景发布服务应消费已核验的 `StoredConversion.Result.Bundle`。
- 人工核验目前是“确认该导入中的全部脚本”，没有逐脚本批注、双人复核或核验撤销。
- 格式无法识别/非法 JSON 不写入表，因为现有 `format` ENUM 无法表示 unknown；已识别 Scenario Bundle 的转换失败会保存为 `failed`。

## 下一接力点

- 根 Session 注册 system-scoped HTTP handler，并在 access 层要求 reviewer/maintainer/owner 才能确认，maintainer/owner 才能应用。
- 增加真实 MySQL 集成测试，验证 `conversion_report` JSON 往返、唯一键并发和事务门禁。
- 将 `Apply` 与场景版本落库服务组合为一个事务，生成 draft scenario/version/steps，并写审计日志。
