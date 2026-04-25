# Known Scheduling Edge Cases

This document records scheduling edge cases that are intentionally left unaddressed for now.

## Mid-Sequence Configuration Changes

The reminder sequence may be reconfigured while a client is already partway through a period's reminder flow. For now, the app does not need to guard against these changes.

Examples:

- The configured per-email minimum business-day gaps change after one or more reminders have already been sent for a period.
- The number of configured reminder steps changes after the send log already contains sequence indexes for the period.
- A stored send log references a sequence index that no longer exists in the client's current configuration.

The implementation uses the current configuration together with persisted successful send records. It does not need migration, reconciliation, or defensive behavior for these mid-period configuration changes yet.

If this becomes important later, define explicit product behavior before implementing it. Possible future choices include freezing the sequence configuration per period, validating send logs against historical configuration snapshots, or alerting administrators when the current configuration cannot explain prior send records.
