# AIA API 文档

本目录描述 **fwerkor/aia-api** 当前的用户体验、运行接口及部署方式，不是上游 New API 官方文档。代码仍保留部分上游的内部包名与兼容性 API，但 AIA 的功能范围以实际路由和配置为准。

| 文档 | 内容 |
| --- | --- |
| [用户指南](usage.md) | 论坛登录、API Token、模型列表、额度、兑换码、共享密钥 |
| [部署指南](deployment.md) | GHCR、Compose、Forum OIDC、密钥、数据持久化与更新 |
| [管理员指南](admin.md) | 上游端点、模型绑定、贡献审核、额度操作 |
| [API 参考](api.md) | 主要 HTTP 接口、鉴权边界和示例 |
| [认证安全](authentication.md) | 登录会话、Cookie、Token、安全属性的实现细节 |
| [渠道附加设置](channel/other_setting.md) | 管理员维护的原生渠道参数（非共享池端点） |
| [共享池实现说明](../AIA-SHARED-POOL.md) | 贡献预算、验证流程、计费与风险 |
| [OpenAPI](openapi/api.json) / [模型 API](openapi/relay.json) | AIA 维护的主要接口规格 |

站点示例：`https://api.nju-aia.com`。自部署者应替换为自己的 HTTPS 域名。问题请提交至 [AIA API Issues](https://github.com/fwerkor/aia-api/issues)，安全漏洞应通过 [GitHub 私密安全公告](https://github.com/fwerkor/aia-api/security/advisories/new) 提交。
