# AIA API 管理员指南

管理员管理 **模型目录**、**上游端点**和**用户贡献**三类资源，不能把“目录中存在模型”当成“该上游可调用”。

## 登录与访问控制

使用 AIA Forum OIDC 登录后，管理操作需要管理员权限；共享池中的模型、端点及赠送额度 API 使用 **RootAuth**，普通用户不可访问。部署时优先完成 OIDC、HTTPS、会话密钥及数据库备份。

## 维护模型和服务端点

- `GET /api/shared/admin/models`、`POST /api/shared/admin/models`、`PUT /api/shared/admin/models/:id`：查看或调整模型 ID、协议、输入/输出单价、最大输出及启用状态。模型新增时默认 `name=code`，不需要额外显示名称。
- `GET /api/shared/admin/providers`、`POST /api/shared/admin/providers`、`PUT /api/shared/admin/providers/:id`：管理上游的 HTTPS Base URL、协议、启停及是否接纳新贡献。
- `PUT /api/shared/admin/providers/:id/models`：把模型目录中的 `model_id` 映射到服务端点。无正确单价或未验证的模型不可投入消费路由。
- 自定义贡献端点只接受合法 HTTPS 地址，后端限制本机/内网地址和非预期路径，并禁止 HTTP 重定向。

内置模型目录包含 `deepseek-flash`、`deepseek-v4-pro`、`gpt-6.1-sol`、`gpt-6-sol`、`gpt-6-luna`、`claude-sonnet-4-6`、`claude-opus-4-8`、`gemini-3.8-flash`、`gemini-3.6-flash`、`gemini-4-argon`、`kimi-k2.5`。**仅表示预置标识，不是可用性、官方供应或当前价格的承诺。** 参考单价单位为 CNY/百万 Token，未确认价格可能为 0，必须由运营者验证后启用计费。

## 共享密钥、贡献者和奖励

用户通过 `/shared-pool` 提交的开发者密钥需要在合规前提下逐模型验证。贡献按日限额预留预算；共享路由在请求成功结算后才归还贡献者额度。`POST /api/shared/admin/grants` 允许 Root 管理员提供幂等的引导额度，调用前要检查授权及审计记录。**共享额度不是上游实际账单，也不可提现。**

429 或短时上游额度不足会使贡献渠道临时冷却；不要把这种状态当成永久失效。管理员可以分别关闭端点接受新贡献与路由功能。关闭时应评估现有贡献、账单和 Token 的影响。

## 与 New API 的边界

上游代码的部分通用渠道类型、SQL 表和内部包名仍保留；AIA 面向用户的现金充值、支付、商业购买和密码注册入口已被裁剪。管理员仍可管理历史订阅及兑换码。问题应在 [AIA API 仓库](https://github.com/fwerkor/aia-api/issues) 跟踪，而不是提交给不负责本 Fork 的上游维护者。详见 [共享池设计](../AIA-SHARED-POOL.md) 和 [API 参考](api.md)。
