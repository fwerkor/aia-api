# AIA API 接口参考

此文档列出与 AIA 当前核心工作流相关的路由。完整服务端仍可能包含继承自 New API 的兼容路径，**并不意味着该路径可由 AIA 的贡献共享渠道使用**。下面的 URL 应根据自部署域名替换。

## 模型调用（Bearer 平台 Token）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/v1/models` | 查询平台 Token 可见的模型 |
| `POST` | `/v1/chat/completions` | OpenAI Chat Completions 兼容接口，贡献共享池主要路由 |
| `POST` | `/v1/responses` | 原生渠道兼容路由，是否受支持取决于启用的渠道；不是共享贡献渠道保证功能 |

```bash
curl https://api.nju-aia.com/v1/chat/completions \
  -H 'Authorization: Bearer YOUR_AIA_API_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{"model":"YOUR_AVAILABLE_MODEL_ID","messages":[{"role":"user","content":"hello"}],"max_tokens":64}'
```

对贡献共享池的请求，仅纯文本 Chat Completions 被允许，并要求设置合理的 `max_tokens` 或 `max_completion_tokens`；工具、图片、音频等能力不能假定可用。

## 登录后的账户和共享池

以下是后端路由 `router/api-router.go` 中的主要端点，使用已登录网页会话鉴权（不是贡献者提供的上游密钥）。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/status` | 公共站点状态 |
| `GET` | `/api/shared/providers` | 可接受贡献的端点列表 |
| `GET` | `/api/shared/models` | 已启用的独立模型目录 |
| `GET` | `/api/shared/mine` | 本人的贡献记录 |
| `POST` | `/api/shared/contributions` | 新增贡献并验证可用模型 |
| `PUT` | `/api/shared/contributions/:id` | 暂停/恢复本人贡献（需提供 `enabled`、`daily_limit`） |
| `DELETE` | `/api/shared/contributions/:id` | 删除本人贡献 |
| `POST` | `/api/user/topup` | 使用兑换码兑换额度，JSON：`{"key":"YOUR_REDEMPTION_CODE"}` |
| `GET` / `POST` | `/api/token/` | 查看/创建平台 API Token |
| `GET` | `/api/user/sessions` | 查看本人登录会话 |
| `POST` | `/api/user/auth/refresh` | 轮换 Refresh Cookie |
| `POST` | `/api/user/auth/logout` | 登出并撤销会话 |

新增共享贡献请求体示例：

```json
{
  "provider_id": 1,
  "key": "YOUR_AUTHORIZED_UPSTREAM_DEVELOPER_KEY",
  "daily_limit": 10000,
  "models": ["YOUR_CONFIGURED_MODEL_ID"]
}
```

`daily_limit` 使用平台额度单位而非人民币金额；后端允许 `50` 至 `50000000`，并检查密钥格式及模型验证。验证会产生少量上游实际费用。新增请求会根据 `AIA_SHARED_SECRET` 加密密钥。更新贡献必须同时提供 `enabled` 和 `daily_limit`，预算限制相同。Root 赠送额度请求需要 `user_id`、`quota` 和不重复的 `reference`。与网页会话相关的接口不应直接将平台 API Token 当作会话 Cookie 使用。

## Root 管理接口

| 方法 | 路径 | 操作 |
| --- | --- | --- |
| `GET` / `POST` | `/api/shared/admin/models` | 列表、新建模型 |
| `PUT` | `/api/shared/admin/models/:id` | 修改模型设置 |
| `GET` / `POST` | `/api/shared/admin/providers` | 列表、新增上游端点 |
| `PUT` | `/api/shared/admin/providers/:id` | 修改上游端点状态 |
| `PUT` | `/api/shared/admin/providers/:id/models` | 绑定或停用模型关联 |
| `POST` | `/api/shared/admin/grants` | 创建幂等的管理员赠送额度 |

API 定义可参见 [管理端 OpenAPI](openapi/api.json) 与 [模型端 OpenAPI](openapi/relay.json)，用户使用说明见 [usage.md](usage.md)。不要将平台 Token、贡献密钥、OIDC client secret 或兑换码发布在 GitHub Issue 中。
