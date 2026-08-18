## What & why

<!-- What does this change do, and why? Link any related issue. -->

## Changes

<!-- Bullet the key changes. -->

-

## Testing

<!-- How did you verify this? -->

- [ ] `cd apps/core && go test -race ./...` passes
- [ ] `make lint` passes
- [ ] Frontend builds (`cd apps/web && pnpm build`) — if UI changed

## Checklist

- [ ] Conventional Commit title (`feat(scope):`, `fix(scope):`, ...)
- [ ] Docs/README updated if behaviour or blueprints changed
- [ ] New blueprints pin an explicit image version (no `:latest`) and use `${DOMAIN}`/`${SCHEME}` for domain-specific env
