# Postman Collection 2.1 导入映射

目标格式为 `Scenario Bundle 1.0`。导入分为解析、转换、绑定、核验四步；原始 JSON 和转换报告存入 `scenario_imports`，不静默丢弃任何无法转换的内容。

## 结构映射

| Postman 2.1 | Scenario Bundle | 规则 |
|---|---|---|
| `info.name` | `scenario.name` | `scenario.key` 由名称生成，可在导入前修改 |
| Collection 根或选定 Folder | 一个 Scenario | 嵌套 Folder 按深度优先顺序展开；原目录路径写入 step extension |
| Request item | HTTP step | `item.id` 优先作为稳定 `step.key`，否则由目录路径和名称生成 |
| Collection/Folder/Request 顺序 | `steps[]` 顺序 | 默认把前一步写入后一步的 `dependsOn`；用户可在核验页改为 DAG |
| `request.method` / `url` | `request.method` / `request.url` | Path、query 和 disabled query 参数分别保留 |
| Header | `request.headers` | disabled header 写入 extension，不参与执行 |
| Body raw JSON | `request.body` | 可解析 JSON 时存对象；否则保留字符串和语言信息 |
| Body form-data/urlencoded | `request.body` | 使用带类型的 extension 保留 key、value、disabled、file source |
| Collection/Environment variable | `scenario.variables` | 环境变量优先成为 `environment` scope；同名按 request → folder → collection → environment 覆盖 |
| Auth | `request.authRef` | 不导入 token/password 明文；生成待绑定 secret reference |
| Example response | step extension | 只作为样例，不自动成为断言 |

## 脚本、提取和断言

- `prerequest` JavaScript 若仅包含变量赋值、时间戳或 UUID 等白名单表达式，可转换为平台变量表达式；否则生成紧邻请求前的 `script` step，保留源码并设置 `requiresReview: true`。
- `test` 中可静态识别的 `pm.response.to.have.status`、`pm.expect(...).to.eql`、header 检查和 JSON path 检查转换为 `assertions`。
- `pm.environment.set`、`pm.collectionVariables.set` 且值来自 response JSON/header 时转换为 `extractors`。
- 动态 `eval`、外部 package、`pm.sendRequest`、循环、任意网络或文件能力不自动执行。源码写入 extension，转换报告标记 `manual_action`。
- Postman sandbox 与平台脚本运行时语义不同，因此任何保留脚本在首次运行前必须人工核验。

## API 与环境绑定

请求按规范化后的 `METHOD + path template` 匹配同一 `system_id` 下的 `api_operations.operation_key`。唯一匹配时写入 `apiOperationRef`；零匹配或多匹配时保留请求但产生阻断项。`{{baseUrl}}` 映射到 HTTP connector，Postman Environment 映射到平台 Environment；敏感变量只能映射为 `secretRef`。

## 转换报告

`conversion_report` 至少包含：

```json
{
  "counts": { "requests": 8, "assertions": 5, "scripts": 2 },
  "bindings": { "apiMatched": 7, "apiUnresolved": 1, "environmentUnresolved": 1 },
  "issues": [
    {
      "severity": "blocking",
      "code": "API_OPERATION_UNRESOLVED",
      "sourcePath": "item[1].item[3]",
      "message": "未找到 POST /api/orders/{id}/approve"
    }
  ]
}
```

严重度为 `info`、`warning` 或 `blocking`。存在 blocking 项、明文密钥、Schema 不合法或未核验脚本时，不允许发布；可保存为导入草稿。

## 幂等与冲突

- 文件 SHA-256 作为 `content_hash`；同一系统重复上传同一内容返回已有导入记录。
- `scenario.key` 不存在时创建场景及版本 1；已存在时创建下一版本，不覆盖历史版本。
- 导入完成后原始文件不可变；人工编辑产生新的 `scenario_versions`。
