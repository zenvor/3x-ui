# AGENTS.md

This file is the authoritative local guidance for AI coding agents working in
this repository.

## Repository Snapshot

3X-UI is a Go web panel for managing Xray-core servers. The Go binary starts
the panel server, a separate subscription server, background jobs, a Telegram
bot, and an external Xray child process. Persistence is handled with GORM over
SQLite or PostgreSQL, depending on runtime configuration.

- Module path: `github.com/mhsanaei/3x-ui/v3`
- Go version: `1.26.4`
- Frontend: React 19, TypeScript, Ant Design 6, Vite 8
- Frontend output: `web/dist/`, embedded into the Go binary
- i18n files: `web/translation/*.json`
- Local branch work often includes custom `subconverter/` changes on top of
  upstream 3X-UI.

## Communication and Workflow

- Respond in Simplified Chinese when working in this workspace.
- Read the code around the target area before editing it.
- For non-trivial work, make a short plan first and revise it if the approach
  changes.
- Prefer root-cause fixes over temporary patches.
- Keep changes narrow. Do not reformat or refactor unrelated files.
- Before calling work done, run the smallest meaningful verification command.
- When merging a GitHub PR through the web UI, review and fill both the commit
  message title and the Extended description before clicking the final confirm
  button.
- Do not run `git pull`, `git pull --rebase`, or automatic merge/rebase flows
  unless explicitly asked. If the branch is behind or diverged, stop and report
  options instead.

## Common Commands

The repository has no Makefile. Use direct Go and npm commands.

```bash
# Backend tests. CI stubs web/dist first because go:embed requires it.
mkdir -p web/dist && touch web/dist/.gitkeep
go test ./...

# CI uses an explicit package list.
go list ./... | grep -v '/frontend/node_modules/' > /tmp/go-packages.txt
go test $(cat /tmp/go-packages.txt)

# Targeted backend tests.
go test ./web/service/...
go test -run TestSomething ./path/to/pkg

# Frontend setup and checks. Use Node 22+ from .nvmrc.
cd frontend
npm ci
npm run lint
npm run typecheck
npm run build

# Local backend run. Copy .env.example to .env first when needed so paths
# resolve to the local x-ui/ folder instead of production locations.
XUI_DEBUG=true go run ./main.go

# Build production binary. Build frontend first so web/dist exists.
cd frontend && npm run build
cd ..
CGO_ENABLED=1 go build -o bin/3x-ui ./main.go
```

CI currently runs Go tests, `govulncheck`, frontend lint, frontend build, and
`npm audit --audit-level=high`. CI does not currently run `npm run typecheck`,
but local frontend changes should. Release builds compile the frontend before Go
because `web/web.go` embeds `web/dist`.

## Frontend Architecture

`frontend/` is the source tree for the panel UI. Vite emits HTML, JS, and CSS
into `web/dist/`. `web/controller/dist.go` serves the page HTML, while
`web/web.go` serves `/assets/` from either disk in debug mode or embedded
`dist/assets` in production.

Important frontend conventions:

- Use existing React page structure under `frontend/src/pages/`.
- Use shared API setup in `frontend/src/api/` and shared domain models in
  `frontend/src/models/`.
- Use `frontend/src/i18n/` for React i18n wiring; locale data lives in
  `web/translation/*.json`.
- Normal panel pages share `frontend/index.html`. Add the React page, register
  it in `frontend/src/routes.tsx`, add sidebar/navigation as needed, and add a
  `panelSPA` route in `web/controller/xui.go`.
- Add a new HTML entry and Vite `rollupOptions.input` only for a standalone
  entry page like login or subpage.
- Vite dev reads the panel base path from the configured SQLite DB through
  `node:sqlite`; keep Node at 22+.
- Do not revive legacy `web/html` templates or `web/assets` vendor bundles.

## Backend Architecture

The main server layers are:

- `main.go` starts the panel server and subscription server, wires global
  handles, and handles signals.
- `web/web.go` configures Gin, sessions, security headers, gzip, i18n,
  embedded frontend assets, controllers, WebSocket, cron jobs, and
  `subconverter` route registration.
- `web/controller/` owns HTTP handlers and request/response concerns.
- `web/service/` owns business logic and database-facing operations.
- `web/job/` owns cron jobs for Xray health, traffic accounting, IP limits,
  LDAP sync, log cleanup, node sync, and other periodic tasks.
- `web/runtime/` abstracts local and remote node runtime behavior.
- `web/websocket/` owns live panel updates.
- `database/` owns main database setup and models.
- `xray/` owns config generation, process lifecycle, and stats API access.

Use config helpers from `config/` instead of reading `XUI_*` environment
variables directly.

## Critical Runtime Invariants

- The panel server and subscription server are separate servers with separate
  lifecycles.
- `sub.SetDistFS(web.EmbeddedDist())` must happen before starting the
  subscription server so subscription pages can use embedded frontend assets.
- Xray config changes should restart Xray with `SIGUSR1` or service methods,
  not by bouncing the whole panel unless the setting really requires it.
- `SIGHUP` currently restarts panel-owned HTTP/background resources and the
  subscription server through `StopPanelOnly` / `StartPanelOnly`; it does not
  restart Xray or the Telegram bot.
- Full shutdown stops the Telegram bot before server teardown to avoid Telegram
  long-polling 409 conflicts. If a future change makes `SIGHUP` restart the bot,
  stop the old bot before starting a new polling session.
- The `cert` command updates both panel cert settings and subscription server
  cert settings; keep them paired.
- CGO is required because of SQLite. `CGO_ENABLED=0` is not a valid normal
  build mode.

## Subconverter Module

`subconverter/` is a local module integrated from `web/web.go` through
`subconverter.RegisterRoutes`.

Routes:

- Admin SPA: `{basePath}/panel/subconverter`
- Admin API: `{basePath}/panel/api/subconverter/*`
- Public Mihomo YAML: `/feed/:token`
- Public Mihomo provider nodes: `/feed/:token/nodes`

Persistence uses a separate SQLite database at
`config.GetDBFolderPath()/subconverter.db`. With the local `.env.example`
defaults this is normally `x-ui/subconverter.db`; on production installs it is
normally `/etc/x-ui/subconverter.db`. Keeping the file separate means normal
3X-UI import/export and upstream database migrations do not touch its data.

Current converter scope is intentionally narrow: exportable nodes are enabled
VLESS TCP Reality inbounds with usable Reality endpoint data, no non-empty
`externalProxy`, no TCP HTTP header, and enabled clients with IDs. The frontend
does pre-filtering for inbound shape and enabled/email clients, but it cannot
verify client IDs from the slim inbound endpoint; the backend resolver remains
the final authority.

## Upstream Sync Risks

This branch carries local work on top of upstream 3X-UI. During upstream merges
or rebases, explicitly check these integration points:

- `subconverter/` package and `subconverter.RegisterRoutes` call in
  `web/web.go`.
- `sub.SetDistFS(web.EmbeddedDist())` before subscription server startup.
- `web/controller/dist.go` and all React SPA serving behavior.
- `frontend/src/routes.tsx`, `frontend/vite.config.js`, and sidebar navigation.
- API docs route whitelist in `web/controller/api_docs_test.go`.
- PostgreSQL support and SQLite-to-Postgres migration paths.

## Generated and Local Files

Do not commit local runtime or dependency output:

- `.env`
- `web/dist/*` except `web/dist/.gitkeep`
- `frontend/node_modules/` and root `node_modules/`
- local `x-ui/` runtime folders
- SQLite files and WAL/SHM files such as `x-ui.db`, `subconverter.db`, `*.db-wal`
  and `*.db-shm`
- logs, `bin/`, release archives, Docker override files
- Docker runtime folders such as `db/`, `cert/`, and `pgdata/`

`frontend/public/openapi.json` is generated by `npm run build` and is tracked.
If it changes, inspect the diff and commit it only when the API documentation
change is intentional.

## i18n

Backend and frontend translations live in `web/translation/*.json`. The Go
bundle registers JSON unmarshalling in `web/locale/locale.go`; React reads the
same locale files through frontend i18n utilities.

When adding user-facing strings:

- Add keys to every locale JSON file, or deliberately document the fallback.
- Prefer stable keys over inline user-facing strings in controllers and shared
  frontend components.
- Keep backend messages that are surfaced to users translatable.

## Database and Migrations

- Main models live under `database/model/`.
- Main DB initialization is in `database/db.go`.
- Default main DB storage is SQLite at `config.GetDBPath()`. When
  `XUI_DB_TYPE=postgres`, `database.InitDB` ignores that path and uses
  `XUI_DB_DSN`.
- `migrate-db -dsn ... -src ...` copies data from a SQLite file into a
  PostgreSQL database.
- The main DB can use PostgreSQL, but `subconverter` still persists in its own
  SQLite file under `config.GetDBFolderPath()`.
- One-time seed/migration history uses the `HistoryOfSeeders` table; check the
  existing pattern before adding a new seeder.
- CLI recovery commands operate directly on the DB and should be tested as CLI
  flows, not only through HTTP handlers.

## CLI Commands

With no arguments, the binary runs the panel. Relevant admin subcommands:

- `x-ui run`
- `x-ui migrate`
- `x-ui migrate-db -dsn <postgres-dsn> [-src <sqlite-file>]`
- `x-ui setting -reset | -show | -port | -username | -password | -webBasePath | -listenIP | -resetTwoFactor | -tgbottoken | -tgbotchatid | -tgbotRuntime | -enabletgbot`
- `x-ui cert -webCert | -webCertKey | -reset`

When changing these paths, run the built binary directly and verify the flags.

## Tests and Verification

Choose verification based on risk:

- Go service/controller changes: targeted `go test ./path/...`.
- Shared backend behavior: `mkdir -p web/dist && touch web/dist/.gitkeep`
  followed by `go test ./...`.
- Frontend changes: `npm run lint`, `npm run typecheck`, and usually
  `npm run build` from `frontend/`.
- Release/build changes: build frontend first, then run `CGO_ENABLED=1 go build
  -o bin/3x-ui ./main.go`.
- Docker/install changes: validate with container or VPS-like commands only
  when that is actually in scope.

Add regression tests for bug fixes when the behavior can be isolated without
external services such as Xray, Telegram, LDAP, or remote nodes.

## Install and Deployment Scripts

`install.sh` is the production installer. It writes under `/usr/local`,
`/etc/x-ui`, and the system service directory, and may start or restart the
`x-ui` service. Do not run it on a normal development machine unless that side
effect is explicitly intended.

## Git and Commit Rules

Use Conventional Commits:

```text
type[(scope)][!]: 简短中文祈使句
```

Allowed types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`,
`build`, `ci`, `chore`, `revert`.

Before committing, always inspect:

```bash
git status --short --branch
git diff --staged
```

Do not include unrelated generated files, local databases, logs, or dependency
trees. Keep one logical change per commit.

## Production Paths

Install scripts and systemd deployments expect:

- Binary directory: `/usr/local/x-ui/`
- Main DB: `/etc/x-ui/x-ui.db`
- Logs: `/var/log/x-ui/`
- systemd unit: `/etc/systemd/system/x-ui.service`

Docker defaults differ: `docker-compose.yml` maps host `$PWD/db/` to container
`/etc/x-ui/`, `$PWD/cert/` to `/root/cert/`, and PostgreSQL profile data to
`$PWD/pgdata/`.

Do not hardcode production or Docker paths in Go code. Use config helpers.
