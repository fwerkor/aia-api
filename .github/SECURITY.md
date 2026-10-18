# AIA API Security Policy

## Scope

Security reports for the **AIA API derivative** should go to this repository, not to New API upstream. Supported source is the latest release and current `main`; older versions should be upgraded before evaluation. Operational configuration and third-party provider vulnerabilities may require coordination with the deployment operator or provider.

## Private vulnerability reporting

**Do not create a public issue containing exploit details, API keys, user data or server secrets.** Use GitHub's [Report a vulnerability](https://github.com/fwerkor/aia-api/security/advisories/new) private advisory workflow. Include a sanitized reproduction, affected version/commit, security impact, and recommended mitigation. The maintainers will triage and coordinate disclosure; no fixed response-time SLA is promised.

If the affected code is inherited from [New API](https://github.com/QuantumNous/new-api), please state that so the maintainers can coordinate an upstream fix where appropriate. Do not assume reporting to the upstream project reports the issue to AIA API as well.

## Operational precautions

- Use HTTPS, a secure `SESSION_SECRET`, and correctly scoped Forum OIDC redirect/trusted-origin settings.
- Keep `AIA_SHARED_SECRET` (64 hexadecimal characters) and OIDC secrets outside version control; encrypt backups and plan key rotation carefully.
- Never share contributed API keys, platform Tokens or redeem codes in bug reports; redact logs.
- Change all Compose example database/Redis credentials, restrict management ports and maintain tested backups.
- Permit only provider keys and model endpoints explicitly authorized for sharing/proxy use.

See [deployment](../docs/deployment.md), [authentication](../docs/authentication.md) and [shared pool security boundaries](../AIA-SHARED-POOL.md).
