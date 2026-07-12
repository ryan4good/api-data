# Session: mysql-system-isolation

## 基本信息

- 状态：completed
- 负责 Session：mysql-system-isolation
- 开始时间：2026-07-11
- 完成时间：2026-07-11
- 修改范围：`db/**`、`deploy/dev/**`、本文件

## 目标

- 提供 MySQL 8.0 本地 Compose、最小开发种子、应用级系统隔离查询契约，并验证 up/down migration。

## 进度登记 1：隔离契约测试（Red）

- 状态：completed
- 测试：新增 `db/tests/test_system_isolation_contract.py`，约束角色枚举、OMS/WMS 种子、成员过滤查询、Compose 和集成测试执行器。
- 命令：`python -m unittest db.tests.test_system_isolation_contract -v`
- 结果：Red；5 项中 1 项通过、4 项因实现文件尚不存在而报错。这是预期失败，证明测试先于实现。
- 限制：静态契约只能证明 SQL 形状，真实权限结果仍需 MySQL 8.0 集成测试。
- 下一接力点：补 scoped query 与开发种子，先让隔离和种子契约 Green。

## 进度登记 2：开发种子与 scoped query（Green）

- 状态：completed
- 最小实现：提供幂等开发种子（Alice/OMS owner、Bob/WMS maintainer、OMS viewer、无成员关系 outsider）和列表/详情两条授权查询。
- 命令：`python -m unittest db.tests.test_system_isolation_contract.SystemIsolationContractTest.test_seed_has_oms_wms_users_and_distinct_memberships db.tests.test_system_isolation_contract.SystemIsolationContractTest.test_application_system_queries_always_join_membership -v`
- 结果：Green；2 项测试全部通过（`Ran 2 tests ... OK`）。
- 限制：`?` 参数中的 `user_id` 必须来自已认证上下文，不能信任请求正文中的用户 ID。
- 下一接力点：补 MySQL Compose 与真实 up/seed/scope/down 执行器。

## 进度登记 3：MySQL 8.0 Compose 与 CI 执行器（Green）

- 状态：completed
- 最小实现：固定 `mysql:8.0.36`，提供健康检查、持久卷、首次启动 migration/seed；无第三方 Python 依赖的执行器依次验证 up、seed、授权列表/详情、角色枚举和 down。
- 命令：`python db/tests/run_mysql_integration.py --static-only`
- 结果：Green；`--static-only` 返回 `PASS: static migration and isolation contracts`；`docker compose ... config` 成功解析服务、健康检查、端口和两个初始化挂载。
- 限制：Compose 的 init 脚本仅在空数据卷运行；变更 migration 后需 `down -v` 重建本地卷。
- 下一接力点：运行全部静态测试，探测 Docker 并登记最终结果。

## 改动文件

- `db/tests/test_system_isolation_contract.py`
- `db/seeds/000001_development.sql`
- `db/queries/business_systems.scoped.sql`
- `db/tests/run_mysql_integration.py`
- `deploy/dev/compose.yaml`
- `deploy/dev/.env.example`
- `deploy/dev/README.md`
- `docs/progress/mysql-system-isolation.md`

## 契约决策

- 应用读取业务系统时必须以 `user_id` 加入 `system_members`，读取单系统时还必须同时绑定 `system_id`。

## 已知限制

- 数据库账号仍可直接读基础表；隔离契约面向应用 Repository 查询模式，后续可按部署需求增加数据库级权限。
- 当前主机 Docker CLI 可用，但 Docker Desktop daemon 未启动（named pipe 不存在），因此本 Session 未执行真实容器测试；CI 应使用 `--require-docker` 强制执行，daemon 缺失会失败而不是静默跳过。

## 下一接力点

- 后端 Repository 直接复用 `db/queries/business_systems.scoped.sql` 的参数顺序：认证用户 ID 在前，详情查询的系统 ID 在后。

## 最终验证

- 状态：completed
- `python -m unittest discover -s db/tests -p "test*.py" -v`：5/5 通过。
- `python db/tests/validate_migration.py`：`PASS: 18 tables; 17 scoped foreign keys`。
- `python db/tests/run_mysql_integration.py --static-only`：通过。
- `docker compose --env-file deploy/dev/.env.example -f deploy/dev/compose.yaml config`：通过。
- 真实 MySQL 命令（Docker daemon/CI 可用时）：`python db/tests/run_mysql_integration.py --require-docker`。
