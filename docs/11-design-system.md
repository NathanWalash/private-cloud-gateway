# Design System

## Product feel

Private Cloud Gateway should feel:

- private
- calm
- sharp
- premium
- developer-friendly
- trustworthy

It should not feel like a generic admin panel.

## Main UI surfaces

- Login page.
- Dashboard.
- App cards.
- Status widgets.
- Backup panel.
- Settings.
- Logs/activity.

## Dashboard widget types

Initial widgets:

- app shortcut
- server metric
- API status
- backup status
- custom link
- recent activity
- alert/warning

## Example widget config

```yaml
type: api_status
title: Portfolio API
endpoint: https://api.example.com/health
refresh_seconds: 60
```

## App card states

```text
Online
Sleeping
Starting
Offline
Warning
Error
```

## Theme customisation

Possible settings:

- light/dark mode
- accent colour
- background style
- dashboard layout
- compact/comfortable density
- custom logo
- custom greeting
