# ADR-001: Use Go for the Core Engine

## Status

Accepted

## Context

The core engine needs to manage Docker, expose APIs, verify sessions, run background jobs, manage backups, and deploy simply.

## Decision

Use Go for the backend control plane.

## Consequences

Positive:

- Small deployable binary/container.
- Good Docker SDK support.
- Good concurrency.
- Low memory usage.
- Strong fit for infrastructure tooling.

Negative:

- Frontend still needs separate web app.
- Some UI-heavy logic is better kept in TypeScript.
