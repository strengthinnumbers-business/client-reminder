# Client Reminder

Hexagonal Go service that sends reminder emails to customers to upload recent data.

## Architecture

- `internal/core/entities`: business entity types.
- `internal/core/ports`: port interfaces used by core logic.
- `internal/core/service`: business flow (daily run).
- `internal/adapters/.../mock`: mock adapters for tests.
- `internal/adapters/...`: concrete adapters for runtime side effects.

## Ports

- `EmailSender`
- `ClientRepository`
- `GlobalConfiguration`
- `CompletionDecider`
- `HolidayChecker`
- `ReminderSendRepository`
- `PeriodResolutionRepository`
- `AdminAlerter`

## Run locally

```bash
go run ./cmd/client-reminder
```

Required env vars:

- `SMTP_HOST`
- `SMTP_FROM`
- `ADMIN_EMAIL`

Optional env vars:

- `SMTP_PORT` (default: `25`)
- `SMTP_USERNAME`
- `SMTP_PASSWORD`
- `CLIENTS_JSON_PATH` (default: `configs/clients.json`)
- `EMAIL_TEMPLATE_PATH` (default: use `EMAIL_BODY_TEMPLATE` env)
- `EMAIL_SUBJECT_TEMPLATE` (default: `Reminder to upload your data`)
- `EMAIL_BODY_TEMPLATE` (used when `EMAIL_TEMPLATE_PATH` is empty)
- `COMPLETION_STATE_PATH` (default: `state/completion-verdicts.json`)
- `REMINDER_SEND_STATE_PATH` (default: `state/reminder-sends.json`)
- `PERIOD_RESOLUTION_STATE_PATH` (default: `state/period-resolutions.json`)

## Docker

```bash
docker build -t client-reminder .
docker run --rm \
  -e SMTP_HOST=mail.example.com \
  -e SMTP_FROM=no-reply@example.com \
  -e ADMIN_EMAIL=admin@example.com \
  -e EMAIL_TEMPLATE_PATH=configs/email-template.txt \
  client-reminder
```

Schedule the container externally (cron, Kubernetes CronJob, ECS Scheduled Task, etc.) once per day.
