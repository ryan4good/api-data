# 功能细化：场景编排、全局运行与成员管理

更新时间：2026-07-12（Asia/Shanghai）

## 本轮完成

- 场景修订：新增 `PUT /api/v1/systems/{systemId}/scenarios/{scenarioId}`。仅 owner/maintainer 可写；每次保存创建不可变的新版本和新步骤，旧版本不修改。
- 场景校验：严格校验状态、UUID、步骤类型、唯一 Key、依赖存在性、重复/自依赖和有向环；MySQL 在事务内锁定系统范围内的场景并以 CAS 更新当前版本。
- 场景编辑器：指定场景可加载、增删步骤、编辑请求配置并保存新版本；reviewer/runner/viewer 严格只读。工作区真实场景记录可直接进入编辑器。
- 创建路径：后端没有空白场景创建契约，因此无 `scenarioId` 时明确引导从候选核验或导入生成场景，不提供虚假创建表单。
- JSON 契约修复：`requestConfig` 输出改为 JSON 对象，修复 `[]byte` 被编码为 Base64、导致 GET 后无法直接回填 PUT 的根因。
- 全局运行记录：按授权系统并发聚合真实运行，支持部分系统失败、全失败、空数据和详情；详情使用 `systemId + runId` 安全定位并展示步骤尝试与断言。
- 系统成员：系统 owner 可读取成员、添加已有平台用户和更新系统角色；其他系统角色既不调用成员接口，也不暴露成员目录。
- 清理误导入口：删除未使用的静态假仪表盘和无后端契约的“新建业务系统”按钮。

## 验证

- API：`go test -count=1 ./...` 通过。
- API：`go vet ./...` 通过。
- Web：18 个测试文件、87 项测试通过。
- Web：TypeScript 检查通过；`VITE_BASE_PATH=/bizdevops/` 且不设置 `VITE_DEV_USER_ID` 的生产构建通过。
- Python DB/腾讯云部署静态契约：21 项通过。
- 修复一个时间型测试根因：JWT 测试按固定时钟签发，却按真实时钟校验；测试现在为解析器注入同一固定时钟，生产认证逻辑未改。

## 未在本轮处理

- 未修改腾讯云公网 release、Nginx、systemd、MariaDB 或现有服务；当前 HTTP + Basic Auth + development identity 试运行状态保持不变。
- 用户已明确暂不处理域名解析；在可信 TLS 完成前仍不得公开切换正式 Cookie JWT 登录。
- 平台设置、新建业务系统仍缺少平台级授权与写接口契约，页面保留诚实说明，后续应先设计 platform admin/auditor 的服务端权限边界再实现。

