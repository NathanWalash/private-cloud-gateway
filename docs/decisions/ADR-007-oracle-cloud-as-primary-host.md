# ADR-007: Oracle Cloud as Primary Host

## Status

Accepted

## Context

The project is intended to run on a low-cost or free-tier cloud VM with custom domain routing.

## Decision

Oracle Cloud Ubuntu VM is the primary production deployment target.

## Consequences

Positive:

- Good fit for always-on single-node hosting.
- ARM64 support.
- Can support Docker/Caddy/Go stack.

Negative:

- DNS setup is still required.
- Oracle Cloud account/VM creation is outside v1 automation.
