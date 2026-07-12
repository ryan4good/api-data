# MySQL 本地开发环境

启动并等待健康检查：

```powershell
docker compose --env-file deploy/dev/.env.example -f deploy/dev/compose.yaml up -d --wait
```

首次创建数据卷时会自动执行初始 migration 和开发种子。迁移或种子变化后，使用新的数据卷重新初始化：

```powershell
docker compose -f deploy/dev/compose.yaml down -v
docker compose --env-file deploy/dev/.env.example -f deploy/dev/compose.yaml up -d --wait
```

开发凭据仅用于本机，不得用于共享或生产环境。完整 migration 隔离测试可执行：

```powershell
python db/tests/run_mysql_integration.py --require-docker
```

也可以复用已经运行的 MySQL/MariaDB 服务。连接信息只通过当前进程的
`MYSQL_HOST`、`MYSQL_PORT`、`MYSQL_USER`、`MYSQL_PASSWORD` 环境变量传入；
执行器会重建专用临时库 `bizdevops_contract_test`，并在成功或失败后删除它：

```powershell
$env:MYSQL_HOST = "127.0.0.1"
$env:MYSQL_PORT = "3306"
$env:MYSQL_USER = "root"
Read-Host "Database password" | Set-Item Env:MYSQL_PASSWORD
try {
    python db/tests/run_mysql_integration.py --environment
} finally {
    Remove-Item Env:MYSQL_PASSWORD -ErrorAction SilentlyContinue
}
```

不要把真实密码写入 `.env.example`、仓库文件或命令行参数。
