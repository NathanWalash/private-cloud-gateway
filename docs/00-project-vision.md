# Project Vision

Private Cloud Gateway is a private personal cloud dashboard and app orchestrator.

It gives one owner a beautiful, secure, customisable home for self-hosted tools, app links, server health, API health, and backups.

> The internal Go backend service is called the **Cloud Core** engine. This name appears in env vars and service references throughout the codebase.

## Product definition

Private Cloud Gateway is:

> A private, login-protected dashboard and Docker app orchestrator for running self-hosted apps on a single Oracle Cloud VM using custom-domain subdomain routing, central authentication, app blueprints, status widgets, and encrypted backup/restore.

## End-user promise

```text
I open one private dashboard and instantly know what is running,
what is healthy, what needs attention, and where to go next.
Everything important is protected behind my login and backed up.
```

## It is

- A private cloud homepage.
- A secure login gate.
- A dashboard and launcher.
- A Docker app manager.
- A custom subdomain router.
- A server/app/API status surface.
- A backup and restore centre.
- A developer-customisable personal control panel.

## It is not

- A public SaaS product.
- A Kubernetes platform.
- An AI agent platform.
- A general enterprise hosting provider.
- A replacement for the apps it hosts.
- An iframe-first app workspace.

## Primary target user

A developer or technical user who wants to run private web tools on their own domain without manually managing reverse proxies, Docker networking, app lifecycle, and backups every time.

## Primary hosting target

Oracle Cloud Infrastructure, using a single Ubuntu VM.

## Design priorities

1. Security and access control.
2. Backup and restore.
3. Simple local development.
4. One-command production install.
5. Beautiful customisable dashboard.
6. Reliable Docker app management.
7. Clear documentation.
