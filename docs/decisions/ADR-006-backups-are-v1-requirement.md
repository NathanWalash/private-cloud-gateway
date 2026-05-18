# ADR-006: Backups Are a v1 Requirement

## Status

Accepted

## Context

The system may store important notes, files, bookmarks, settings, and app data.

Data loss would make the project untrustworthy.

## Decision

Backup and restore are core v1 requirements.

## Consequences

Positive:

- Safer for real usage.
- Better trust.
- Easier migration/recovery.
- Supports Safe Escape feature.

Negative:

- More work before first production release.
