# Contributing

Private Cloud Gateway is currently a personal/developer-led project.

## Branch workflow

**PRs are required before anything merges to `main`. Do not push directly to `main`.**

All work must happen on a feature branch:

```bash
git checkout -b feat/your-feature-name
# do your work
git push origin feat/your-feature-name
# open a pull request
```

See [docs/git-workflow.md](docs/git-workflow.md) for branch naming conventions, commit style, and the full PR process.

## Development principles

- Keep the architecture simple.
- Build local-first before cloud deployment.
- Keep apps behind Caddy and the auth gate.
- Prefer boring, reliable infrastructure.
- Treat backups as a v1 feature, not an afterthought.
- Avoid Kubernetes, agents, or multi-tenant SaaS complexity for v1.

## Commit style

Use [Conventional Commits](https://www.conventionalcommits.org/):

```text
feat(auth): add login endpoint with Argon2id password hashing
fix(session): set SameSite=Lax on session cookie
docs(architecture): describe forward-auth flow
test(blueprints): add blueprint parser unit tests
chore(deps): upgrade Go to 1.23
```

## Pull request checklist

Before opening a PR, verify:

- [ ] Work is on a feature branch, not `main`.
- [ ] Commits follow Conventional Commit style.
- [ ] Apps remain behind Caddy and the auth gate.
- [ ] No app container ports are exposed publicly.
- [ ] Docs are updated if behaviour changed.
- [ ] Tests are added or updated if applicable.
- [ ] Local Docker Compose still starts cleanly.
- [ ] `.env` is not committed (it is in `.gitignore`).
- [ ] No secrets, keys, or credentials are in the diff.
