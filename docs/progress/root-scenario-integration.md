# Session：root-scenario-integration

## 基本信息

- 状态：completed
- 范围：第五轮场景发现、执行与共享 Server/MySQL 集成

## Red

- `httpapi` 集成测试首先因 `NewWithDependencies` / `Dependencies` 不存在而编译失败，证明 discovery/execution/runrecord 尚未注册共享 Server。
- `cmd/server` 测试随后因 workflow dependencies 仍只包含 scanner/importer 而编译失败，证明新模块尚未使用真实 MySQL。
- 真实 E2E 首次创建 code discovery 返回 400；定位为 API Operation 输入缺少明确 `systemId`，测试补齐 system scope，未放宽服务校验。

## Green

- 新增共享 `httpapi.Dependencies`，统一装配 System、Scanner、Importer、Discovery、Execution 与 RunRecord。
- MySQL workflow pool 同时提供 Scanner、Importer、Discovery、Execution Repository 和 MySQLStepProvider。
- 未注入 connector executor 时使用 DisabledExecutor：不访问网络、不制造成功，持久化失败 attempt 后返回稳定 503。
- 真实 E2E 覆盖代码-only P0 候选、Viewer 核验 403、Owner accept、执行器未配置 503、failed run/attempt 查询。

## 下一接力点

- 为 accepted candidate 增加 promote/publish，生成 scenarios/version/steps。
- 实现受环境和 host allowlist 约束的 connector-aware HTTP executor。
- React 接入发现、核验、执行到指定步骤、单步 retry 和运行详情。
