# Caddy Infrastructure

Caddy is the public HTTPS gateway.

It should:

- terminate HTTPS
- route subdomains
- call Cloud Core auth verification
- proxy only to private Docker services

Caddy admin/API access must not be public.
