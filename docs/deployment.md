# AIA API 部署和登录配置

本指南针对 **fwerkor/aia-api** 的自部署，不适用于上游 New API 应用商店镜像。镜像发布在 `ghcr.io/fwerkor/aia-api`，最新版本标签和发布状态以本仓库的 [Releases](https://github.com/fwerkor/aia-api/releases) 和 [Packages](https://github.com/fwerkor?tab=packages) 为准。

## 1. 先决条件

- Docker Engine + Compose，或能编译 Go、Bun 前端的 Linux 主机
- 受保护的 PostgreSQL / MySQL / SQLite 持久化存储，按部署规模选用 Redis
- 可以配置客户端和回调地址的 **AIA Forum OIDC** 提供方
- 对外 HTTPS 域名、反向代理及数据库备份方案

## 2. 使用本仓库 Compose

```bash
git clone https://github.com/fwerkor/aia-api.git
cd aia-api
openssl rand -hex 32
```

把生成的随机值作为 `AIA_SHARED_SECRET` 保存到不入库的 `.env`（权限 0600）或服务器密钥管理服务；不要把值写进 Issue、日志或仓库。**部署前必须修改** `docker-compose.yml` 内置的数据库/Redis 示例密码、SQL_DSN 和对应服务配置：仓库默认值仅供本地演示。使用原有数据库升级前先备份，避免误覆盖生产数据。

```bash
docker compose config    # 检查变量和服务配置；不要在共享日志中泄露密码
docker compose up -d
docker compose ps
curl -fsS http://127.0.0.1:3000/api/status
```

默认容器在本机 `3000` 端口提供 HTTP，由反向代理对外提供 HTTPS。当前 Compose 服务仍叫 `new-api`，这是历史内部服务名；**实际发布镜像为 AIA API 的 GHCR 镜像**，不要把上游 `calciumion/new-api` 当成同一个产品。请将数据库和 Redis 限制在私有网络；正式使用前务必更换所有默认密码。

## 3. 配置论坛 OIDC

AIA API 的 OAuth 网页回调地址由站点的 `ServerAddress` 组成：`https://YOUR_DOMAIN/oauth/oidc`。在论坛的 OIDC 客户端中添加完全一致的 HTTPS redirect URI。管理员在站点系统配置中启用 OIDC，并配置 `client_id`、`client_secret`、`well_known` 或服务商提供的 authorization/token/userinfo endpoints，显示名称可设为 `AIA Forum`。不要在公开配置示例中放置真实 client secret。

登录入口走 `/api/oauth/oidc` 与网页 `/oauth/oidc` 回调。确保论坛 OIDC 声明能够提供稳定的 `sub` 和用户 `email`；OIDC 首次登录应根据管理员策略正确建立站点账户。原生邮箱/密码注册和密码登录在 AIA API 中不可用；管理员应提前验证 OIDC 登录流程再开放站点。

生产环境建议配置：`SESSION_SECRET` 为不可预测的固定值、`SESSION_COOKIE_SECURE=true`，并设置与实际站点精确匹配的 `SESSION_COOKIE_TRUSTED_URL=https://YOUR_DOMAIN`。前端及后端经 HTTPS 正确传递协议/Host。更多关于 Refresh Cookie 与会话的说明见 [authentication.md](authentication.md)。

## 4. 共享池加密与计费

- `AIA_SHARED_SECRET`：32 字节随机密钥的 **64 位十六进制** 表示；生产环境必须设置并安全备份，不能随意轮换。轮换前必须处理所有现有密文，否则既有贡献密钥无法解密。
- 后端数据库存储模型目录、服务端点、贡献、验证关联、平台额度与结算日志；配置与数据库需要一致备份。
- 模型默认目录不等于已验证上游；仅启用符合使用条款、协议、价格和合法访问要求的服务端点及模型。
- 配置新增模型实际价格并进行小额端到端验证，确认实际扣费及贡献者返还。

## 5. 更新与故障排查

升级前备份数据库、数据卷和秘密密钥，记录当前镜像 digest。审阅 release notes 后执行 `docker compose pull && docker compose up -d`，随后检查 `/api/status`、OIDC 登录、`/v1/models`、一次小额模型调用和兑换码流程。敏感凭据不可出现在诊断信息中。

本仓库也提供构建方式：`cd web && bun install --frozen-lockfile && bun run build`，然后 `go test ./...`、`CGO_ENABLED=0 go build -o aia-api .`。代码的 Go 模块路径仍保留上游内部命名，属兼容实现细节。
