---
description: Scaffold a new app blueprint following the project's conventions
---

Create a new app blueprint at `blueprints/<id>.yaml` for: **$ARGUMENTS**

Follow these rules (all enforced by `TestBundledBlueprintsAreValid`):

1. Find the app's official Docker image and **pin it to a specific current
   version** — never `:latest` or a rolling tag like `stable`/`release`. Verify
   the tag actually exists by querying the registry before using it.
2. Use the `${DOMAIN}` and `${SCHEME}` placeholders for any environment value
   that embeds the app's own public URL (trusted domains, host, webhook or base
   URL). Never hard-code `localtest.me` or `http://` — they are substituted at
   install time by `Blueprint.Render`.
3. Include the required fields (`id`, `name`, `container.image`,
   `route.subdomain`, `route.internal_port`) and add `health`, `backup`,
   `lifecycle`, and `resources` where they make sense.
4. Pick a `route.subdomain` that no existing blueprint uses.
5. Add the app to both blueprint tables: `README.md` and `blueprints/README.md`.
6. Verify with `cd apps/core && go test ./internal/blueprint/`.

Use an existing file such as `blueprints/nextcloud.yaml` as a shape reference,
and see `CLAUDE.md` for the full conventions.
