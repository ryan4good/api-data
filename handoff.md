# BizDevOps 平台接力说明

更新时间：2026-07-12（Asia/Shanghai）

## 1. 当前目标

继续完成腾讯云试运行部署。用户已授权使用 `docs/tencent.txt` 中的 SSH 信息连接服务器，并同意在现有 Nginx 中增加独立路径。推荐部署入口为 `/bizdevops/`，API 仅监听远端回环地址 `127.0.0.1:18080`。

本轮部署尚未真正修改远端服务器；目前只完成了只读审计、前端子路径适配和部署配置草稿。

## 2. Git 状态

- 仓库：`git@github.com:ryan4good/api-data.git`
- 当前分支：`feature/platform-refactor-e2e`
- 远端分支：`origin/feature/platform-refactor-e2e`
- 已推送提交：
  - `43e74e9 feat: build system-scoped API scenario platform`
  - `f75e8d5 feat: add async worker secrets and management overview`
- PR 创建入口：<https://github.com/ryan4good/api-data/pull/new/feature/platform-refactor-e2e>

当前未提交改动：

- `apps/web/src/api/client.test.ts`
- `apps/web/src/api/client.ts`
- `apps/web/src/router.tsx`
- `apps/web/vite.config.ts`
- `deploy/tencent/`（新增）
- `handoff.md`（本文件）
- `docs/tencent.txt`（本地凭据文件，绝对不能提交）

接手后先执行：

```powershell
rtk git status --short --branch
rtk git diff -- apps/web/src/api/client.ts apps/web/src/api/client.test.ts apps/web/src/router.tsx apps/web/vite.config.ts
rtk git diff --no-index NUL deploy/tencent/README.md
```

注意：工作区所有可执行 shell 命令需按 `C:\Users\ryanf\.codex\RTK.md` 要求以 `rtk` 开头。

## 3. 已完成系统能力

技术栈为 React + TypeScript、Go、MySQL/MariaDB。当前实现包括：

- 业务系统维度的 JWT/RBAC 与数据隔离。
- 代码扫描和 API 资产库。
- Postman 2.1 / Scenario Bundle 场景导入。
- 从代码、PRD 或一句话需求发现 P0 场景候选。
- 候选人工接受/拒绝并提升为场景、版本和步骤。
- 同步执行、执行到指定步骤、失败重试和步骤尝试记录。
- 异步执行队列、租约、心跳、取消、超时、幂等与独立 Worker。
- 环境和密钥引用；数据库只保存 secret reference，不保存真实 secret value。
- 管理视角总览以及单业务系统工作区。

各模块 TDD 和接力记录位于 `docs/progress/`，总览可先读：

- `docs/progress/root-end-to-end-scenario.md`
- `docs/progress/root-operations-integration.md`
- `docs/progress/execution-async-worker.md`
- `docs/progress/environment-secret-resolver.md`
- `docs/progress/frontend-management-overview.md`

## 4. 最近一次已通过验证

在提交 `f75e8d5` 前以下检查均已通过：

- API：`go test ./...`
- API：`go vet ./...`
- Web：10 个测试文件、44 个测试通过。
- Web：生产构建通过。
- DB：9 个系统隔离测试通过。
- 本地真实 MariaDB migration runner 和端到端测试通过。
- 管理总览和单系统摘要已通过真实 API 浏览器验证，无 console warning/error。

部署子路径改动额外完成了 Red→Green：

- `resolveApiBaseUrl('/bizdevops/')` 测试先失败后通过。
- `createBrowserRouter` 使用 `import.meta.env.BASE_URL` 作为 basename。
- Vite 支持 `VITE_BASE_PATH=/bizdevops/`。
- 使用临时环境变量构建后，`dist/index.html` 资源路径为 `/bizdevops/assets/...`。

部署前仍应重新跑一次完整检查，避免未提交增量引入回归。

## 5. 腾讯云只读审计结果

SSH 信息位于 `docs/tencent.txt`。不要输出文件内容，不要把密码放进命令行日志，不要提交该文件。推荐使用 Python Paramiko 读取文件并建立连接。

已确认的远端状态：

- Debian 13，x86_64，SSH 用户为 root。
- 已核验 SSH host key SHA256：`BUtVmsbcLGzAuIkgHCjD3JY+mC3bmxr3kwUcuH+kQ5U`。
- Docker / Compose 未安装。
- Go 未安装；Node.js 20/npm 已安装。
- Nginx 1.26.3 正常运行，对外监听 8080。
- MariaDB 11.8.6，只监听回环地址 3306，root 可用 Unix socket 管理。
- 80 端口由一个现有 Go 程序直接占用，绝对不要动。
- 现有服务包括 stock-analyzer、ai-data-mvp、rent-platform，部署不得影响它们。
- 现有 Nginx 站点：`/etc/nginx/sites-available/stock-analyzer`，已启用。
- 现有 8080 路由包含 `/api`、`/ai-data/` 和根 SPA。
- 磁盘和内存充足。

建议采用原生 systemd + 交叉编译 Go 二进制 + 静态 React 文件，不为本次试运行安装 Docker。

## 6. 当前部署草稿

新增文件：

- `deploy/tencent/bizdevops-api.service`
- `deploy/tencent/bizdevops-worker@.service`
- `deploy/tencent/nginx-bizdevops.conf`
- `deploy/tencent/api.env.example`
- `deploy/tencent/README.md`

草稿约定：

- Web 路径：`/bizdevops/`
- API 代理路径：`/bizdevops/api/`
- API 上游：`127.0.0.1:18080`
- API 二进制：`/opt/bizdevops/bin/bizdevops-api`
- Worker 二进制：`/opt/bizdevops/bin/bizdevops-worker`
- Web 静态目录：`/var/www/bizdevops/`
- 环境文件：`/etc/bizdevops/api.env`
- systemd 用户：`bizdevops`
- Nginx Basic Auth 文件：`/etc/nginx/.htpasswd-bizdevops`

正式落地前需审查 Nginx SPA fallback。当前 `alias` 与 `try_files ... /bizdevops/index.html` 的组合应在一份临时 Nginx 配置或远端 `nginx -t` 中验证；如果实际请求出现内部重定向问题，改为可靠的 named location 或 `root` 映射并补静态文件 smoke test。

## 7. 安全边界

系统尚未实现正式登录页面。试运行方案只能是：

- 后端设置 `AUTH_MODE=development`。
- 前端构建时设置开发用户 UUID。
- `/bizdevops/` 和 `/bizdevops/api/` 全部置于 Nginx Basic Auth 后。
- Basic Auth 生效前不得暴露开发身份 Header 模式。
- API 只能监听 `127.0.0.1:18080`，不能监听公网地址。

需在服务器生成随机数据库密码和 Basic Auth 密码；不要复用 SSH 密码或本地 MySQL 密码。数据库建议新建：

- database：`bizdevops`
- user：`bizdevops`@`127.0.0.1`
- 仅授权 `bizdevops.*`

可把试运行访问账号写入远端 `/root/bizdevops-trial-access.txt` 并设为 `0600`，最终只把访问方式安全地交给用户，不写入仓库。日志、终端输出和提交中都不要出现 DSN、SSH 密码或 Basic Auth 密码。

## 8. 推荐接力步骤

严格继续 TDD：先为部署约束补会失败的静态测试，再完成 Green。

1. 审查未提交改动，确认 `docs/tencent.txt` 未被暂存，并把它加入本地/仓库 ignore（若尚未覆盖）。
2. 为部署文件增加自动校验，至少覆盖：无明文 secret、API 仅绑定回环地址、Nginx 两个路径都启用 Basic Auth、systemd 使用独立用户、前端 base path 正确。
3. 运行完整本地测试与构建。
4. 交叉编译 Linux amd64 API 和 Worker，产物放临时目录，不提交二进制或 `dist/`。
5. 通过 Paramiko SFTP 上传到远端 staging 目录，备份待覆盖文件后原子替换。
6. 创建 `bizdevops` 系统用户及最小权限目录。
7. 创建独立 MariaDB 数据库/用户，按文件名顺序应用所有 `db/migrations/*.up.sql`，试运行可应用 `db/seeds/000001_development.sql`。
8. 写入 `/etc/bizdevops/api.env`（建议 `0640 root:bizdevops`），启动 API 和 OMS Worker。
9. 上传 Nginx snippet，在现有 server block 中只增加一条 include。修改前备份原站点；执行 `nginx -t` 成功后才 reload，失败立即恢复备份。
10. 做远端 smoke test和日志检查。
11. 新增 `docs/progress/tencent-trial-deployment.md` 记录 Red、Green、远端验证和回滚方法。
12. 只提交源代码、部署模板、测试和文档，推送到当前 feature 分支。

建议启用的 Worker：

```text
bizdevops-worker@20000000-0000-4000-8000-000000000001.service
```

试运行前端开发用户：

```text
10000000-0000-4000-8000-000000000001
```

## 9. 本地构建命令

API 与 Worker：

```powershell
cd D:\work\tools\bizDevOps\apps\api
rtk go test ./...
rtk go vet ./...
$env:GOOS='linux'
$env:GOARCH='amd64'
$env:CGO_ENABLED='0'
rtk go build -o "$env:TEMP\bizdevops-api" ./cmd/server
rtk go build -o "$env:TEMP\bizdevops-worker" ./cmd/worker
```

Web：

```powershell
cd D:\work\tools\bizDevOps\apps\web
$env:VITE_BASE_PATH='/bizdevops/'
$env:VITE_DEV_USER_ID='10000000-0000-4000-8000-000000000001'
rtk npm test -- --run
rtk npm run build
```

数据库验证参考根 README 和 `db/tests/run_mysql_integration.py`。本地 MySQL 已启动，连接信息由用户在当前会话提供；只能通过临时环境变量传入，不要写入任何文件。

## 10. 远端验收清单

部署完成必须同时满足：

- `systemctl is-active bizdevops-api` 为 active。
- OMS Worker 实例为 active。
- `curl http://127.0.0.1:18080/healthz` 返回成功。
- 未带 Basic Auth 请求 `/bizdevops/` 返回 401。
- 带 Basic Auth 请求 `/bizdevops/` 返回 200，JS/CSS asset 返回 200。
- 带 Basic Auth 和 `X-Dev-User-ID: 10000000-0000-4000-8000-000000000001` 请求 `/bizdevops/api/v1/management/overview` 返回 200。
- 原有 `/`、`/api`、`/ai-data/` 行为未变化。
- `nginx -t` 通过，API/Worker/Nginx 日志无循环重启、权限错误、数据库错误或 secret 泄漏。

## 11. 回滚原则

- Nginx：恢复修改前的站点备份，`nginx -t` 后 reload。
- systemd：stop/disable `bizdevops-api` 和对应 Worker 实例，移除新增 unit 后 daemon-reload。
- 文件：恢复 `/opt/bizdevops`、`/var/www/bizdevops` 的上一个版本或删除本次独立目录。
- 数据库：试运行 DB 独立，确认无需要保留的数据后再由用户授权删除；不要自动执行破坏性删除。
- 不得停止、覆盖或重启现有 stock-analyzer、ai-data-mvp、rent-platform 服务。

## 12. 尚未完成的产品级事项

腾讯云部署跑通后，下一阶段优先级建议为：

1. 正式登录与身份提供方集成，移除 development identity。
2. Vault/AWS/GCP SecretProvider adapter，并在 Worker 执行前注入 Resolver。
3. `audit_events` migration、写入器和管理端审计页面。
4. 真实并发 Worker E2E、容量限制和任务可观测性。
