# User Experience

## Primary daily flow

```text
User visits nathan.me
  ↓
User sees clean login page
  ↓
User logs in
  ↓
Dashboard opens
  ↓
User sees apps, links, widgets, server health, backup status
  ↓
User clicks an app card
  ↓
App opens through a protected subdomain
  ↓
If app is sleeping, system wakes it before routing
```

## Key screens

### 1. Login page

Purpose:

- Brand the private cloud.
- Authenticate the owner.
- Protect dashboard and all apps.

Possible elements:

- Custom logo.
- Custom background.
- Password field.
- Optional MFA/passkey later.
- Status indicator.
- Light/dark theme.

### 2. Dashboard home

Purpose:

- Show everything important at a glance.

Widgets:

- App cards.
- Server CPU/RAM/disk.
- Backup status.
- API health.
- Recent activity.
- Custom links.
- Warnings/alerts.

### 3. Apps page

Purpose:

- Install, view, start, stop, restart, and remove apps.

### 4. App detail page

Purpose:

- Show app status, URL, health, logs, volumes, backup inclusion, lifecycle policy.

### 5. Backups page

Purpose:

- Show backup health and allow manual backup, Safe Escape download, and restore.

### 6. Settings page

Purpose:

- Domain settings.
- Theme settings.
- Backup settings.
- Admin account settings.
- Security settings.

## Access model

The login page protects all private apps.

Example:

```text
files.nathan.me
pdf.nathan.me
bookmarks.nathan.me
status.nathan.me
```

If the user is not logged in, the app redirects to login.

## App opening model

Default:

```text
Dashboard card -> protected app subdomain
```

Optional future behaviour:

```text
Dashboard card -> inline view if app supports safe embedding
```

Inline embedding is not a v1 requirement.
