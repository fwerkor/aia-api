# AIA API

[简体中文](README.zh_CN.md) · **繁體中文** · [English](README.md) · [Français](README.fr.md) · [日本語](README.ja.md)

AIA API 是面向南京大學學生人工智慧社群的自架模型 API 閘道與**使用者貢獻金鑰共享池**。使用者透過 AIA 論壇 OIDC 統一登入，使用平台額度呼叫模型，也可以在符合上游條款的前提下貢獻開發者 API Key。

本專案為 [New API](https://github.com/QuantumNous/new-api) 的 AIA 衍生版本，並非上游官方專案。授權條款為 [AGPL-3.0-or-later](LICENSE)，詳見 [NOTICE](NOTICE) 與 [第三方授權聲明](THIRD-PARTY-LICENSES.md)。

## 功能

- **論壇登入**：透過 AIA Forum OIDC 登入；不開放原生密碼註冊或登入。
- **統一模型 API**：以平台 Token 使用 OpenAI 相容 `/v1` 介面。可用模型取決於已驗證且啟用的上游。
- **共享 API Key**：貢獻合法取得的開發者金鑰、選擇模型與每日上限；平台驗證並加密保存，呼叫成功結算後獎勵貢獻者。
- **獨立模型目錄**：模型與上游端點分離管理；使用人民幣參考價格進行統一額度核算。
- **額度兌換**：保留兌換碼；不支援現金儲值、付款閘道或商業訂閱購買。暫時限流不會永久停用貢獻金鑰。

## 開始使用

1. 前往 `https://api.nju-aia.com` 或管理員提供的地址，以 AIA 論壇帳號登入。
2. 建立平台 API Token，查詢目前可用的模型 ID。
3. 將 OpenAI 相容用戶端的 Base URL 設為 `https://api.nju-aia.com/v1`，並使用該 Token 呼叫 `/chat/completions`。
4. 在 `/wallet` 兌換額度；在 `/shared-pool` 貢獻**允許分享的開發者 API Key**。請勿上傳訂閱 OAuth 憑證或瀏覽器 Cookie。

## 自行部署

```bash
git clone https://github.com/fwerkor/aia-api.git
cd aia-api
openssl rand -hex 32  # 安全儲存結果並設定為 AIA_SHARED_SECRET
docker compose up -d
```

容器映像為 `ghcr.io/fwerkor/aia-api:latest`。部署前請更改 Compose 範例的資料庫與 Redis 密碼，啟用 HTTPS，設定 AIA Forum OIDC（回呼 `https://你的網域/oauth/oidc`），並備份資料庫與固定加密密鑰。密鑰缺失時共享池無法使用。

## 文件與回報

[文件目錄](docs/README.md) · [使用指南](docs/usage.md) · [部署指南](docs/deployment.md) · [管理指南](docs/admin.md) · [API 參考](docs/api.md) · [共享池設計](AIA-SHARED-POOL.md)

問題請回報至 [AIA API Issues](https://github.com/fwerkor/aia-api/issues)；安全漏洞請使用[私人安全通報](https://github.com/fwerkor/aia-api/security/advisories/new)。
