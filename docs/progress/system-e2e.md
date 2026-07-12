# Session：System API MySQL E2E

## 基本信息

- 状态：completed
- 负责人/Session：system_e2e
- 开始时间：2026-07-11
- 完成时间：2026-07-11
- 修改范围：`tests/e2e/**`、`docs/progress/system-e2e.md`

## 目标

- 针对独立 `bizdevops_e2e` 数据库执行 migration/seed 并启动真实 Go API。
- 通过 `X-Dev-User-ID` 验证系统隔离、越权隐藏和 owner 成员管理。
- 测试无论成功或失败都停止 API 并删除临时数据库。

## Red

- 测试：`python -m unittest tests.e2e.test_system_api_mysql -v`
- 预期失败原因：Go HTTP Server 当前仍注册内存 Repository，尚未读取 migration/seed 写入的 MySQL 数据。
- 命令与摘要：2026-07-11 已执行真实本地 MySQL E2E。Alice/Bob 授权列表均错误返回空数组，Alice owner 更新错误返回 403；证明 HTTP Server 仍使用空内存 Repository。失败后 API 已停止且 `bizdevops_e2e` 已删除。
- 第二次 Red：MySQL Repository 接入后所有隔离读取通过，owner 更新返回 400。定位为 development seed 的 version-0 UUID 不满足写接口的 UUID 1–5 校验；测试临时库追加合法 v4 用户 fixture，不修改共享 seed/契约。
- 第三次 Red：合法 v4 fixture 写入已返回 200，但响应 `displayName` 回显 `userId`。后端 Session 将 upsert 调整为返回数据库回读的完整 Member；E2E 同时验证 POST 响应和后续 GET 的真实 display name。

## Green

- 最小实现：harness 使用独立数据库执行 migration/seed，追加合法 v4 成员 fixture，临时编译/启动 API，并在 `finally` 路径停止进程和删除数据库；后端共享 Session 已把 Server 装配到 MySQL Repository，并让 upsert 返回回读后的完整成员。
- 命令与摘要：`python -m unittest tests.e2e.test_system_api_mysql -v`：1/1 通过（真实 `127.0.0.1:3306`）。覆盖 Alice→仅 OMS owner、Bob→仅 WMS maintainer、outsider→空列表/越权详情 404、maintainer 更新 403、owner 更新 200 且 GET 验证持久化。
- 回归：Go `test ./...` 全部通过。
- 清理证据：测试完成后查询 `information_schema.SCHEMATA`，`bizdevops_e2e` 计数为 0。

## 改动文件

- `tests/e2e/test_system_api_mysql.py`
- `tests/e2e/README.md`
- `tests/__init__.py`
- `tests/e2e/__init__.py`
- `docs/progress/system-e2e.md`

## 契约决策

- 数据库密码只读取 `MYSQL_PASSWORD`，通过子进程环境 `MYSQL_PWD`/`MYSQL_DSN` 传递，永不写文件或进入命令行参数。
- 固定种子 ID 来断言 Alice→OMS owner、Bob→WMS maintainer、outsider 无权限。
- outsider 越权详情必须返回 404；Bob 作为 maintainer 更新成员必须返回 403；Alice owner 更新后再次查询验证持久化。

## 已知限制

- harness 独占固定数据库名 `bizdevops_e2e`，不支持同一 MySQL 实例上的并发 E2E。
- 本地 MariaDB/MySQL 命令行客户端可用；测试已能连接用户指定的 `127.0.0.1:3306`。
- development seed 的固定用户 ID 使用 UUID version 0，无法作为严格 UUID 写接口的 payload；harness 因此只在临时库追加 v4 fixture，不改共享 seed。

## 下一接力点

- CI/后续 Session 可按 `tests/e2e/README.md` 注入 `MYSQL_PASSWORD` 后直接执行；同一数据库实例上须串行运行。
- 若未来允许并行 E2E，应把数据库名参数化并限制为安全前缀后再实施清理。
