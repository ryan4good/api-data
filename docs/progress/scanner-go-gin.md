# Session：scanner-go-gin

## 基本信息

- 状态：completed
- 负责人/Session：scanner-go-gin
- 开始时间：2026-07-11
- 完成时间：2026-07-11
- 修改范围：`apps/api/internal/modules/scanner/**`、`apps/api/cmd/scanner/**`

## 目标

- 使用 Go 标准库 AST 扫描 Gin 风格 API 路由，输出稳定、可序列化的 API operation。
- 明确定义扫描任务状态和合法转换，并提供 JSON CLI 入口。

## Red

- 测试：模块测试覆盖五种 HTTP 方法、大小写误报、递归目录和噪声排除、状态转换；CLI 测试覆盖 JSON 输出和参数校验。
- 失败原因：当前仅有 HTTP 占位注册器，不存在 `Analyze`、operation 模型、状态机和扫描 CLI。
- 命令与摘要：`go test ./internal/modules/scanner ./cmd/scanner` 编译失败，明确报告 `Analyze`、`Operation`、`StatusSucceeded`、`run` 等符号不存在，确认测试先于实现进入 Red。随后针对 `r.Get` 误报新增回归测试，测试先观察到错误的额外 GET operation，再收紧为 Gin 的大写方法名。

## Green

- 最小实现：基于 `go/parser`/`go/ast` 遍历仓库，识别 `GET/POST/PUT/PATCH/DELETE` 选择器调用；抽取字面量路径、最后一个 handler 参数及相对文件/行号；增加状态机和单参数 JSON CLI。
- 命令与摘要：
  - `go test ./internal/modules/scanner ./cmd/scanner`：通过（两个 package）。
  - `go run ./cmd/scanner ../../sample-repos/mock-oms-service-go`：通过，稳定输出 3 个真实样例接口及来源行。
  - `go test ./...` / `go vet ./...`：执行时被并行 importer session 的 Red 测试阻断；错误均为 `internal/modules/importer` 尚未实现的符号，scanner package 显示通过。

## 改动文件

- `apps/api/internal/modules/scanner/analyzer.go`
- `apps/api/internal/modules/scanner/status.go`
- `apps/api/internal/modules/scanner/scanner_test.go`
- `apps/api/internal/modules/scanner/module.go`（保留原有 HTTP 占位注册器，未修改共享路由）
- `apps/api/cmd/scanner/main.go`
- `apps/api/cmd/scanner/main_test.go`
- `docs/progress/scanner-go-gin.md`

## 契约决策

- operation JSON 字段固定为 `method`、`path`、`handler`、`source.file`、`source.line`。
- 扫描结果的 source file 相对扫描根目录并统一为 `/` 分隔，结果按文件和行号排序，便于快照比较。
- 支持状态为 `queued -> running -> succeeded|failed`；不允许跳级、自循环或终态回退。
- 多个 Gin handler 参数时，以最后一个参数作为最终业务 handler。
- 仅接受 Gin 风格大写方法选择器，避免把常见的 `client.Get(...)` 当作路由。

## 已知限制

- 当前是无类型信息的语法扫描：任何接收者的同名方法都可能被视作路由；后续可引入类型分析降低误报。
- 仅解析静态字符串路径，暂不拼接 `Group()` 前缀、常量路径或字符串表达式。
- 排除 `vendor`、`node_modules`、`.git` 和 `*_test.go`；单个 Go 文件语法错误会使本次扫描失败。
- 本 session 不接入 HTTP API、不持久化扫描任务，这是后续模块职责。

## 下一接力点

- 将 `Analyze` 接入 scanner 应用服务：创建 job、记录状态转换与错误，再持久化 operation。
- 增加 Gin `Group` 路径传播、`Handle(method, path, ...)` 和跨函数注册分析。
- importer session Green 后重新执行一次全量 `go test ./...` 与 `go vet ./...`，消除共享工作区 Red 阶段干扰。
