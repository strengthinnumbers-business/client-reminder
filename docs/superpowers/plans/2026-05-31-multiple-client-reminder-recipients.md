# Multiple Client Reminder Recipients Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver each client reminder as one email addressed to all configured client recipients.

**Architecture:** Replace the scalar client email with a recipient slice across the core and delivery port. Keep JSON configuration structured, parse the Notion text property at the Notion adapter boundary, and leave reminder send-state semantics unchanged.

**Tech Stack:** Go standard library, `net/smtp`, JSON-file adapters, Notion adapters.

---

### Task 1: Lock Down Client Recipient Loading

**Files:**
- Create: `internal/adapters/client/jsonfile/json_client_repository_test.go`
- Modify: `internal/adapters/client/jsonfile/json_client_repository.go`
- Modify: `internal/adapters/client/notion/notion_client_repository_test.go`
- Modify: `internal/adapters/client/notion/notion_client_repository.go`
- Modify: `internal/core/entities/entities.go`

- [ ] Add failing JSON repository tests proving that `Emails` is an array whose values are preserved exactly and that an empty array is rejected.
- [ ] Add failing Notion repository tests proving that `Contact Emails` accepts mixed comma/newline separators, trims items, removes blanks, and rejects an empty result.
- [ ] Run `GOCACHE="$(pwd)/.gocache" go test ./internal/adapters/client/...` and confirm the new tests fail because recipient slices are not implemented.
- [ ] Replace `Client.Email` with `Client.Emails`, validate non-empty JSON arrays, and add a Notion-only parser.
- [ ] Run `GOCACHE="$(pwd)/.gocache" go test ./internal/adapters/client/...` and confirm it passes.

### Task 2: Send One Email To All Recipients

**Files:**
- Modify: `internal/core/ports/email_sender.go`
- Modify: `internal/adapters/email/mock/mock_email_sender.go`
- Create: `internal/adapters/email/smtp/smtp_email_sender_test.go`
- Modify: `internal/adapters/email/smtp/smtp_email_sender.go`
- Modify: `internal/adapters/adminalert/email/email_admin_alerter.go`
- Modify: `internal/core/service/reminder_service_test.go`
- Modify: `internal/core/service/reminder_service.go`

- [ ] Add failing service and SMTP tests proving that a reminder passes one recipient slice, SMTP uses it as the envelope-recipient list, and the message contains a comma-separated `To` header.
- [ ] Run focused tests and confirm they fail because the email port is still scalar.
- [ ] Change `EmailSender.SendEmail` to accept `[]string`, update reminder delivery and logs, and wrap the administrator address as a one-item slice.
- [ ] Run `GOCACHE="$(pwd)/.gocache" go test ./internal/core/service ./internal/adapters/email/... ./internal/adapters/adminalert/...` and confirm it passes.

### Task 3: Update Configuration, Documentation, And Generated Output

**Files:**
- Modify: `configs/clients.json`
- Modify: `AGENTS.md`
- Modify: `docs/DETAILS.md`
- Regenerate: `demo-cleaned-code/**`

- [ ] Replace the sample JSON scalar `Email` with an `Emails` array.
- [ ] Document JSON arrays, the Notion `Contact Emails` field, and one-transaction delivery semantics.
- [ ] Run `make demo-clean-code` to regenerate tracked demo output.
- [ ] Run `gofmt` on changed source test files.
- [ ] Run `GOCACHE="$(pwd)/.gocache" go test ./...`.
- [ ] Inspect `git diff --check` and `git status --short`.
