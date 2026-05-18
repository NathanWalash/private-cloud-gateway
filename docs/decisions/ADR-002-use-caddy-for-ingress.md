# ADR-002: Use Caddy for Ingress

## Status

Accepted

## Context

Cloud Core needs HTTPS, custom subdomain routing, reverse proxying, and auth verification before apps are reached.

## Decision

Use Caddy as the public HTTPS gateway.

## Consequences

Positive:

- Automatic HTTPS.
- Good reverse proxy support.
- Suitable for forward-auth pattern.
- Simple configuration.

Negative:

- Dynamic config must be managed carefully.
- Caddy admin/API access must never be public.
