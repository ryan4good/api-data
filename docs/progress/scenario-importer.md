# scenario-importer

## 状态

- Session：`scenario-importer`
- 状态：已完成
- 范围：Scenario Bundle 1.0 / Postman Collection 2.1 JSON 导入与转换报告

## TDD 记录

### Red

- 先新增 `apps/api/internal/modules/importer/importer_test.go`，覆盖：原生 Bundle、嵌套 Postman Folder/Request、变量、脚本人工核验、疑似密钥脱敏、未绑定变量、非法 JSON 与不支持格式。
- 执行 `go test ./internal/modules/importer`，按预期编译失败：`Import`、`Options`、`Issue` 等导入契约尚未实现。
- 增加 Postman status test 转换断言后，测试按预期失败：`Assertions: 0`。
- 增加原生 Bundle 的 `apiOperationRef` / `extractors` 无损导入断言后，测试按预期编译失败，证明类型尚未覆盖契约字段。
- 增加“后续步骤使用前序 extractor 输出”的断言后，测试按预期报 `UNBOUND_VARIABLE`，随后补齐按步骤顺序登记 extractor binding 的逻辑。

### Green

- 实现格式判定、原生 Bundle 严格解析、Postman 2.1 嵌套目录深度优先展开及顺序依赖。
- Collection、Folder、Request 的 prerequest/test 脚本全部保留为 JavaScript step，统一设置 `requiresReview: true` 并生成 warning。
- 静态识别 `pm.response.to.have.status(...)` 并转换为 status assertion；原脚本仍保留，避免丢失语义。
- 导入变量；疑似 token/password/secret 等变量不保留明文，改为待绑定 `secretRef`；扫描 `{{variable}}` 并报告未绑定变量。
- 未绑定变量检测会把前序 HTTP step 的 extractor key 视为后续步骤可用绑定。
- 原生 Bundle 保留 HTTP、script、delay、API operation reference、extractor、assertion 和 extension 等执行字段。
- `go test ./internal/modules/importer`：通过。
- `go test ./...`：通过。
- `go vet ./...`：通过。

## 契约

- 入口：`Import(data []byte, options Options) (Result, error)`。
- 输入格式：`scenario-bundle-1.0`、`postman-collection-2.1`；其他格式返回 `UNSUPPORTED_FORMAT`，非法 JSON 返回 `INVALID_JSON`。
- 输出：标准 `Bundle` + `ImportReport`；报告包含 request/script/assertion/variable counts，以及结构化 warnings/errors。
- 关键 issue code：`SECRET_VALUE_REDACTED`、`SCRIPT_REQUIRES_REVIEW`、`UNBOUND_VARIABLE`、`SENSITIVE_VALUE_REJECTED`、`INVALID_SCENARIO_BUNDLE`。
- Postman 请求按源顺序转为串行依赖，原目录位置写入 step `extensions.postmanSourcePath`。

## 文件

- `apps/api/internal/modules/importer/importer_test.go`
- `apps/api/internal/modules/importer/types.go`
- `apps/api/internal/modules/importer/importer.go`
- `docs/progress/scenario-importer.md`

## 限制与下一接力

- 当前是纯领域转换器，尚未接 HTTP 上传入口、数据库 `scenario_imports` 持久化和发布门禁。
- Postman Environment 尚未作为第二输入合并；当前导入 Collection variables。
- 仅静态转换 status assertion；其余 Postman sandbox test/prerequest 代码原样保留并强制人工核验，不直接执行。
- raw JSON body 会转为对象；其他 Postman body mode、disabled header/query、auth 的无损 extension 映射仍需扩展。
- 原生 Bundle 当前执行结构校验和严格字段解析，但未在 Go 内运行完整 JSON Schema validator；建议 HTTP 层接入契约校验。
- 下一接力：增加导入草稿表与 hash 幂等、环境/secret 绑定页面、人工核验状态和发布门禁，再将 importer 接入 system-scoped API。
