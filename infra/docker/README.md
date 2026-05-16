# Docker Infrastructure

Docker Compose is used for v1.

Rules:

- only Caddy exposes public ports
- apps use private network
- apps expose internal ports only
- app data is mounted under `/var/lib/cloud-core/apps`
