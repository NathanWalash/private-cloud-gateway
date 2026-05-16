# ADR-003: Use SQLite for Control-Plane State

## Status

Accepted

## Context

Cloud Core is a single-node, single-owner system.

It needs to store sessions, app records, settings, widget layout, backup records, and audit events.

## Decision

Use SQLite in WAL mode for Cloud Core control-plane state.

## Consequences

Positive:

- No separate DB server needed.
- Simple backup.
- Low overhead.
- Good fit for single-node use.

Negative:

- Not intended for multi-node clustering.
- Hosted apps may still need their own data stores.
