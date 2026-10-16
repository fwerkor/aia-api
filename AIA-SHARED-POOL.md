# AIA Shared API Pool

This repository contains the AIA API customizations to [New API](https://github.com/QuantumNous/new-api), including a contributed API-key pool. The upstream New API code and this derivative are released under **AGPL-3.0-or-later** (see `LICENSE`, copyright headers and notices). AIA does not claim authorship of upstream work.

## What it does

- An administrator creates HTTPS endpoint/model templates with a verified per-million-token price and maximum output cap. Newly added templates start disabled. **Only permit models/providers whose contract expressly allows the proposed proxy/sharing use**; working credentials alone do not indicate permission.
- Users sign in through the existing New API account system, submit their own upstream credentials, and have them automatically verified with a minimal real completion. Credentials use AES-256-GCM encryption at rest and keyed SHA-256 fingerprints to prevent duplicates; other users and ordinary account APIs never receive them.
- Per-contribution daily USD-equivalent budgets are pre-reserved inside a database transaction. The service uses a separate shared-credit account instead of existing paid wallet/plan quotas.
- A user may consume others' keys when they have shared credits; using one's own key requires no shared-credit balance. Actual audited upstream usage transfers shared credits from consumer to contributor. Grants require a privileged administrator and unique idempotency reference.
- Text-only OpenAI-compatible `chat.completions` is available at `/shared/v1/chat/completions`. Model discovery is available at `/shared/v1/models`. Use your ordinary New API-issued platform token. Streaming relies on upstream usage reporting; a stream with missing usage is conservatively charged at the maximum reservation. Self-usage has no transferred credits.
- Credentials are automatically paused on upstream HTTP 401/403 and cooled down for 60 seconds on 429. The contributor may pause or revoke them immediately from the user dashboard. On revocation the saved ciphertext is destroyed after active requests settle.

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

Existing New API database migrations create separate `shared_providers`, `shared_contributions`, `shared_accounts` and `shared_charges` tables on startup. Back up the DB and verify upgrade/rollback before production changes.

After login, open `/shared-pool` for contribution controls. Root administrators can add, enable, and disable provider templates and issue small bootstrap grants. For shared API calls:

```python
from openai import OpenAI

client = OpenAI(
    api_key="YOUR_PLATFORM_NEW_API_TOKEN",
    base_url="https://api.nju-aia.com/shared/v1",
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
- Credits are represented as integer micro-USD units for accounting; they are **not cash, transferable money, or redeemable against paid plans**. Pricing is the administrator's stated proxy unit cost, not proof of provider invoices. No guarantee that usage pricing exactly matches the upstream's total financial bill.
- An upstream failure without usage generally refunds a reservation; an already-accepted stream with missing usage is billed up to the reservation. Concurrent reservations count toward the daily budget; settling is idempotent through unique request identifiers. External provider changes and server failures still require operator reconciliation.
- The default release allows administrators to grant bootstrap shared credits. It does not allow ordinary users to mint or cash out credits.

## AIA customizations

The existing AIA concurrent-active-subscription purchase-limit fix is retained in this source snapshot (`model/subscription.go`, front-end subscription forms and regression tests). Deployment secrets, production databases, credentials and infrastructure-specific overrides are deliberately excluded from the repository.

## License and upstream attribution

Based on the New API project at https://github.com/QuantumNous/new-api (base revision `0ab02020603d22e5613bc4cf46bfab06f8567769`). This derivative retains original AGPL notices and third-party credits. `LICENSE` governs redistribution and network deployment source availability.
