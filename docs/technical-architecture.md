# API Data Intelligence Platform 技术架构与模块拆分

## 1. 技术栈基线

| 层级 | 选型 |
|---|---|
| Web | React + TypeScript + Vite |
| API / Worker | Go，模块化单体，HTTP JSON API |
| Database | MySQL 8.0，InnoDB，utf8mb4 |
| 场景交换 | 原生 Scenario Bundle JSON Schema；兼容 Postman Collection 2.1 |
| 实时运行状态 | SSE；第一阶段可以轮询降级 |
| 文件存储 | MVP 使用受控本地目录，后续接对象存储 |
| 后台任务 | MySQL 任务表 + Worker；规模扩大后再引入专用队列 |

第一阶段采用模块化单体，不立即拆成微服务。模块在代码、数据表和接口层保持边界，后续可以按扫描、执行等高负载模块独立部署。

## 2. 仓库结构

```text
apps/
  web/                         React + TypeScript 管理控制台
  api/                         Go 后端
    cmd/server/                HTTP API 进程
    cmd/worker/                扫描、发现、导入和执行任务进程
    internal/
      platform/                配置、数据库、HTTP、中间件、任务基础设施
      identity/                用户身份和平台角色
      system/                  业务系统、成员、数据范围
      scanner/                 代码源和扫描任务
      catalog/                 API 资产、模型和版本差异
      discovery/               代码/需求/PRD 场景发现
      importer/                Scenario/Postman 导入
      scenario/                候选、核验、编排和版本发布
      connector/               环境、连接器、密钥引用和安全策略
      execution/               场景执行引擎
      runrecord/               运行、步骤、断言和上下文记录
      governance/              审计、风险和管理视角读模型

contracts/
  scenario-bundle.schema.json  原生场景协议
  examples/                    协议示例

db/
  migrations/                  MySQL 版本化迁移

docs/
  refactoring-blueprint.md     产品与总体重构方案
  technical-architecture.md    本文档
  technical-contracts.md       数据和协议契约
  wireframes/                  HTML 线框原型
```

## 3. 前端模块

### 3.1 全局管理空间

- `dashboard`：跨系统指标、P0 覆盖、风险和运行趋势。
- `systems`：当前用户已授权的业务系统。
- `global-runs`：权限范围内的跨系统运行记录。
- `profile`：个人系统角色和待办。

### 3.2 单业务系统空间

- `system-overview`：当前系统资产、场景、运行和风险。
- `code-sources`：Git 仓库、本地扫描代理和扫描历史。
- `api-catalog`：API 资产、模型、代码证据和变更。
- `scenario-discovery`：代码挖掘、一句话、PRD、场景包导入。
- `scenario-review`：P0、证据、API 匹配和跨系统核验。
- `scenario-editor`：步骤编排、单步编辑和部分执行。
- `system-runs`：当前系统相关运行。
- `system-settings`：成员、环境、连接器、安全和审计。

### 3.3 前端边界

- 页面不得自行拼接跨系统数据；所有查询必须携带明确的系统上下文。
- 前端菜单隐藏只用于体验，不能替代后端授权。
- API Client 统一处理身份、当前系统、错误模型和请求追踪 ID。
- 服务端 DTO 与前端类型分层，禁止页面直接依赖数据库字段。

## 4. Go 后端模块

### 4.1 Identity

职责：

- 用户身份。
- 平台管理员和管理查看者。
- 登录会话或外部身份映射。

不负责系统内角色；系统角色由 System 模块管理。

### 4.2 System & Access

职责：

- 业务系统注册和状态。
- 系统成员关系。
- Owner、Maintainer、Reviewer、Runner、Viewer 角色。
- 系统级数据范围授权。

所有系统资源查询必须先通过该模块完成访问判断。

### 4.3 Scanner

职责：

- 代码源配置。
- 创建扫描任务。
- 调用语言扫描插件。
- 保存原始扫描产物和证据。

输出标准化扫描事件，不直接生成正式 API 或场景。

### 4.4 API Catalog

职责：

- 接收扫描产物并生成 `ApiOperation`。
- 管理请求/响应模型、代码证据和置信度。
- 计算新增、删除和签名变化。
- 导出 OpenAPI/Postman。

### 4.5 Scenario Discovery

支持四种来源：

- 从代码调用图、状态流转和测试代码发现。
- 从一句话需求匹配 API。
- 从 PRD 提取业务事实。
- 从导入场景包生成可复用草稿。

只生成候选和证据，不绕过人工核验直接发布。

### 4.6 Scenario Importer

职责：

- 识别 Scenario Bundle 或 Postman Collection 2.1。
- 转换请求、变量、脚本和断言。
- 检查明文密钥、危险脚本和未绑定 API。
- 输出导入报告和 DRAFT 场景。

Importer 不执行场景，也不管理环境密钥。

### 4.7 Scenario

职责：

- 候选场景人工核验。
- 场景步骤和参数映射。
- 单步骤草稿修改。
- 场景版本、审核、发布和下线。
- 跨系统 Reviewer 协作。

已发布版本不可原地修改；编辑必须生成新草稿版本。

### 4.8 Connector

职责：

- 测试环境和 Base URL。
- 认证方式与密钥引用。
- 超时、重试和安全黑名单。
- 连接器健康检查。

敏感值不通过普通 DTO 返回前端。

### 4.9 Execution

职责：

- 完整执行。
- 执行到指定步骤。
- 单步执行。
- 从失败步骤重试。
- 参数渲染、变量提取和断言。
- 超时、取消、并发和失败策略。

Execution 只读取不可变的场景版本快照，并将事件交给 Run Record。

### 4.10 Run Record

职责：

- 场景运行、步骤运行和断言结果。
- 上下文快照。
- 请求/响应脱敏记录。
- SSE 运行事件。
- 日志归档策略。

### 4.11 Governance

职责：

- 审计日志。
- 跨系统管理视角读模型。
- API 变更影响和场景风险。
- 用户待办。

Governance 只消费其他模块事件，不反向修改业务模块。

## 5. 依赖规则

```text
Identity ───────┐
System/Access ──┼──> 所有系统级模块
                │
Scanner ────────┴──> API Catalog
API Catalog ───────> Discovery ───> Scenario Candidate
Importer ─────────────────────────> Scenario Draft
Scenario + Connector ─────────────> Execution
Execution ────────────────────────> Run Record
各模块事件 ───────────────────────> Governance
```

规则：

1. 模块间通过服务接口或领域事件交互，禁止直接操作其他模块的 Repository。
2. Scanner 不依赖 Scenario。
3. Discovery 不依赖 Execution。
4. Importer 不依赖 Scanner；它只依赖 Catalog 查询接口和 Scenario 草稿接口。
5. Execution 不修改场景定义。
6. Governance 不成为核心写链路的同步依赖。

## 6. API 组织

```text
/api/v1/me
/api/v1/systems
/api/v1/systems/{systemId}/members
/api/v1/systems/{systemId}/code-sources
/api/v1/systems/{systemId}/scans
/api/v1/systems/{systemId}/apis
/api/v1/systems/{systemId}/discoveries
/api/v1/systems/{systemId}/scenario-imports
/api/v1/systems/{systemId}/scenario-candidates
/api/v1/systems/{systemId}/scenarios
/api/v1/systems/{systemId}/environments
/api/v1/systems/{systemId}/runs
/api/v1/runs/{runId}
```

跨系统场景由主责系统路由管理，参与系统通过场景授权关系获得受限访问。

## 7. 同步请求与后台任务

同步处理：

- 系统、成员和环境配置。
- API 资产查询。
- 场景草稿编辑。
- 人工核验和版本发布。

后台任务：

- 代码拉取与扫描。
- 代码场景发现。
- PRD 分析。
- Postman/Scenario 导入转换。
- 场景执行。
- 大型导出和日志归档。

后台任务统一记录状态、进度、发起人、系统、错误和重试次数。

## 8. 第一条端到端竖切

第一条竖切固定为 OMS/WMS 库存分配：

1. 创建 OMS 和 WMS 业务系统。
2. 为用户分配 OMS Owner、WMS Reviewer。
3. 注册并扫描示例 Go 仓库。
4. 生成 API 资产。
5. 从代码调用关系生成库存分配场景候选。
6. 人工核验跨系统 API。
7. 保存并发布 Scenario Bundle v1。
8. 执行到“分配库存”步骤。
9. 查看步骤、断言、业务单号和失败记录。

## 9. 并行实施边界

以下工作可并行：

- Web：路由、布局、页面壳和 API Client。
- Go：HTTP 服务、模块骨架和授权中间件。
- Data/Contracts：MySQL 迁移、Scenario Schema 和 Postman 映射。

首次集成点：

- 统一系统 ID、用户 ID、API ID、场景 ID 和运行 ID 的类型。
- 统一分页、错误、任务状态和时间格式。
- 统一 Scenario Bundle Schema。
- 以 `/api/v1/status` 和系统列表作为前后端第一次联调。

## 10. TDD 开发约束

所有功能采用 Red → Green → Refactor：

1. Red：先写描述外部行为的测试并运行，确认测试因功能缺失而失败。
2. Green：只实现让当前测试通过的最小代码。
3. Refactor：在全部测试保持通过的前提下整理结构、命名和重复逻辑。

提交要求：

- 每个功能提交必须包含对应测试。
- PR 或任务记录必须说明 Red 时的失败原因和 Green 后的通过结果。
- 不接受先完成实现、再补一个永远不会失败的测试作为 TDD 证据。
- Bug 修复必须先增加可以复现该 Bug 的回归测试。
- 数据库迁移先写结构/约束验证，再补迁移。
- JSON Schema 先准备非法/合法样例，再实现 Schema。
- 前端优先测试路由、权限可见性、用户操作和状态转换，不依赖脆弱的样式快照。
- 后端优先测试领域服务、权限边界、HTTP 契约和持久化约束。

测试分层：

| 层级 | 目标 |
|---|---|
| 单元测试 | 领域规则、解析器、转换器、执行步骤 |
| 模块测试 | 模块服务与 Repository 接口 |
| HTTP 契约测试 | 路由、状态码、错误结构和权限 |
| 数据库集成测试 | MySQL 迁移、索引、外键和事务 |
| 前端组件测试 | 用户可见行为、系统上下文和权限状态 |
| 端到端测试 | 关键 OMS/WMS 竖切闭环 |

CI 顺序：

```text
格式检查
  -> 单元测试
  -> 契约测试
  -> MySQL 集成测试
  -> 前端构建
  -> Go 构建
  -> 关键端到端测试
```
