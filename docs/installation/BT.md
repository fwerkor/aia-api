# 使用宝塔面板部署 AIA API

**不要从宝塔应用商店安装「New API」代替 AIA API**：两者不是同一个构建产物。本指南使用宝塔的 Docker Compose 功能和 `ghcr.io/fwerkor/aia-api` 镜像。

1. 准备 Docker/Compose、受保护的数据库存储、反向代理和可申请 OIDC 客户端的 AIA 论坛管理员权限。
2. 使用宝塔终端克隆仓库：`git clone https://github.com/fwerkor/aia-api.git`，进入 `aia-api` 目录。
3. 生成 `openssl rand -hex 32`，将输出保存到**私有** `.env` 的 `AIA_SHARED_SECRET`；不要在此页面公开该值。
4. 编辑 `docker-compose.yml`，替换 PostgreSQL/Redis 示例密码及 SQL_DSN，确保数据库、缓存不能直接对公网访问。确认镜像为 `ghcr.io/fwerkor/aia-api:latest`。
5. 在宝塔的 Compose 管理功能中使用仓库 Compose 文件启动，或在终端执行 `docker compose up -d`。
6. 配置站点 HTTPS 反向代理到本机 `127.0.0.1:3000`，验证 `/api/status`，再设置论坛 OIDC（回调 `https://YOUR_DOMAIN/oauth/oidc`）。
7. 登录测试、兑换码和共享池验证成功后再正式开放，并建立数据库备份及密钥保管流程。

详情见 [AIA API 部署指南](../deployment.md) 与 [认证安全](../authentication.md)。不支持现金充值、密码注册、New API 商业购买或使用上游 New API 的应用商店配置自动替代这些步骤。
