# Backup and Restore

Backups are a v1 requirement.

The user should be able to trust Cloud Core with notes, files, app data, dashboard settings, and route configuration.

## Backup goals

Protect:

- app data
- uploaded files
- notes data
- bookmarks
- dashboard layout
- widgets
- app blueprints
- installed app records
- Caddy route data
- SQLite control database
- backup metadata

## Backup modes

### 1. Scheduled encrypted backup

Runs automatically.

### 2. Manual backup now

User clicks a button in the dashboard.

### 3. Safe Escape download

User downloads a full encrypted archive to their own device.

### 4. Restore

Fresh server can be restored from a backup archive.

## Safe Escape flow

```text
User clicks Safe Escape
  ↓
Cloud Core prepares backup plan
  ↓
Freeze/quiet relevant services if needed
  ↓
Copy SQLite, blueprints, configs, volumes
  ↓
Create archive
  ↓
Encrypt archive
  ↓
Browser downloads backup file
```

## Restore success condition

A successful restore means:

```text
Fresh Oracle VM
  ↓
Install Cloud Core
  ↓
Upload/provide backup archive
  ↓
Restore data/config
  ↓
Restart services
  ↓
Apps and dashboard return
```

## Backup metadata

Each backup should record:

- backup ID
- created time
- included apps
- included paths
- size
- encryption status
- checksum
- restore compatibility version

## Restore tests

Cloud Core should eventually support:

```bash
cloud-core backup verify <backup-file>
cloud-core restore --dry-run <backup-file>
```
