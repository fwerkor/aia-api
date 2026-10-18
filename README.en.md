# AIA API

[简体中文](README.zh_CN.md) · [繁體中文](README.zh_TW.md) · **English** · [Français](README.fr.md) · [日本語](README.ja.md)

AIA API is a self-hosted, OpenAI-compatible model gateway and **contributed API-key pool** for the NJU student AI community. Members sign in with **AIA Forum OIDC**, use a shared credit balance, redeem credit codes, and optionally contribute permitted upstream developer API keys. Administrators manage approved endpoints, models, routing, and metering.

> **Project scope:** This repository is an AIA-specific derivative of [New API](https://github.com/QuantumNous/new-api), licensed under AGPL-3.0-or-later. It is **not** the upstream New API project. See [LICENSE](LICENSE), [NOTICE](NOTICE) and [THIRD-PARTY-LICENSES.md](THIRD-PARTY-LICENSES.md) for attribution.

## Features

- **Single sign-on:** AIA Forum OIDC; native password sign-up and password login are disabled. An authorized forum account can be provisioned at first login.
- **Unified API access:** platform-issued tokens for the OpenAI-compatible `/v1` gateway. Model availability depends on verified contributions and administrator-enabled upstream channels.
- **Bring-your-own-key pool:** users submit supported developer API keys, choose models and a daily budget; the server probes access, encrypts keys, and credits contributors for successfully settled shared requests.
- **Independent model catalog:** model IDs and provider endpoints are administered separately; routing only uses enabled, verified, correctly priced model associations.
- **Credit accounting:** consumption is metered in the platform's quota ledger, with reference prices shown in CNY. **Redemption codes are supported; cash top-ups, payment gateways and commercial subscription purchases are not.** Existing subscription entitlements may still be managed by administrators.
- **Transient upstream exhaustion:** rate-limit and temporary quota failures cool a contributed channel down rather than permanently disabling its key.

## Use the hosted service

1. Open the AIA API web interface at the address provided by your administrator (the AIA deployment uses `https://api.nju-aia.com`).
2. Sign in through **AIA Forum**, then create a platform API token in the token-management page.
3. Inspect available model IDs in the model list or via `GET /v1/models`. The model catalog is **not** proof that a model has an active usable upstream.
4. Use the token with the OpenAI-compatible base URL:

```bash
curl https://api.nju-aia.com/v1/chat/completions \
  -H 'Authorization: Bearer YOUR_AIA_API_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{"model":"YOUR_AVAILABLE_MODEL_ID","messages":[{"role":"user","content":"Hello"}],"max_tokens":128}'
```

Use `/wallet` to view credits and redeem a code. Use `/shared-pool` to contribute an **authorized developer API key** with an explicit daily limit. Never upload an IDE session token, browser cookie, or subscription OAuth credential.

## Self-hosting

```bash
git clone https://github.com/fwerkor/aia-api.git
cd aia-api
# Generate a secret and place it only in a private .env file or secrets manager:
openssl rand -hex 32
# Set AIA_SHARED_SECRET to that value; configure database, Redis and AIA Forum OIDC.
docker compose up -d
```

The checked-in [`docker-compose.yml`](docker-compose.yml) pulls `ghcr.io/fwerkor/aia-api:latest` and includes **example database/Redis passwords that must be changed**. Do **not** expose an instance using those defaults. Production needs HTTPS and a correctly registered Forum OIDC redirect URI (`https://YOUR_DOMAIN/oauth/oidc`). The `AIA_SHARED_SECRET` is a persistent 32-byte key encoded as 64 hex characters; without it, contributed-key sharing is unavailable. Back up the database and this encryption key before upgrades. No actual forum credentials or production secrets are included in this repository.

For a local frontend/backend build:

```bash
cd web
bun install --frozen-lockfile
bun run build
cd ..
go test ./...
CGO_ENABLED=0 go build -o aia-api .
```

## Documentation

- [Documentation index](docs/README.md)
- [Using tokens, models, credits and the shared pool](docs/usage.md)
- [Administrator setup, OIDC and deployment](docs/deployment.md)
- [Administrator operations](docs/admin.md)
- [HTTP API reference](docs/api.md) · [OpenAPI: management](docs/openapi/api.json) · [OpenAPI: model relay](docs/openapi/relay.json)
- [Session security and authentication details](docs/authentication.md)
- [Shared-pool design, threat model and upstream attribution](AIA-SHARED-POOL.md)

## Development and support

Please file AIA API issues and pull requests in **[fwerkor/aia-api](https://github.com/fwerkor/aia-api/issues)**. For security vulnerabilities, use [private security advisories](https://github.com/fwerkor/aia-api/security/advisories/new), not a public issue. See [security policy](.github/SECURITY.md).

AIA API is released under [AGPL-3.0-or-later](LICENSE). Source distributions and network deployments must satisfy the applicable license terms. Upstream New API documentation describes a **different, broader product** and may not match this fork. Supported upstream credential types and pricing must also comply with each provider's terms; subscription allowances are not automatically transferable to developer APIs.
