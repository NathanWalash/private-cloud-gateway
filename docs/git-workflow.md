# Git Workflow

## Branch strategy

`main` is the stable branch. It is protected.

**PRs are required before anything merges to `main`. Do not push directly to `main`.**

All work happens on a short-lived feature branch. When the work is ready, open a pull request and merge it.

```text
main
 └── chore/initial-repo-foundation   ← you are here
 └── feat/auth-login-endpoint
 └── feat/caddy-forward-auth
 └── fix/session-cookie-samesite
 └── docs/update-routing-guide
```

## Branch naming

Use this pattern:

```text
<type>/<short-description>
```

Types:

| Type | When to use |
|---|---|
| `feat` | New feature or behaviour |
| `fix` | Bug fix |
| `chore` | Tooling, config, dependencies, repo maintenance |
| `docs` | Documentation only |
| `test` | Tests only |
| `refactor` | Code restructure without behaviour change |
| `ci` | CI/CD changes |

Examples:

```text
feat/go-auth-session
feat/caddy-forward-auth
feat/blueprint-parser
fix/session-expiry-edge-case
chore/update-dependencies
docs/add-routing-guide
test/blueprint-parser-unit
refactor/docker-lifecycle-manager
ci/add-go-lint
```

Keep branch names lowercase, hyphenated, and concise.

## Commit style

Use [Conventional Commits](https://www.conventionalcommits.org/):

```text
<type>(<scope>): <short description>
```

The description should complete the sentence: "This commit will…"

Good examples:

```text
feat(auth): add login endpoint with Argon2id password hashing
feat(caddy): add forward-auth middleware to protect app routes
fix(session): set SameSite=Lax on session cookie
chore(deps): upgrade Go to 1.23
docs(architecture): add sequence diagram for forward-auth flow
test(blueprints): add unit tests for blueprint YAML parser
refactor(docker): extract lifecycle manager into its own package
ci(lint): add golangci-lint to PR checks
```

Bad examples (do not use these):

```text
update stuff
changes
initial
fixes
docs
wip
added feature
```

Keep the subject line under 72 characters. Use the body for context if needed.

## Pull request process

1. Branch from `main`.
2. Make small, focused commits.
3. Open a PR against `main`.
4. Fill in the PR template.
5. Review the diff yourself before requesting review.
6. Merge once the checklist is satisfied.

Keep PRs focused. A PR that adds a feature should not also refactor unrelated code.

## Merge strategy

Prefer **squash merge** or **merge commit** — whichever keeps the `main` history readable.

Do not force-push to `main`.

## First-time setup

```bash
git clone <repo-url>
cd private-cloud-gateway
cp .env.example .env
```

See [docs/dev-local.md](dev-local.md) for full local development setup.
