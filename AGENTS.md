Always update AGENTS.md with the latest learnings and emerging conventions. If sections in AGENTS.md become too big, refactor those sections into Markdown files in the `/context/` folder.

Keep README.md at a high level overview, only. If README.md really needs details, then link to the relevant section(s) in AGENTS.md or the relevant files in `/context/`.

---

This is a Go app to send regular email reminders to clients / customers, reminding them to upload their recent data to a file folder shared on the internet.

The app runs in a Docker container, once a day, scheduled via a cron job or similar. During that scheduled run, it checks it's current configuration and currently stored state, and depending on that, decides which emails to send to which recipient.

The app is constructed after the principles of the "hexagonal microservice architecture". I.e. the business entities and the business logic are at the core of the service, and are completely independent of any implementation details that connect them to the outside world, like email services, state storage / repository or configuration storage / repository, etc. The inner core defines the interfaces for the connections to the outside world that it needs. Those connections are called "ports".
For each port we provide at least 2 "adapters".
One adapter is a mock adapter to help with testing. It fulfills the contract of the port only by name, but has no further side effects other than help with test assertions.
The other adapter implements an actual connection to a real service or facility that will have the actual intended side effects, like sending actual emails, or provide true persistence for storing state and / or configuration.
These "port" interfaces MUST BE FREE OF any implementation details of any and all outside adapter implementations.

Ports MUST STAY FREE of any adapter details at all times!

I.e. the core package MUST NOT have any dependencies on any of the adapters or any of their implementation details, like any Go types that represent storage data in a format that is specific to the storage backend, like DB rows, etc., or API requests and responses of the email sending service, etc.

You can find all ports [here](./internal/core/ports).

You can find all business entities [here](./internal/core/entities).

When building this app, please set the GOCACHE env var to an absolute path pointing at the `./.gocache` sub-dir to avoid sandbox issues.

Standalone development helper scripts live in `./scripts`. Keep them self-contained and runnable with `go run`; for example, `scripts/print-calendar-comments.go` prints pasteable Go-comment calendars for scheduling tests. Scripts that need subcommands should use `github.com/alecthomas/kong`; bind interface-typed command dependencies with `kong.BindTo(value, (*Interface)(nil))`. `scripts/try-notion-api-client.go` manually exercises the sparse Notion API client against a real Notion connection and accepts `--notion-api-key` or falls back to `NOTION_API_KEY`.

Scheduling details and known edge cases are documented in:

- [Periods and Sequences](./context/PERIODS_AND_SEQUENCES.md)
- [Known Scheduling Edge Cases](./context/KNOWN_SCHEDULING_EDGE_CASES.md)

Reminder scheduling uses per-email minimum business-day gaps in the `ReminderGaps` client field. Successful sends advance the sequence; failed sends are only logged for debugging. Missing completion verdicts mean `CompletionVerdictNotRequested`, so reminders continue until an upload triggers `CompletionUndecided`.

Reminder email templates are retrieved through `GlobalConfiguration.GetEmailBodyTemplate(sequenceIndex, style)`, using the current `ReminderEligibility.SequenceIndex` and `Client.EmailStyle`. The port returns both subject and body templates; `EmailSender.SendEmail` accepts the rendered subject explicitly.

Reminder send logs store `ClientID` directly on each `SendLogEntry`; JSON reminder-send persistence should marshal a flat `[]SendLogEntry` under `sends`, not wrap entries in adapter-specific record objects.

Log all errors to allow diagnosing any failed operation, if available with added context like client and period, etc.

Shared sparse Notion API code lives in `./internal/adapters/notionapi` so multiple outer adapters can reuse it without leaking Notion request / response details into core ports. It only supports the endpoints this app needs, uses internal-connection tokens from `NOTION_API_KEY`, sends the current `Notion-Version` header, and spaces every API request at least 333 ms after the previous request ends.

Notion-backed client configuration lives in `./internal/adapters/client/notion`. Keep Notion field-name mapping configurable there; the default field names mirror `entities.Client` plus a `Status` field. Client queries should filter Notion `Status` to `active`, and the repository should return core `entities.Client` values only.
