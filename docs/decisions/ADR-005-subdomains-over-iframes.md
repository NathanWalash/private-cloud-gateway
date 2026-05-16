# ADR-005: Use Protected Subdomains Over Iframes

## Status

Accepted

## Context

Early ideas included embedding apps inside a dashboard frame. Many apps do not support this reliably, and browser security policies can block it.

## Decision

The default app access model is protected subdomains.

Example:

```text
files.example.com
pdf.example.com
bookmarks.example.com
```

## Consequences

Positive:

- Apps run naturally.
- Better compatibility.
- Easier routing model.
- Still protected by central login.

Negative:

- The user may leave the dashboard page when opening an app.
- Optional inline mode can be added later for compatible apps.
