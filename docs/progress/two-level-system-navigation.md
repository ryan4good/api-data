# 单系统两级导航与设置拆页

更新时间：2026-07-12（Asia/Shanghai）

## 根因

单系统侧栏原先只有扁平 `NavItem[]`，代码源、环境、密钥引用和成员又被聚合到同一个 `/settings` 页面。随着功能增加，导航没有领域层级，设置页也同时加载多个无关接口，导致页面过长且难以定位。

## 信息架构

- 工作台
  - 系统概览
- 资产管理
  - 代码源
  - 代码扫描
  - API 资产
- 场景管理
  - 场景发现
  - 待核验场景
  - 场景编排
- 执行中心
  - 运行记录
- 系统管理
  - 环境与密钥
  - 成员与权限

一级分组只表达领域，二级菜单才是可点击路由。侧栏导航区域独立滚动，窄屏隐藏一级文字但保留二级图标和当前激活项。

## 路由拆分

- `/systems/{systemId}/code-sources`
- `/systems/{systemId}/settings/environments`
- `/systems/{systemId}/settings/members`
- 旧 `/systems/{systemId}/settings` 保留并重定向到环境与密钥页。

代码源、环境和成员页面现在只加载自身所需接口；扫描与发现的空状态直接链接到代码源二级页。

## 验证与部署

- 干净提交：22 个 Web 测试文件、103 项测试通过。
- TypeScript 检查和 `/bizdevops/` 生产构建通过。
- Web release：`20260712171546-two-level-nav`。
- 远端 bundle 已验证包含“资产管理”“系统管理”和 `.nav-group-title` 样式。
- API、Worker、Nginx、MariaDB active；未认证 `/bizdevops/` 为 401；原 `/`、`/api`、`/ai-data/` 保持 200/404/200。
- 前一 Web 回滚目标：`/opt/bizdevops/releases/20260712170231-settings-layout/web`。

