# Docker and App Blueprints

## Purpose

Blueprints make apps installable without hard-coding every app into the core backend.

A blueprint tells Cloud Core:

- what image to run
- what subdomain to use
- what port the app exposes internally
- what volumes it needs
- whether it should be backed up
- whether it can sleep
- how to health check it

## Example blueprint

```yaml
id: stirling-pdf
name: PDF Tools
description: Merge, split, compress and convert PDFs
icon: file-text
category: utilities

route:
  subdomain: pdf
  internal_port: 8080

container:
  image: frooodle/s-pdf:latest
  environment: []
  volumes: []

lifecycle:
  policy: scale-to-zero
  sleep_after_minutes: 15

health:
  path: /
  expected_status: 200
  timeout_seconds: 10

backup:
  enabled: false

resources:
  memory_limit: 512m
```

## Lifecycle policies

### always-on

Use for:

- notes sync
- file storage
- monitoring
- core services
- databases

### scale-to-zero

Use for:

- PDF utilities
- converters
- occasional tools
- admin-only tools

## Install flow

```text
User clicks Install
  ↓
Go Core reads blueprint
  ↓
Validate blueprint
  ↓
Create volumes
  ↓
Create/start container
  ↓
Connect to private Docker network
  ↓
Register Caddy route
  ↓
Add app card to dashboard
  ↓
Start health check
```

## Backup declaration

Blueprints should explicitly say what data must be backed up.

Example:

```yaml
backup:
  enabled: true
  paths:
    - /var/lib/cloud-core/apps/filebrowser/data
    - /var/lib/cloud-core/apps/filebrowser/config
```
