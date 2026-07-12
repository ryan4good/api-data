# Session：tencent-trial-deployment

## 基本信息

- 状态：completed
- 部署时间：2026-07-12（Asia/Shanghai）
- 远端 release：`20260712031205`
- 范围：腾讯云 Debian 13 原生 systemd + MariaDB + Nginx 试运行部署
- 入口：`/bizdevops/`
- API：`127.0.0.1:18080`
- Worker：`bizdevops-worker@20000000-0000-4000-8000-000000000001.service`

## Red

- 新增 `db/tests/test_tencent_deployment_contract.py` 后，凭据文件 ignore 约束首次失败：`docs/tencent.txt` 尚未被 `.gitignore` 覆盖。
- 其余静态安全约束在 Red 基线上已满足：API 回环监听、UI/API 双路径 Basic Auth、独立 systemd 用户、模板无明文 secret、前端与代理统一 `/bizdevops/` base path。
- 首次远端 smoke test 中，未认证 UI 正确返回 401，但认证 UI 失败。部署流程自动恢复原 Nginx 站点并停用新服务；根因是 htpasswd 的 `0600 root:root` 权限使 Nginx worker 无法读取。

## Green

- `.gitignore` 新增 `/docs/tencent.txt`；敏感 SSH 文件保持未跟踪、未暂存。
- 部署约束 6 项全部通过，并将 SPA fallback 从有歧义的 `alias` 组合改为 `root /var/www` + `try_files ... /bizdevops/index.html`。
- htpasswd 使用 `0640 root:www-data`，访问凭据仅保存在远端 `/root/bizdevops-trial-access.txt`（`0600 root:root`）。
- API 环境文件为 `/etc/bizdevops/api.env`（`0640 root:bizdevops`）；数据库密码和 Basic Auth 密码均独立随机生成，未写入仓库或终端输出。
- Linux amd64 API 与 Worker 由 Go 1.22.12、`CGO_ENABLED=0`、`-trimpath` 交叉编译到本地临时目录；二进制和 Web `dist/` 未纳入提交。
- 产物先经 Paramiko SFTP 上传到 root-only staging，再落入版本化 release，通过符号链接原子切换 API/Worker 与 Web。
- 新建 `bizdevops` 系统用户；新建独立 `bizdevops` MariaDB database 和 `bizdevops`@`127.0.0.1` 用户，只授权 `bizdevops.*`。
- 按文件名应用全部 `db/migrations/*.up.sql` 和 development seed，远端共验证 19 张表。
- 现有 Nginx 站点只新增 `/etc/nginx/snippets/bizdevops.conf` include；修改前生成时间戳备份，`nginx -t` 成功后才 reload。

## 本地验证

- `go test ./...`：通过。
- `go vet ./...`：通过。
- Web Vitest：10 个测试文件、45 个测试通过。
- Web production build：通过，`dist/index.html` 资源路径确认使用 `/bizdevops/assets/`。
- Python DB/部署静态契约：15 项通过。
- Legacy root smoke：通过（临时使用 3137，复用既有 mock 服务；仅停止本次启动的临时进程）。
- 本地 MariaDB 环境变量凭据未传入本次接续任务，空密码 root 被拒绝，因此没有对本机数据库做破坏性凭据恢复；真实 migration、seed、表结构和 API 查询改由远端独立 MariaDB 完成验证。

## 远端验收

- SSH host key 使用 handoff 中记录的 SHA256 指纹严格校验；未知密钥不会自动接受。
- `bizdevops-api.service`：active。
- OMS Worker：active。
- `127.0.0.1:18080/healthz`：成功；18080 仅绑定 `127.0.0.1`。
- 未带 Basic Auth 的 `/bizdevops/`：401。
- 带 Basic Auth 的 `/bizdevops/`：200；构建生成的 JS/CSS asset：200。
- 带 Basic Auth 与试运行开发身份 Header 的 `/bizdevops/api/v1/management/overview`：200。
- 原路由回归：`/` 200、`/api` 404、`/ai-data/` 200，与部署前一致；80 端口现有 Go 服务未修改。
- `nginx -t`：通过。
- API/Worker 最近日志未发现权限、数据库、循环重启错误，也未发现 `MYSQL_DSN`、`@tcp(...)` 或密码字段泄漏标记。

## 回滚

1. 停止并禁用 `bizdevops-api.service` 与 OMS Worker；只操作 BizDevOps 新增 unit，绝不停止 stock-analyzer、ai-data-mvp 或 rent-platform。
2. 恢复 `/etc/nginx/sites-available/stock-analyzer.bizdevops-backup-<timestamp>` 到原站点，执行 `nginx -t`，成功后 reload Nginx。
3. 将 `/opt/bizdevops/bin` 与 `/var/www/bizdevops` 符号链接切回上一个 `/opt/bizdevops/releases/<release>`；确认后重启 BizDevOps 自身服务。
4. 若完全撤销 systemd，移除新增 unit 后执行 `systemctl daemon-reload`。
5. 独立试运行数据库不自动删除；确认无数据需要保留后，必须另获用户授权才能删除 database/user。

## 访问凭据

访问 URL、用户名和随机 Basic Auth 密码只保存在服务器 `/root/bizdevops-trial-access.txt`，权限为 `0600`。通过已验证 SSH 连接在服务器本地读取，不要复制到仓库、issue、日志或聊天记录。

## 生产身份 Header 修复

- 用户首次访问时 UI 全部显示 `development user header is required`。根因确认是前端用 `import.meta.env.DEV` 门控 `VITE_DEV_USER_ID`；Vite 生产构建中 `DEV=false`，导致部署时显式提供的试运行 UUID 被丢弃。
- 保留 Nginx Basic Auth，未移除后端 RBAC；仅让显式配置的 `VITE_DEV_USER_ID` 在受 Basic Auth 保护的生产试运行构建中生效。
- 新增 TS 回归测试和部署静态契约，完成 Red→Green；Web 现为 10 个测试文件、46 个测试通过。
- 修复 release：`20260712033413-identity-hotfix`。远端确认 bundle 含试运行 UUID，未授权 UI 401，授权 UI/asset 200，管理总览 API 200，原路由行为不变。
