# Routing and Domains

## Production domain model

Example:

```text
nathan.me              -> Cloud Core dashboard/login
files.nathan.me        -> File Browser
pdf.nathan.me          -> Stirling PDF
bookmarks.nathan.me    -> Shiori
status.nathan.me       -> Uptime Kuma/status
```

## Recommended DNS

Use wildcard DNS if possible:

```text
A     nathan.me       -> ORACLE_SERVER_IP
A     *.nathan.me     -> ORACLE_SERVER_IP
```

This allows new app subdomains without manually adding DNS records per app.

## Routing rule

All protected subdomains use the same auth gate.

```text
*.nathan.me
  ↓
Caddy
  ↓
Cloud Core auth verification
  ↓
App container
```

## Local development domains

Use localhost subdomains where possible:

```text
home.localhost
files.localhost
pdf.localhost
bookmarks.localhost
```

Alternative:

```text
dashboard.local
files.local
pdf.local
```

These may require `/etc/hosts` entries.

## Route registration

When an app is installed:

1. Cloud Core reads blueprint.
2. Cloud Core creates app container.
3. Cloud Core registers the subdomain route.
4. Caddy proxies traffic to the private container.
5. Dashboard shows app as installed.
