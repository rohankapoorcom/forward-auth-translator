# Security

## Threat model

Authentication is enforced by **Authentik** (or your configured outpost). This translator only reformats headers on the forward-auth subrequest. Public source visibility does not weaken auth: valid Authentik app passwords or browser sessions are still required.

Attackers who can reach the translator without Traefik in front could send probe headers, but Authentik must accept the credentials. Invalid credentials receive upstream `401`/`403` — the translator does not grant access locally.

## Trust boundaries

- The service trusts the reverse proxy for `X-Forwarded-*` headers (`trustForwardHeader: true` on Traefik).
- Deploy on an internal network with NetworkPolicy restricting ingress to your Traefik namespace.
- `AUTHENTIK_OUTPOST_URL` is fixed at startup; client `Host` headers cannot redirect the proxy to arbitrary hosts.

## Credential handling

- Probe secrets appear only in request memory during proxying.
- Secrets are **never** logged, written to disk, or embedded in the container image.
- Operators rotate credentials in Authentik and their secret store; no git-managed passwords.

## Non-goals

This project does not provide:

- Rate limiting or WAF
- mTLS between components
- Per-application authorization (Authentik policies handle that)

Operators are responsible for network placement, TLS termination, and Traefik middleware chains that strip sensitive headers before backends.

## Reporting

Please report vulnerabilities via [GitHub private security advisories](https://github.com/rohankapoorcom/forward-auth-translator/security/advisories/new) on this repository.
