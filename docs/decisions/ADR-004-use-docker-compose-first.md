# ADR-004: Use Docker Compose First

## Status

Accepted

## Context

The project targets a single Oracle Cloud VM.

## Decision

Use Docker Compose for v1 app runtime and local development.

## Consequences

Positive:

- Simple.
- Familiar.
- Easy to run locally.
- Easy to install on one VM.

Negative:

- Not a multi-node scheduler.
- No Kubernetes-native reconciliation.
