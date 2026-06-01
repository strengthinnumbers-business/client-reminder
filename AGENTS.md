# Agent Notes

## Commands

- Use an absolute repo-local Go cache to avoid sandbox/cache permission issues: `GOCACHE="$(pwd)/.gocache" go test ./...`.
- `make test` only runs `go test ./internal/...`; use `go test ./...` when changes may affect `cmd/...` or bootstrap wiring.
- Run the production app with `go run ./cmd/client-reminder`; run demo wiring with `go run ./cmd/demo-client-reminder`.
- Docker builds only the production entrypoint: `docker build -t client-reminder .` builds `./cmd/client-reminder` into an Alpine image.
- Manual Notion helpers are `//go:build ignore` scripts under `scripts/`; run them with `go run scripts/<name>.go`, not as packages.

## Architecture Boundaries

- This is a hexagonal Go service: core domain and flow live under `internal/core`, runtime side effects live under `internal/adapters`, and composition lives under `internal/bootstrap`.
- Core ports in `internal/core/ports` must stay free of adapter/storage/API types; do not leak Notion payloads, JSON file records, SMTP details, or HTTP response shapes into core.
- `cmd/client-reminder` uses JSON-file client/completion/state adapters and SMTP; `cmd/demo-client-reminder` uses Notion clients/tasks plus JSON send/period state and SMTP.
- Keep demo-only behavior in `internal/bootstrap/wire-demo.go` or outer adapters. The core should receive injected clock/logger abstractions, not demo-specific conditionals.

## Runtime Configuration

- Production bootstrap requires `SMTP_HOST`, `SMTP_FROM`, and `ADMIN_EMAIL`; email body content must come from `EMAIL_TEMPLATE_PATH` or `EMAIL_BODY_TEMPLATE`.
- Important default paths are `configs/clients.json`, `state/completion-verdicts.json`, `state/reminder-sends.json`, `state/period-resolutions.json`, and `state/holiday-cache`.
- `LOG_LEVEL` accepts `demo`, `debug`, `info`/empty, and `error`; demo logs are intentionally audience-facing for demo runs.
- Demo bootstrap requires `NOW` parseable as `YYYY-MM-DD` or RFC3339, plus Notion data source IDs via `NOTION_CLIENTS_DATA_SOURCE_ID` and `NOTION_TASKS_DATA_SOURCE_ID`.

## Scheduling And State

- Reminder scheduling uses per-email minimum business-day gaps in `Client.ReminderGaps`; empty gaps default to `entities.ReminderGapsStandard`.
- Scheduling details and accepted edge cases are in `context/PERIODS_AND_SEQUENCES.md` and `context/KNOWN_SCHEDULING_EDGE_CASES.md`; consult them before changing reminder sequence behavior.
- Failed sends are recorded for diagnostics but must not advance the sequence; only successful sends returned by `ListSuccessfulSends` determine the next sequence index.
- Reminder send JSON persistence is a flat object with `sends: []entities.SendLogEntry`; `ClientID` belongs directly on each entry.
- Missing completion verdicts mean `CompletionVerdictNotRequested` and reminders continue; `CompletionUndecided` pauses reminders; `CompletionIncomplete` sends then resets the verdict.

## Adapters And External Services

- Constructors with optional seams use `New(required, options ...Option)` and `With...` helpers; keep `canadaholidaysapi.New(cacheDir, options...)` with `cacheDir` as the fixed first parameter.
- Application logging goes through the core-owned `ports.Logger`; do not add package-level `log`/`slog` calls in core or adapters.
- Ordinary `go test ./...` must stay deterministic and offline; real-service checks belong in `scripts` or behind explicit env/build-tag gates.
- Shared sparse Notion API code lives in `internal/adapters/notionapi`; reuse it instead of duplicating Notion request/response mapping in higher-level adapters.
- The Notion API client uses internal-connection tokens from `NOTION_API_KEY`, Notion-Version `2026-03-11`, a default 333 ms gap between requests, and one retry for 429 responses.
- Use `notionapi.Properties.Text(name)` for common Notion text-like extraction; only add page-property-item support if multi-value people/relation pagination becomes required.
- Notion client configuration maps active Notion pages to core `entities.Client`; default field names include `Contact Emails`, `Period Type`, `Schedule Preset`, and `Status`, and queries filter `Status` to `active`.
- Notion completion maps task `Status` values `unset`, `undecided`, `upload_incomplete`, and `upload_complete`; it caches the queried task snapshot once per app run and resets incomplete tasks to `unset` with `UpdatePageSelect`.
