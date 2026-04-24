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

When building this app, please set the GOCACHE env var to point at the `./.gocache` sub-dir to avoid sandbox issues.
