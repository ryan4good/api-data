# Session：environment-secret-resolver

## 基本信息

- 状态：completed
- 负责人/Session：scanner-persistence-api
- 开始时间：2026-07-12
- 修改范围：`apps/api/internal/modules/environment/**`、`apps/api/internal/modules/connector/**`、`docs/progress/environment-secret-resolver.md`

## Red

- 先新增 Resolver 测试：普通变量+secret 合并、Provider 轮换即时生效、缺失 secret 错误脱敏、disabled/cross-system 拒绝、connector host policy 只能收紧全局 allowlist。
- 先新增 sqlmock：环境/secret reference 查询携带 system scope，写入参数只有 reference/metadata。
- 先新增 HTTP：owner 写、runner 只读、viewer secret reference 脱敏、非成员 404、严格 JSON。
- `go test ./internal/modules/environment` 编译失败，明确缺少领域、Repository、Resolver、MySQL 和 HTTP 路由，确认先 Red。

## Schema 补齐

- 初始 migration 缺少 `environment_secrets`；根 Session 已新增 `000002_environment_secrets` up/down migration。
- 新表只保存 reference 和审计字段，带 `(system_id, environment_id)` 复合外键及 scoped 唯一键；真实数据库 up/seed/down 与 API E2E 已通过。

## Green

- Environment/SecretReference 领域与 system-scoped Repository，提供 Memory/MySQL 实现；环境支持 active/disabled，普通变量为 JSON map。
- Resolver 每次执行即时调用注入的 SecretProvider，普通变量与密钥值合并；Provider 轮换无需更新数据库即可生效。
- 安全输出：ResolvedEnvironment 的 Variables 标记 `json:"-"`，String/GoString 固定 `[REDACTED]`；Provider 原始错误完全丢弃，只返回 `secret unavailable: variable <key>`，不会包含 provider 消息或 secret value。
- host policy：connector `NarrowHostAllowlist` 只计算 connector 与全局集合交集；connector 空策略继承全局，全局为空时任何 connector 都不能放宽。
- HTTP：环境列表/写入、secret reference 列表/写入；owner/maintainer 写，runner/viewer 只读；viewer 返回 secret ref 空值，runner 可见执行绑定 reference；非成员 404。
- 输入：规范 UUID、严格 JSON、1MiB；敏感变量名禁止写入普通 variables，secretRef 只接受外部 provider URI scheme，不接受疑似明文。
- sqlmock 验证 environment/secret reference 查询显式 systemID，secret 写入参数只有 reference/metadata，没有 value 字段。
- `go test ./internal/modules/environment ./internal/modules/connector` 和对应 vet 通过。
- 并行 management session 完成 Green 后复验 `go test ./...`、`go vet ./...`，全量通过。

## 改动文件

- `apps/api/internal/modules/environment/domain.go`
- `apps/api/internal/modules/environment/repository.go`
- `apps/api/internal/modules/environment/memory_repository.go`
- `apps/api/internal/modules/environment/mysql_repository.go`
- `apps/api/internal/modules/environment/resolver.go`
- `apps/api/internal/modules/environment/http.go`
- `apps/api/internal/modules/environment/resolver_test.go`
- `apps/api/internal/modules/environment/mysql_repository_test.go`
- `apps/api/internal/modules/environment/http_test.go`
- `apps/api/internal/modules/connector/policy.go`
- `apps/api/internal/modules/connector/policy_test.go`
- `docs/progress/environment-secret-resolver.md`

## 契约决策

- 数据库永不出现 secret value；SecretReference 只有 provider URI 和审计元数据，解析值只存在本次执行内存。
- Resolver 不缓存 provider value，确保轮换立即可见，也减少敏感值驻留时间。
- disabled environment 在任何 secret provider 调用前拒绝；cross-system environment 表现为 not found。
- viewer 允许查看环境和“存在绑定”信息，但 secretRef 字段省略；runner 只读可见 provider reference，供执行绑定排查。
- 全局 host allowlist 是不可越过的上限，环境/connector 只能交集收紧。

## 已知限制

- `environment_secrets` migration 与真实 MySQL E2E 已补齐；E2E 验证 Owner 写 reference、Viewer 隐藏 reference、Runner 只读可见。
- HTTP 对敏感普通变量采用 key 名启发式拦截，不能识别被伪装在普通名称中的凭据；组织侧仍需审计和 secret scanning。
- 本轮不实现具体 Vault/AWS/GCP provider，只定义可注入接口并使用 fake provider 验证。
- 根 Session 已注册 shared server 并接入 MySQL Repository。

## 下一接力点

- 实现真实 Vault/AWS/GCP SecretProvider，并由 Worker 注入 Resolver。
- 执行引擎只接收 ResolvedEnvironment，不持久化合并后的变量；request/response snapshot redaction 复用敏感 key 集合。
- 实现 provider adapter、超时/重试/熔断与审计事件，但审计内容仍不得包含 value。
