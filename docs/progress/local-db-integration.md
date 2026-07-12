# Session: local-db-integration

## 基本信息

- 状态：completed
- 负责 Session：local-db-integration
- 开始时间：2026-07-11
- 修改范围：`db/**`、`deploy/dev/**`、本文件
- 测试目标：本地数据库服务（凭据仅由进程环境提供）

## 目标

- 扩展集成执行器，支持通过 `MYSQL_HOST`、`MYSQL_PORT`、`MYSQL_USER`、`MYSQL_PASSWORD` 连接现有服务。
- 在独立临时数据库 `bizdevops_contract_test` 验证 up、seed、系统隔离、角色枚举和 down，并在结束后清理数据库。
- 保持 migration 对 MySQL 8.0 有效；本机 MariaDB 只作为额外兼容性验证目标。

## 进度登记 1：环境直连与密钥安全契约（Red）

- 状态：completed
- 测试：新增 `test_runner_supports_secret_safe_environment_connection`，要求四个连接环境变量、固定独立临时库、无脚本固定密码、通过子进程环境传递密码。
- 命令：`python -m unittest db.tests.test_system_isolation_contract.SystemIsolationContractTest.test_runner_supports_secret_safe_environment_connection -v`
- 结果：Red；执行器缺少 `MYSQL_HOST` 支持，测试按预期失败，证明测试先于实现。
- 安全：失败输出及本登记均不包含本地数据库密码。
- 下一接力点：实现本地客户端连接适配器与临时数据库生命周期，再运行该契约到 Green。

## 改动文件

- `db/tests/test_system_isolation_contract.py`
- `db/tests/run_mysql_integration.py`
- `deploy/dev/README.md`
- `docs/progress/local-db-integration.md`

## 进度登记 2：环境直连与临时库生命周期（Green）

- 状态：completed
- 最小实现：新增 `--environment` 模式；四项连接参数只读环境变量，密码仅经子进程环境传给客户端；固定使用 `bizdevops_contract_test`，运行前重建、运行后在 `finally` 中删除。
- Docker 回归：容器测试密码改为运行时随机生成，不再保留脚本默认密码，也不再使用命令行 `-p...` 参数。
- 命令：`python -m unittest db.tests.test_system_isolation_contract.SystemIsolationContractTest.test_runner_supports_secret_safe_environment_connection -v`
- 结果：Green；`Ran 1 test ... OK`。
- 下一接力点：对现有本地服务执行真实 up/seed/scope/enum/down，预期首次运行可暴露 MariaDB 与 MySQL 8.0 的语法差异。

## 进度登记 3：真实服务首次运行（Red）

- 状态：completed
- 真实结果：up migration 与开发 seed 均成功；首次 scoped assertion 失败，因为 MariaDB 客户端把安全警告写到 stderr，而通用子进程封装把 stderr 合并进 stdout，污染了查询值。
- 清理验证：失败路径的 `finally` 已删除 `bizdevops_contract_test`，`information_schema` 查询返回 0。
- 兼容结论：这不是 migration 的 MySQL/MariaDB 语法差异，而是客户端输出通道差异；无需修改生产 schema。
- 后续 Red：新增 `test_runner_keeps_client_warnings_out_of_query_results`，要求 stdout/stderr 分离；现有实现仍为 `stderr=subprocess.STDOUT`，测试按预期失败。
- 下一接力点：最小修改通用执行器为独立捕获 stderr，再重跑真实完整契约。

## 进度登记 4：真实服务完整契约（Green）

- 状态：completed
- 最小实现：子进程 stdout/stderr 分离，MariaDB 客户端警告不再污染 SQL 查询结果。
- 契约结果：本地 MariaDB 12.2.2 上 up、seed、Alice/Bob/outsider 列表隔离、允许/拒绝详情读取、五角色枚举、down 全部通过。
- 清理结果：完整执行结束后再次查询 `information_schema`，`bizdevops_contract_test` 数量为 0。
- MySQL 兼容性：未为 MariaDB 修改任何 migration SQL；原 MySQL 8.0 schema 与静态契约保持不变。
- 后续 Red：新增本地服务运行说明契约，要求文档列出 `--environment` 和四项环境变量但不出现密码值；测试因 README 尚无该说明而按预期失败。
- 下一接力点：补无密文使用说明到 Green，再执行全量回归。

## 进度登记 5：文档与全量回归（Green）

- 状态：completed
- 文档实现：README 增加交互读取密码的环境直连示例，不包含任何密码值，并明确禁止把真实密码写入仓库或命令行参数。
- 文档测试：`test_local_server_runner_is_documented_without_a_password_value` 从 Red 转为 Green。
- 全量测试：`python -m unittest discover -s db/tests -p "test*.py" -v`，8/8 通过。
- 结构验证：`python db/tests/validate_migration.py`，18 张表、17 个系统隔离复合外键通过。
- 语法验证：执行器和契约测试 `py_compile` 通过。
- 最终真实验证：本地 MariaDB 12.2.2 完整契约通过，临时数据库已清理；全程未将本地密码写入仓库、脚本默认值或进度输出。

## 下一接力点

- 后续 Session 可在 MySQL 8.0 CI/Compose 上继续使用 `--require-docker`，或在已有服务上通过 `--environment` 复用同一套契约。
