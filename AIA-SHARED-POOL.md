# AIA Shared API Pool

This repository contains the AIA API customizations to [New API](https://github.com/QuantumNous/new-api), including a contributed API-key pool. The upstream New API code and this derivative are released under **AGPL-3.0-or-later** (see `LICENSE`, copyright headers and notices). AIA does not claim authorship of upstream work.

## What it does

- An administrator creates HTTPS endpoint/model templates with a verified per-million-token price and maximum output cap. Newly added templates start disabled. **Only permit models/providers whose contract expressly allows the proposed proxy/sharing use**; working credentials alone do not indicate permission.
- Users sign in through the existing New API account system, submit their own upstream credentials, and have them automatically verified with a minimal real completion. Credentials use AES-256-GCM encryption at rest and keyed SHA-256 fingerprints to prevent duplicates; other users and ordinary account APIs never receive them.
- Per-contribution daily USD-equivalent budgets are pre-reserved inside a database transaction. The shared contributions join native New API channels and the reward is credited to the original users.quota wallet after native settlement.
- All clients retain the existing New API-issued token, original `/v1` API URL and `users.quota` wallet. New API alone deducts consumer quota and writes native usage logs; a successful native settlement atomically awards the corresponding quota to the contributor. An idempotent grant is available to root administrators.
- Shared channels participate in the original New API channel scheduler and `/v1/chat/completions`, alongside administrator channels. Only plain-text chat with an explicit bounded `max_tokens` or `max_completion_tokens` is allowed through contributed keys; ordinary channels remain unchanged. Streaming settlement follows native New API usage accounting.
- Contributor and administrator can pause/disable channels. The existing New API upstream-failure and auto-ban workflow applies. Contributors can revoke saved ciphertext once no requests are active. Exhausted budgets and disabled providers are filtered from native channel selection.

## Deployment

The original New API backend/frontend dependencies are still required. To build locally:

```bash
cd web
bun install --frozen-lockfile
bun --bun run typecheck
bun --bun run build
cd ..
go test ./...
CGO_ENABLED=0 go build -o new-api .
```

Set the `AIA_SHARED_SECRET` environment variable **only on the server** to a persistent random 32-byte key represented as 64 hex characters:

```bash
openssl rand -hex 32
```

Do not commit the output or put it in Compose YAML, CI logs, or source. Store it in the server's secret management or mode-0600 `.env` file. The Compose file accepts `AIA_SHARED_SECRET` from the deployment environment. Never rotate the secret without first migrating/encrypting saved credentials: an old ciphertext cannot be read under a different key. Without the setting, sharing endpoints remain unavailable, but the ordinary New API service still starts.

Existing New API migrations create `shared_providers`, `shared_contributions` and `shared_charges` for endpoint allowlisting, encrypted credentials and idempotent budget/reward records; the old `shared_accounts` table is not used by this version. Back up the DB and verify upgrade/rollback before production changes.

After login, open `/shared-pool` for contribution controls. Root administrators can add, enable, and disable provider templates and issue small bootstrap grants. For shared API calls:

```python
from openai import OpenAI

client = OpenAI(
    api_key="YOUR_PLATFORM_NEW_API_TOKEN",
    base_url="https://api.nju-aia.com/v1",
)
print(client.chat.completions.create(
    model="MODEL_CONFIGURED_BY_AIA",
    messages=[{"role": "user", "content": "Hello"}],
    max_tokens=128,
))
```

## Security and accounting boundaries

- Endpoints are administrator-defined, HTTPS-only and subject to public-IP DNS checks to reduce SSRF risk. Redirects are not followed. Upstream traffic can expose request content to a contributor's provider; never send sensitive content through untrusted providers.
- Shares must be explicitly opted into, with a daily budget. Tokens are limited to text chat, bounded input, bounded max output, the model selected by administrators, and user platform-token model/IP restrictions.
- The common wallet uses native New API quota units (nominally 500,000 quota = $1). Administrators must configure a **positive native model price or ratio** before enabling shared endpoints. The specified upstream input/output prices are used only for conservative contribution-budget reservation, not as a separate consumer pricing engine. Actual rewards follow New API's settled consumption quota; matching the upstream's invoice must be checked by the operator.
- Failed relay attempts release the contribution-budget reservation. Native successful settlement credits the contributor wallet once per unique attempt ID; budget reservations are transactional and bounded. A native charge exceeding the reserved bound is capped for contributor rewards and should trigger administrator review of model pricing; upstream invoices are independent of proxy quota. External provider changes and server failures still require operator reconciliation.
- Only root administrators may grant native wallet quota using an idempotent reference. Ordinary users cannot mint credits or withdraw cash. Five previously active subscriptions and old wallet balances are deliberately preserved; the old subscription preference logic remains compatible.

## AIA customizations

The existing AIA concurrent-active-subscription purchase-limit fix is retained in this source snapshot (`model/subscription.go`, front-end subscription forms and regression tests). Deployment secrets, production databases, credentials and infrastructure-specific overrides are deliberately excluded from the repository.

## License and upstream attribution

Based on the New API project at https://github.com/QuantumNous/new-api (base revision `0ab02020603d22e5613bc4cf46bfab06f8567769`). This derivative retains original AGPL notices and third-party credits. `LICENSE` governs redistribution and network deployment source availability.
