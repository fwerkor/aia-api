# AIA Shared API Pool

This repository contains the AIA API customizations to [New API](https://github.com/QuantumNous/new-api), including a contributed API-key pool. The upstream New API code and this derivative are released under **AGPL-3.0-or-later** (see `LICENSE`, copyright headers and notices). AIA does not claim authorship of upstream work.

## What it does

- An administrator creates HTTPS endpoint/model templates with a verified per-million-token price and maximum output cap. Newly added templates start disabled. **Only permit models/providers whose contract expressly allows the proposed proxy/sharing use**; working credentials alone do not indicate permission.
- Users sign in through AIA Forum OIDC (native password sign-up/login disabled), submit their own upstream credentials, and have them automatically verified with a minimal real completion. Credentials use AES-256-GCM encryption at rest and keyed SHA-256 fingerprints to prevent duplicates; other users and ordinary account APIs never receive them.
- Per-contribution daily budgets in native quota units are pre-reserved inside a database transaction. The shared contributions join native New API channels and the reward is credited to the original users.quota wallet after native settlement.
- All clients use AIA API-issued platform tokens, the compatible `/v1` API URL and inherited `users.quota` wallet. The native accounting pipeline deducts consumer quota and writes native usage logs; a successful native settlement atomically awards the corresponding quota to the contributor. An idempotent grant is available to root administrators.
- Shared channels participate in the original New API channel scheduler and `/v1/chat/completions`, alongside administrator channels. Only plain-text chat with an explicit bounded `max_tokens` or `max_completion_tokens` is allowed through contributed keys; ordinary channels remain unchanged. Streaming settlement follows native New API usage accounting.
- Contributor and administrator can pause/disable channels. The existing New API upstream-failure and auto-ban workflow applies. Contributors can revoke saved ciphertext once no requests are active. Exhausted budgets and disabled providers are filtered from native channel selection.

## Automatic model-to-endpoint verification

The admission flag (`shared_providers.accept_contributions`) is separate from
the routing flag (`shared_providers.enabled`). The four built-in upstream templates are trusted routing candidates and
accept contributions by default. Native channels are only created after a
developer API key has passed model-specific validation. No channel exists
merely because an endpoint is configured. Disabling
contribution admission does **not** revoke previously approved routing.

When a user submits an API key, the server:

1. Selects a bounded set of at most eight enabled catalog models matching the
   endpoint protocol (or the user's explicitly selected models). Explicitly
   disabled model associations are excluded from automatic probes.
2. Attempts the upstream `GET /models` API with the supplied credential. An
   available listing only filters candidates; it is **not proof of access**.
   If listing is unavailable, the service falls back to direct model probes.
3. Makes a small, metered text request per candidate through the appropriate
   Chat Completions, Responses, or Anthropic Messages API, and checks both
   response content and usage records. Unverified models are not mapped.
4. Stores only successfully tested associations, encrypts the user key, and
   creates a standard New API channel exposing the verified models. Individual
   probe results are returned to the user; credentials are never included.
5. An administrator may pause any endpoint independently of contribution
   admission; its verified contributions are retained. A model without catalog
   and native billing prices is excluded from actual routing.

Each probe may consume the contributor's upstream API balance (at most eight
short completions per submission). The service cannot discover or test model
entitlements without a user-provided developer API key. OAuth subscriptions and
IDE usage allowances are not accepted as credentials.

## Independent model catalog

Models and endpoints are now separate resources:

- `shared_models`: globally unique API model code, display name, protocol, per-model reference input/output prices, maximum output, enabled state.
- `shared_providers`: upstream HTTPS base URL and protocol, with no required model or model pricing.
- `shared_provider_models`: explicit many-to-many mappings; multiple endpoints can provide the same model, and one endpoint can provide several models.
- `shared_contributions.models`: the set of models an owner explicitly selected for one encrypted key; validation runs once per selected model. Native New API channels expose all selected models in the existing `/v1` route and keep one combined daily contribution budget.

Admin API: `GET /api/shared/admin/models`, `POST /api/shared/admin/models`, `PUT /api/shared/admin/models/:id`, `PUT /api/shared/admin/providers/:id/models`. Signed-in users can browse `GET /api/shared/models` independently of whether any upstream is enabled.

Seeded model catalog: `deepseek-flash` (display: DeepSeek V4.1 Flash), `deepseek-v4-pro` (legacy compatibility), `gpt-6.1-sol`, `gpt-6-sol`, `gpt-6-luna`, `claude-sonnet-4-6`, `claude-opus-4-8`, `gemini-3.8-flash`, `gemini-3.6-flash`, `gemini-4-argon`, `kimi-k2.5`. Official DeepSeek documentation specifies `deepseek-flash` as the API ID for DeepSeek V4.1 Flash, including the October 17 update. Missing reference prices are kept at zero until explicitly checked. Catalogue presence is not a claim that the model is callable through any particular provider. Prior four endpoint/model links migrate into the join table without generating credentials or spending usage.

Sources: [DeepSeek official API guide](https://api-docs.deepseek.com/zh-cn/), [DeepSeek updates](https://api-docs.deepseek.com/zh-cn/updates/), [OpenAI model list](https://developers.openai.com/api/docs/models), [Claude model IDs](https://platform.claude.com/docs/en/about-claude/models/model-ids-and-versions), [Gemini models](https://ai.google.dev/gemini-api/docs/models).

## Built-in upstream API templates

The following four provider templates are inserted idempotently into the administrator list. Their original model associations are migrated to the independent model catalog. **All start enabled as potential upstreams**, but no consumer channel is available until a validated key has been contributed. These are official developer **API-key** products, not gateways into subscription/session/OAuth usage:

| Label | Developer API base | Verification protocol | Example model |
|---|---|---|---|
| OpenCode Zen | `https://opencode.ai/zen/v1` | OpenAI Chat Completions | `kimi-k2.5` |
| Codex-related OpenAI API | `https://api.openai.com/v1` | OpenAI Responses | `gpt-6-sol` |
| Claude Code-related Claude API | `https://api.anthropic.com/v1` | Anthropic Messages | `claude-sonnet-4-6` |
| Antigravity-related Gemini API | `https://generativelanguage.googleapis.com/v1beta/openai` | Gemini-compatible Chat Completions | `gemini-3.8-flash` |

Model availability, billing and provider terms vary. Confirm terms explicitly permit proxy sharing, set native New API model pricing, and verify provider input/output pricing before enabling. OpenCode Zen is created with **unset** prices; the other example prices are reference estimates that must be checked again at activation. Google Gemini 3.8 Flash introductory pricing is scheduled to increase after December 31, 2026.

**These developer API credentials do not consume existing Codex/ChatGPT, Claude Code Pro/Max, or Antigravity IDE subscription quota.** Never submit browser cookies, IDE refresh tokens, Google sign-in tokens or Claude Code OAuth credentials into this API-key contribution form.

References: [OpenCode Zen](https://docs.opencode.ai/docs/zen/), [OpenAI API models](https://developers.openai.com/api/docs/models/gpt-6-sol), [Claude model API](https://platform.claude.com/docs/en/models/sonnet-4-6/overview), [Gemini OpenAI compatibility](https://ai.google.dev/gemini-api/docs/openai).

## Deployment

The inherited Go/React backend/frontend toolchain is still required. To build locally:

```bash
cd web
bun install --frozen-lockfile
bun --bun run typecheck
bun --bun run build
cd ..
go test ./...
CGO_ENABLED=0 go build -o aia-api .
```

Set the `AIA_SHARED_SECRET` environment variable **only on the server** to a persistent random 32-byte key represented as 64 hex characters:

```bash
openssl rand -hex 32
```

Do not commit the output or put it in Compose YAML, CI logs, or source. Store it in the server's secret management or mode-0600 `.env` file. The Compose file accepts `AIA_SHARED_SECRET` from the deployment environment. Never rotate the secret without first migrating/encrypting saved credentials: an old ciphertext cannot be read under a different key. Without the setting, sharing endpoints remain unavailable, but the ordinary New API service still starts.

The project migrations create `shared_providers`, `shared_contributions` and `shared_charges` for endpoint allowlisting, encrypted credentials and idempotent budget/reward records; the old `shared_accounts` table is not used by this version. Back up the DB and verify upgrade/rollback before production changes.

After login, open `/shared-pool` for contribution controls. Root administrators can add, enable, and disable provider templates and issue small bootstrap grants. For shared API calls:

```python
from openai import OpenAI

client = OpenAI(
    api_key="YOUR_AIA_API_TOKEN",
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
- The common wallet uses the inherited native quota units (nominally 500,000 quota = $1). Administrators must configure a **positive native model price or ratio** before enabling shared endpoints. The specified upstream input/output prices are used only for conservative contribution-budget reservation, not as a separate consumer pricing engine. Actual rewards follow New API's settled consumption quota; matching the upstream's invoice must be checked by the operator.
- Failed relay attempts release the contribution-budget reservation. Native successful settlement credits the contributor wallet once per unique attempt ID; budget reservations are transactional and bounded. A native charge exceeding the reserved bound is capped for contributor rewards and should trigger administrator review of model pricing; upstream invoices are independent of proxy quota. External provider changes and server failures still require operator reconciliation.
- Only root administrators may grant native wallet quota using an idempotent reference. Ordinary users cannot mint credits or withdraw cash. Five previously active subscriptions and old wallet balances are deliberately preserved; the old subscription preference logic remains compatible.

## AIA customizations

The existing AIA concurrent-active-subscription purchase-limit fix is retained in this source snapshot (`model/subscription.go`, front-end subscription forms and regression tests). Deployment secrets, production databases, credentials and infrastructure-specific overrides are deliberately excluded from the repository.

## License and upstream attribution

Based on the New API project at https://github.com/QuantumNous/new-api (base revision `0ab02020603d22e5613bc4cf46bfab06f8567769`). This derivative retains original AGPL notices and third-party credits. `LICENSE` governs redistribution and network deployment source availability.

## AIA-only simplified deployment (October 2026)

- Account creation and login are through the configured **AIA Forum OIDC** provider. The native password/sign-up endpoints and pages are disabled; forum first-login account provisioning remains available.
- Commercial top-ups, online payment gateways/webhooks, invite kickbacks, daily check-ins and commercial subscription purchases are removed. Existing subscription usage rights and historic database records remain intact.
- **Redemption codes remain supported** through the existing `POST /api/user/topup` redemption handler and the `/wallet` balance/redeem page.
- The 11 model IDs in the shared catalogue are the AIA API default model pricing set. Display names are not required. Prices are reference rounded **CNY per million input/output tokens**, with the native 500,000 quota-unit denominator preserved for existing accounts. Default accounting uses a nominal 7 CNY per USD historical unit.
- Quota exhaustion or 429 responses temporarily cool down a contributed channel for 5–30 minutes. It is automatically eligible again after the cooldown; the credential is not permanently disabled on transient lack of quota.
- [OpenCode Go](https://docs.opencode.ai/docs/go/) is the confirmed official subscription endpoint at `https://opencode.ai/zen/go/v1`, using the user's Go plan API key. The catalogue `deepseek-flash` model maps to OpenCode Go's `deepseek-v4.1-flash` model ID. No browser cookies or IDE refresh tokens are accepted.
- New API separately has a built-in Codex OAuth channel adapter. Linking third-party subscription/OAuth accounts to shared-key contribution requires a provider-supported authorization flow; an ordinary OpenAI API endpoint is not equivalent to Codex, Claude Code, or Antigravity subscriptions. Do not treat those API endpoints as subscription usage.
