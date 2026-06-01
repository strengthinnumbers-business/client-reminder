# Client Reminder

Client Reminder is a small Go service that sends scheduled reminder emails when clients need to upload recurring data.

For a demo audience, think of it as an automated daily checklist:

1. Load the active clients.
2. Decide which reporting period each client is in.
3. Check whether today is an eligible business day for that client's region.
4. Check whether the client's upload is already complete, still pending review, or missing.
5. Send at most one reminder email per client when a reminder is due.
6. Record what happened so the next run continues the sequence correctly.

The app is designed to be run once per day by an external scheduler such as cron, a Kubernetes CronJob, or an ECS Scheduled Task.

## What The Demo Shows

The demo runner connects the reminder engine to:

- Notion, for the client list and upload/completion status.
- SMTP, for outgoing reminder and administrator alert emails.
- Local JSON state files, for reminder history and missed-period tracking.
- The Canada Holidays API, for province/territory holiday awareness.
- A fixed `NOW` value, so the demo can reliably replay a specific day.

The production runner uses the same core reminder logic, but reads clients and completion state from local JSON files instead of Notion.

## Key Concepts

- Client: an organization that must upload data on a recurring schedule.
- Period: the current reporting window, such as a monthly upload period.
- Reminder sequence: the configured series of reminder emails for a period.
- Reminder gap: the minimum number of eligible business days between emails.
- Completion verdict: the current status of the client's upload for the period.
- Missed period: a period that ended before any reminder could be sent; the app alerts an administrator.

Reminder scheduling is business-day aware. Weekends and statutory holidays for the client's region are not eligible send days and do not count toward reminder gaps.

## Demo Run

Set the required demo environment variables, then run the demo entrypoint:

```bash
export NOTION_API_KEY=secret_xxx
export NOTION_CLIENTS_DATA_SOURCE_ID=your_clients_data_source_id
export NOTION_TASKS_DATA_SOURCE_ID=your_tasks_data_source_id
export NOW=2026-05-05

export SMTP_HOST=mail.example.com
export SMTP_FROM=no-reply@example.com
export ADMIN_EMAIL=admin@example.com
export EMAIL_TEMPLATE_PATH=configs/email-template.txt

go run ./cmd/demo-client-reminder
```

Useful demo options:

- `LOG_LEVEL=demo` prints audience-friendly progress logs.
- `SMTP_PORT` defaults to `25`.
- `SMTP_USERNAME` and `SMTP_PASSWORD` are optional unless your SMTP server requires them.
- `REMINDER_SEND_STATE_PATH` defaults to `state/reminder-sends.json`.
- `PERIOD_RESOLUTION_STATE_PATH` defaults to `state/period-resolutions.json`.
- `HOLIDAY_CACHE_DIR` defaults to `state/holiday-cache`.
- `UPLOAD_DIR` defaults to `state/upload-mirror`.
- `UPLOAD_SNAPSHOT_DIR` defaults to `state/upload-snapshots`.

`NOW` must be either `YYYY-MM-DD` or RFC3339, for example `2026-05-05` or `2026-05-05T09:00:00Z`.

## Production Run

The production entrypoint uses local JSON files for client and completion inputs:

```bash
export SMTP_HOST=mail.example.com
export SMTP_FROM=no-reply@example.com
export ADMIN_EMAIL=admin@example.com
export EMAIL_TEMPLATE_PATH=configs/email-template.txt

go run ./cmd/client-reminder
```

Default local files:

- `configs/clients.json` contains client configuration.
- `state/completion-verdicts.json` contains upload completion verdicts.
- `state/reminder-sends.json` records successful and failed send attempts.
- `state/period-resolutions.json` records periods that were already handled.
- `state/holiday-cache` caches holiday lookups.

Optional production settings:

- `CLIENTS_JSON_PATH` overrides `configs/clients.json`.
- `COMPLETION_STATE_PATH` overrides `state/completion-verdicts.json`.
- `REMINDER_SEND_STATE_PATH` overrides `state/reminder-sends.json`.
- `PERIOD_RESOLUTION_STATE_PATH` overrides `state/period-resolutions.json`.
- `EMAIL_SUBJECT_TEMPLATE` defaults to `Reminder to upload your data`.
- `EMAIL_BODY_TEMPLATE` can be used instead of `EMAIL_TEMPLATE_PATH`.
- `LOG_LEVEL` accepts `demo`, `debug`, `info`, or `error`.

Client reminder recipients are configured as an `Emails` JSON array. Each
array item is one address. The production JSON adapter preserves those values
exactly and rejects an empty array.

The Notion-backed demo reads recipients from the `Contact Emails` property.
That text-like property accepts addresses separated by commas, newlines, or
both. Surrounding whitespace and blank entries are removed.

Each client reminder is sent as one SMTP transaction addressed to all of the
client's configured `To` recipients. A successful transaction advances the
client's reminder sequence once.

## Email Template

The default template is `configs/email-template.txt`. It can use fields from each client and the current run, including:

- `{{Greeting}}`
- `{{PeriodID}}`
- `{{UploadPrompt}}`
- `{{FolderURL}}`
- `{{RunDate}}`

## Docker

Build and run the production entrypoint:

```bash
docker build -t client-reminder .
docker run --rm \
  -e SMTP_HOST=mail.example.com \
  -e SMTP_FROM=no-reply@example.com \
  -e ADMIN_EMAIL=admin@example.com \
  -e EMAIL_TEMPLATE_PATH=configs/email-template.txt \
  client-reminder
```

## Development

Run all tests with a repo-local Go cache:

```bash
GOCACHE="$(pwd)/.gocache" go test ./...
```

High-level architecture:

- `internal/core` contains the reminder rules and workflow.
- `internal/core/ports` defines interfaces for side effects.
- `internal/adapters` contains integrations such as SMTP, Notion, JSON files, logging, and holidays.
- `internal/bootstrap` wires the core service to production or demo adapters.
- `cmd/client-reminder` is the production entrypoint.
- `cmd/demo-client-reminder` is the Notion-backed demo entrypoint.

More scheduling detail lives in `context/PERIODS_AND_SEQUENCES.md`.
