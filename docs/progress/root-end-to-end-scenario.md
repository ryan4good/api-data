# Session：root-end-to-end-scenario

## 基本信息

- 状态：completed
- 范围：第六轮场景发布、HTTP executor 配置、共享装配与真实端到端

## Red

- HTTP API 集成测试先因 Dependencies 缺少 Scenario Repository 而失败。
- Config 测试先因 ConnectorHTTP 配置不存在而编译失败。
- Server 测试先因 workflow dependencies 缺少 Scenario 和 executor selector 而失败。
- 真实 E2E 在新增 API Operation fixtures 后，旧“资产为空”断言先失败，随后改为验证真实 3 条资产。

## Green

- Scenario promotion Repository 已接共享 MySQL 与 HTTP：accepted candidate 原子生成 Scenario/Version/Steps，重复调用幂等。
- Connector HTTP executor 通过环境配置显式启用；无 allowlist 时继续使用 DisabledExecutor。
- 配置支持 allowed hosts、private 开关、请求/响应体上限及 timeout 上限。
- 真实 E2E 使用临时 loopback mock API 和显式 private allow：P0 candidate → accept → promote → scenario → HTTP step → status assertion → passed run/attempt 全链路通过。
- 临时 mock、API 进程和 E2E 数据库均由 harness 清理；未访问公网。
- 浏览器验证 Owner 可见四类发现输入、场景/版本/截止步骤选择器；Viewer 不显示发现表单，页面无 console warning/error；临时浏览器 QA 服务与数据库已清理。

## 下一接力点

- 环境变量/secret resolver 与 connector credentials 管理。
- 将同步执行改为 queued worker，增加取消、超时和幂等 retry。
- 增加分页、审计日志和管理总览统计。
