# AIA API 用户指南

## 登录与创建令牌

进入管理员提供的 AIA API 网页（AIA 站点为 `https://api.nju-aia.com`），选择 **AIA 论坛登录**。系统通过管理员配置的 OIDC 身份提供方验证用户并按策略创建账号；**不提供普通密码注册或密码登录**。

在网页的 API 令牌管理处创建 Token。这是调用 AIA API 所需的平台凭据，**不同于**向共享池提交的第三方开发者 API Key。请勿将两者混用。平台 Token 有自身的权限、额度、模型及 IP 限制。

## 调用模型

获取当前实际开放的模型 ID 后，使用 OpenAI 兼容接口。`GET /v1/models` 可使用同一个 Bearer Token 获取模型列表。

```bash
curl https://api.nju-aia.com/v1/models \
  -H 'Authorization: Bearer YOUR_AIA_API_TOKEN'

curl https://api.nju-aia.com/v1/chat/completions \
  -H 'Authorization: Bearer YOUR_AIA_API_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{"model":"YOUR_AVAILABLE_MODEL_ID","messages":[{"role":"user","content":"你好"}],"max_tokens":128}'
```

模型目录中的预设型号不代表任何指定上游的实时可用性。OpenAI Responses 等兼容路由仍可能由管理员已有的原生渠道提供；**用户贡献的密钥仅支持受约束的纯文本 Chat Completions 路由**，不能据此推断所有模态、工具调用或协议都可通过贡献密钥使用。

## 额度与兑换码

在 `/wallet` 查看现有额度，输入管理员发放的兑换码兑换额度。平台额度与模型使用记录由统一账本管理；价格参考采用 CNY，实际消耗以站点配置和结算为准。平台不提供现金充值、第三方支付或商业订阅购买入口。**已存在的订阅权益**可能继续有效，具体由管理员管理。

## 贡献开发者 API Key

1. 确认自己拥有允许以共享/转发形式使用的**开发者 API Key**，并知晓其按次收费、限额和服务条款。
2. 打开 `/shared-pool`，选择允许接入的服务端点和模型，设置每日共享预算，提交密钥。
3. 平台对模型逐一发起少量真实请求验证权限，可能产生上游消耗。仅验证通过的模型可以映射到共享渠道。
4. 提交后可以在共享池查看自己的密钥提示、模型、状态、已用预算，并暂停或删除贡献。服务端对密钥进行加密存储，不会向其他用户返回明文。
5. 他人成功调用并完成平台额度结算后，贡献者获得对应额度；额度不等于可提现现金。暂时 429 或额度耗尽会冷却一段时间，之后尝试恢复。

**不能贡献：** ChatGPT/Codex、Claude Code、Antigravity IDE 等订阅 Cookie、OAuth Refresh Token 或未授权的第三方代理密钥。即使能调用成功，也不代表条款允许分享。上游可能收到请求内容，不应发送敏感材料。

## 遇到问题

- `401`：检查平台 API Token 是否正确、是否被撤销。不要使用上游 Key 直接调用 AIA 网关。
- 模型不可用：确认模型 ID、Token 限制和管理员启用的渠道；仅展示在模型目录不保证可调用。
- 共享池 `503`：通常表示服务器未正确设置 `AIA_SHARED_SECRET`，需管理员处理。
- 贡献被拒：检查服务端点接纳状态、API Key、预算和模型验证结果；不要无限次重复验证造成上游费用。
- 兑换失败：检查兑换码是否有效，联系管理员；不应在公开 Issue 中暴露密钥或兑换码。
