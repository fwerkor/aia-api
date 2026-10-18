# AIA API

**简体中文** · [繁體中文](README.zh_TW.md) · [English](README.md) · [Français](README.fr.md) · [日本語](README.ja.md)

AIA API 是面向南京大学学生人工智能社区的模型 API 网关及**用户贡献密钥共享池**。用户通过 **AIA 论坛 OIDC** 统一登录，使用平台额度调用模型，也可以在符合上游服务条款的前提下贡献开发者 API Key。管理员维护模型、服务端点、路由与计费。

> 本仓库是 [New API](https://github.com/QuantumNous/new-api) 的 AIA 定制衍生版本，并非 New API 官方仓库。遵循 AGPL-3.0-or-later；来源及第三方声明详见 [LICENSE](LICENSE)、[NOTICE](NOTICE)、[THIRD-PARTY-LICENSES.md](THIRD-PARTY-LICENSES.md)。

## 主要功能

- **论坛统一登录**：通过 AIA Forum OIDC 登录并自动创建授权用户；不提供原生密码登录和注册。
- **统一模型接口**：使用平台发放的 API Token 调用 `/v1` OpenAI 兼容接口；是否可用取决于管理员启用的渠道及实际验证结果。
- **贡献密钥共享池**：用户选择模型、填写合法取得的开发者 API Key、设定每日额度；平台验证访问能力，密文保存密钥，并在实际结算后向贡献者发放额度。
- **独立模型目录**：模型 ID 和服务端点分别管理，同一模型可绑定多个服务端点；未验证的绑定不能用于路由。
- **统一额度**：平台按模型价格和实际使用情况扣费，参考价格以人民币表示。保留**兑换码兑换额度**，不提供现金充值、支付网关及面向用户的商业订阅购买。
- **额度耗尽自动恢复**：限流和临时额度不足会触发冷却，而不是永久禁用贡献密钥。

## 使用方法

1. 打开管理员提供的 AIA API 站点（AIA 部署地址为 `https://api.nju-aia.com`），通过 AIA 论坛登录。
2. 在令牌管理页面生成平台 API Token，并查看当前可用的模型 ID。
3. 使用 OpenAI 兼容接口：

```bash
curl https://api.nju-aia.com/v1/chat/completions \
  -H 'Authorization: Bearer YOUR_AIA_API_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{"model":"YOUR_AVAILABLE_MODEL_ID","messages":[{"role":"user","content":"你好"}],"max_tokens":128}'
```

在 `/wallet` 页面查看额度或输入**兑换码**；在 `/shared-pool` 页面配置允许共享的**开发者 API Key**及每日预算。不得提交浏览器 Cookie、IDE OAuth Token、订阅账户凭据。模型目录中存在一个 ID 并不代表该模型当前存在可用的上游渠道。

## 自行部署

```bash
git clone https://github.com/fwerkor/aia-api.git
cd aia-api
openssl rand -hex 32   # 将结果安全保存在私有 .env 中，设置 AIA_SHARED_SECRET
docker compose up -d
```

Compose 默认从 `ghcr.io/fwerkor/aia-api:latest` 拉取镜像。仓库示例 Compose 包含**仅供演示的数据库和 Redis 密码**，生产环境必须更换，并配置 HTTPS、数据库备份以及 AIA 论坛 OIDC（重定向地址 `https://你的域名/oauth/oidc`）。`AIA_SHARED_SECRET` 必须为固定的 64 位十六进制密钥，改变后原有贡献密钥将无法解密；未设置时共享池功能不可用。

源码构建：在 `web/` 执行 `bun install --frozen-lockfile && bun run build`，然后返回根目录执行 `go test ./...` 和 `CGO_ENABLED=0 go build -o aia-api .`。

## 文档

- [文档总览](docs/README.md) · [用户指南](docs/usage.md) · [部署与登录配置](docs/deployment.md)
- [管理员指南](docs/admin.md) · [API 接口](docs/api.md) · [会话安全设计](docs/authentication.md)
- [共享池实现、计费和安全说明](AIA-SHARED-POOL.md)

## 反馈与许可证

请通过 [AIA API Issues](https://github.com/fwerkor/aia-api/issues) 反馈问题；安全漏洞请通过 [私密安全公告](https://github.com/fwerkor/aia-api/security/advisories/new) 报告。详见 [安全政策](.github/SECURITY.md)。本项目遵循 [AGPL-3.0-or-later](LICENSE) 并保留 New API 上游版权及第三方声明。上游 New API 的外部文档不代表 AIA API 当前的功能或部署方式。
