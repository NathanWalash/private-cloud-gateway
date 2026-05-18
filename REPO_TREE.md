# Repository Structure

```text
private-cloud-gateway/
├── README.md                         Project overview
├── ROADMAP.md                        Milestone definitions and task lists
├── CONTRIBUTING.md                   How to contribute and PR requirements
├── SECURITY.md                       Security policy and requirements
├── INSTALL_PREREQS.md                Quick prerequisite checklist
├── Makefile                          Helper targets (dev, test, lint)
├── .env.example                      Environment variable template
├── .gitignore                        Ignored files
│
├── .github/
│   ├── pull_request_template.md      Default PR template
│   └── workflows/
│       └── ci.yml                    CI pipeline
│
├── docs/
│   ├── dev-local.md                  Local development setup guide
│   ├── git-workflow.md               Branch, commit, and PR process
│   ├── 00-project-vision.md
│   ├── 01-user-experience.md
│   ├── 02-architecture.md
│   ├── 03-technical-stack.md
│   ├── 04-security-model.md
│   ├── 05-routing-and-domains.md
│   ├── 06-docker-and-app-blueprints.md
│   ├── 07-backup-and-restore.md
│   ├── 08-testing-strategy.md
│   ├── 09-deployment-oracle-cloud.md
│   ├── 10-development-roadmap.md     → mirrors ROADMAP.md
│   ├── 11-design-system.md
│   ├── 12-ai-coding-agent-brief.md
│   └── decisions/
│       ├── ADR-001-use-go-for-core-engine.md
│       ├── ADR-002-use-caddy-for-ingress.md
│       ├── ADR-003-use-sqlite-for-control-plane.md
│       ├── ADR-004-use-docker-compose-first.md
│       ├── ADR-005-subdomains-over-iframes.md
│       ├── ADR-006-backups-are-v1-requirement.md
│       └── ADR-007-oracle-cloud-as-primary-host.md
│
├── blueprints/
│   ├── README.md
│   └── filebrowser.example.yaml      Example blueprint
│
├── apps/
│   ├── core/                         Go Core backend (not yet implemented)
│   │   └── README.md
│   └── web/                          Vite + TypeScript dashboard (not yet implemented)
│       └── README.md
│
├── infra/
│   ├── caddy/                        Caddy configuration
│   │   └── README.md
│   ├── docker/                       Docker Compose files
│   │   └── README.md
│   └── oracle/                       Oracle Cloud production setup
│       └── README.md
│
└── scripts/
    ├── README.md
    ├── dev-up.sh                     Start dev stack (stub)
    ├── dev-down.sh                   Stop dev stack (stub)
    ├── test.sh                       Run tests (stub)
    ├── backup-now.sh                 Trigger local backup (stub)
    ├── restore.sh                    Restore from backup (stub)
    └── lint.sh                       Format and lint checks (stub)
```

Scripts marked as `(stub)` print a TODO message and exit. They will be implemented in Milestone 1.
