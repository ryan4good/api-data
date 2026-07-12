# Session：connector-http-executor

## 基本信息

- 状态：completed
- 负责人/Session：`scenario-import-review`
- 开始时间：2026-07-12
- 完成时间：2026-07-12
- 修改范围：`apps/api/internal/modules/connector/**`、`apps/api/internal/modules/execution/**`、本进度文件

## 目标

- 实现可由根 Session 显式注入的受控 HTTP `execution.StepExecutor`，在支持业务接口执行的同时，将 SSRF、重定向、超时、体积和敏感数据风险收敛在 connector 边界。

## Red

- 先新增本地 `httptest` 功能测试，覆盖 baseURL/method/path/header/query/body、变量替换、JSON dot-path extractor、status/body assertion 与快照脱敏。
- 首次目标测试按预期编译失败：`HTTPExecutorConfig`、`NewHTTPExecutor` 和安全错误类型尚不存在。
- 增加 SSRF/allowlist/private IP/协议、跨 host redirect、请求响应体限制、timeout、未绑定变量和失败断言测试，再完成最小 Green。
- 将测试切换为既有场景契约的 `expression: $.data.id`、`actual: $.data.state` 后按预期失败，随后补齐 `path/expression/actual` 兼容归一化。

## Green

- 实现 `HTTPExecutor`，编译期确认满足 `execution.StepExecutor`；构造器为 `NewHTTPExecutor(HTTPExecutorConfig)`。
- Step config 支持 `baseURL/method/path/headers/query/body/timeoutMs/extractors/assertions`，所有字符串与嵌套 body 支持 `{{variable}}` 替换，未绑定变量明确失败。
- 仅允许 HTTP/HTTPS；host 必须精确存在于显式 allowlist。
- 默认拒绝 loopback、private、link-local、multicast、unspecified（覆盖 metadata IP）；仅 `AllowPrivate=true` 时允许私网。
- URL 预检查和实际 Dial 均重新解析/校验 IP；默认 Transport 禁用环境代理，降低 DNS rebinding 或代理绕过目标控制的风险。
- 每次 redirect 都重新执行 scheme、host allowlist 与 IP 检查，禁止跳转到非允许 host。
- 请求/响应体分别配置上限，读取响应时使用 limit+1 检测；step timeout 受全局 MaxTimeout 封顶，并覆盖 DNS、连接、请求和响应阶段。
- 仅保存请求/响应摘要；authorization/cookie/password/token/secret 等字段及已知敏感变量值统一替换为 `[REDACTED]`，URL 不保存 query。
- 支持简化 dot path 和数字数组索引；兼容 `path`、`$.expression`、`actual`；支持 required extractor，以及 status/body 的 eq/ne/contains assertion。
- connector 目标测试：5/5 通过；execution 回归 13/13 通过；`go test ./...`、`go vet ./...` 通过。

## 配置契约

- `AllowedHosts []string`：必填、精确 hostname，不包含端口；为空时构造失败。
- `AllowPrivate bool`：默认 false；仅本地开发或明确受控内网 connector 可开启。
- `MaxRequestBodyBytes` / `MaxResponseBodyBytes`：默认各 1 MiB。
- `DefaultTimeout`：默认 30 秒；`MaxTimeout` 默认 60 秒；步骤 timeout 不可突破 MaxTimeout。
- 根 Session 若没有完整安全配置，继续注入 `execution.DisabledExecutor`；不要用空 allowlist 或宽泛 host 规则降级。

## 稳定错误

- `ErrUnsupportedScheme`
- `ErrTargetNotAllowed`
- `ErrPrivateTarget`
- `ErrRequestBodyTooLarge`
- `ErrResponseBodyTooLarge`
- `ErrRequestTimeout`
- `ErrUnboundVariable`
- `ErrInvalidStepConfig`
- `ErrExtractorMissing`

## 改动文件

- `apps/api/internal/modules/connector/http_executor.go`
- `apps/api/internal/modules/connector/http_executor_test.go`
- `docs/progress/connector-http-executor.md`

## 已知限制

- allowlist 当前是 hostname 精确匹配，不支持 wildcard、CIDR、端口级规则或每业务系统独立规则；生产配置应由后续 connector repository 按 system/environment 生成。
- 当前只支持 JSON extractor/assertion 的简化 dot path，不支持完整 JSONPath、脚本断言或 schema assertion。
- 请求 body 为字符串时按原文发送，否则 JSON 编码；尚未支持 multipart、form-urlencoded、流式上传和二进制下载。
- 不自动设置认证或 Content-Type；均应由受控 connector/step config 显式提供，secret 由后续 resolver 注入变量。
- 当前同步执行；大响应虽然限制读取大小，但生产环境仍应配合连接池、并发限制、审计和熔断器。

## 下一接力点

- 根 Session 从可信配置构造 HTTPExecutor 并注入 execution Service；配置缺失时保持 DisabledExecutor 503。
- 后续实现 system-scoped connector repository、环境 baseURL/secret resolver，以及每系统 allowlist/私网策略。
- 增加真实 MySQL + 本地受控服务 E2E，验证 scenario_steps 配置到 HTTP attempt/assertion 持久化的完整链路。
