# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Reference Material

A more detailed development guide already lives at `.github/copilot-instructions.md`. Read it before starting non-trivial work — it covers service/controller patterns, i18n usage, deployment script conventions and the production filesystem layout. This file captures the bigger picture and the rules that aren't obvious from `go doc` or directory listings.

## Project Overview

3X-UI is a Go web panel for managing **Xray-core** (VPN/proxy) servers. The Go binary embeds all frontend assets, drives a sibling **subscription server**, manages an external **Xray binary as a child process**, and ships a **Telegram bot** for remote administration. Persistence is SQLite via GORM. Module path: `github.com/mhsanaei/3x-ui/v2` (Go 1.26).

## Common Commands

Local dev uses `go run` directly — there is no Makefile or task runner. Same VS Code tasks live in `.vscode/tasks.json`.

```bash
# Run locally (debug mode serves HTML/assets from disk instead of embedded FS)
XUI_DEBUG=true go run ./main.go

# Build a binary (matches the VS Code "go: build" task)
go build -o bin/3x-ui.exe ./main.go

# CI gate — must all pass before pushing (see .github/workflows/release.yml)
gofmt -l .          # must produce no output
go vet ./...
staticcheck ./...   # `go install honnef.co/go/tools/cmd/staticcheck@latest`
go test -race -shuffle=on ./...

# Local formatting fix (not a CI check, but the way to satisfy `gofmt -l`)
gofmt -w .

# Run a single package's tests
go test ./web/service/...

# Run a single test by name
go test -run TestSomething ./path/to/pkg

# Container validation (when changes might affect Docker build/runtime)
docker compose up --build
```

CONTRIBUTING.md says local dev expects an `x-ui/` directory at the repo root and a `.env` copied from `.env.example` — the dotenv defaults point `XUI_DB_FOLDER` / `XUI_LOG_FOLDER` / `XUI_BIN_FOLDER` at that local folder so you don't write to `/etc/x-ui`.

## CLI Subcommands (main.go)

The binary is dual-purpose: with no args it runs the panel, otherwise it dispatches admin subcommands. These operate **directly on the SQLite DB** and are the supported way to recover from a locked-out panel:

- `x-ui run` — start panel (default)
- `x-ui migrate` — migrate from older x-ui schemas
- `x-ui setting -reset | -show | -port | -username | -password | -webBasePath | -listenIP | -resetTwoFactor | -tgbottoken | -tgbotchatid | -tgbotRuntime | -enabletgbot`
- `x-ui cert -webCert -webCertKey` (or `-reset` to clear)

When fixing bugs in this area, test by running the binary directly — these flags don't go through the web UI.

## Architecture (the parts that span files)

### Two embedded servers under one process
`main.go` boots both `web.Server` (panel) and `sub.Server` (subscription delivery) and registers them with `web/global` so cross-package code (e.g. the Telegram bot) can grab handles without import cycles. They listen on **different ports** and have independent lifecycles.

### Signal-driven restart loop
`main.go` blocks on `SIGHUP`/`SIGTERM`/`SIGUSR1`:
- `SIGHUP` → graceful restart of both servers (settings reload).
- `SIGUSR1` → restart only the embedded `xray-core` child (`server.RestartXray()`), used by jobs after Xray config changes.
- Any other signal → full shutdown.

**Critical invariant:** every restart/shutdown branch calls `service.StopBot()` *before* stopping servers. Skipping it triggers Telegram 409 conflicts because the new bot polls before the old one releases its long-poll session. Preserve this when editing the signal loop.

### Layered web stack
- `web/controller/` — Gin handlers; embed `BaseController`; use `I18nWeb(c, "key")` for translations.
- `web/service/` — business logic; services hold dependencies (e.g. `xray.XrayAPI`) and operate on GORM models.
- `web/job/` — `robfig/cron/v3` jobs (traffic counters, IP-limit enforcement, CPU alerts, LDAP sync, log rotation, hash storage check). Registered in `web/web.go` during server init — new jobs go there.
- `web/middleware/`, `web/session/` — cookie sessions via `gin-contrib/sessions`.
- `web/websocket/` — `Hub` for live updates pushed to the panel UI.

### Embedded frontend
`web/web.go` declares three `embed.FS`:
- `web/assets` → static JS/CSS/images
- `web/html/*` → Go templates (parsed recursively)
- `web/translation/*` → `nicksnyder/go-i18n` TOML bundles

In `XUI_DEBUG=true` mode the server reads **both HTML templates and `web/assets/`** directly from disk (see the `config.IsDebug()` branch in `Server.initRouter`), so template, CSS and JS edits all hot-reload. Production builds serve everything from the embedded FS, so non-debug runs require a rebuild for any frontend change.

### Xray integration
`xray/` owns the external binary:
- `xray/config.go` — translates panel inbounds/outbounds into a `config.json` for Xray.
- `xray/process.go` — spawns/kills the binary at `{bin_folder}/xray-{os}-{arch}`.
- `xray/api.go` — gRPC client to Xray's stats API for live traffic counters.
- `xray/client_traffic.go`, `xray/inbound.go`, `xray/traffic.go` — domain helpers around the API.

When changing config generation, regenerate the running config by sending `SIGUSR1` (or call `Server.RestartXray()`) — don't bounce the whole panel.

### Database & seeders
- `database/db.go` opens SQLite at `config.GetDBPath()` and runs GORM `AutoMigrate`.
- All models live in the single file `database/model/model.go` — extend there.
- One-time data migrations are tracked via the `HistoryOfSeeders` table; check it before writing a new seeder so you don't double-run (the existing bcrypt password migration is the canonical example).

### Telegram bot
`web/service/tgbot.go` is large (~3700 lines) and uses `mymmrac/telego` with long polling and `telegohandler.BotHandler` for routing. The i18n FS from `web/web.go` is passed in at startup. Anything that restarts the panel must funnel through `service.StopBot()` (see the signal loop point above).

## Conventions That Aren't Obvious From The Code

- **i18n keys, not strings**: user-facing text in controllers/templates goes through `I18nWeb(c, "pages.x.y")` and lives in `web/translation/translate.*.toml`. Add the key in **every** language file or the missing-locale will fall through to English.
- **Config helpers > env reads**: use `config.GetDBPath()`, `config.GetLogLevel()`, `config.IsDebug()` etc. rather than reading `XUI_*` env vars directly.
- **Don't add new top-level dirs of HTML/assets** — they won't be embedded unless you also extend the `//go:embed` directives in `web/web.go`.
- **CGO is required** (sqlite3); the release workflow cross-compiles with Bootlin musl toolchains and links statically. Pure-Go builds (`CGO_ENABLED=0`) will not work.

## Testing & PR Conventions

- Place tests beside the code they cover as `*_test.go`. Prefer table-driven tests for branching service/utility logic.
- Add a regression test for any bug fix where the behaviour can be isolated without an external service (Xray, Telegram, LDAP).
- PRs should include: short summary, the testing performed, linked issues, and screenshots/short recordings for visible UI changes.
- When a change touches **login, TLS/cert handling, the Telegram bot, the database schema, or Xray config generation**, call out the security/operational impact explicitly in the PR description — these are the panel's trust boundaries.

## Gotchas Worth Remembering

1. SIGHUP path must `StopBot()` first — repeated 409 errors are the symptom of a regression here.
2. Subscription server runs on a different port and has its own cert settings (`SetSubCertFile` / `SetSubKeyFile`); updating the panel cert via `cert` subcommand also updates the sub server cert — keep that paired.
3. IP-limit enforcement is "last IP wins" — when a client exceeds `LimitIP`, the oldest connections are killed via the Xray API (see `web/job/check_client_ip_job.go`). Behaviour is intentional, not a bug.
4. Default credentials are `admin/admin` (bcrypt-hashed by the seeder). `showSetting` warns when they're still in use — useful when reproducing user reports.
5. Production paths assumed by install scripts: binary `/usr/local/x-ui/`, DB `/etc/x-ui/x-ui.db`, logs `/var/log/x-ui/`, systemd unit `/etc/systemd/system/x-ui.service`. Don't hardcode these in Go — they come from `config.Get*Path()`.
