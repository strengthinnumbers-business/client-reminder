# Periods and Sequences

- Each client is assigned a region (typically a province or a territory) which informs which days are considered statutory holidays.

- This app does only trigger email sending on business days (Monday to Friday) that are not considered to be statutory holidays in the client's region.

- The first Monday of a period is the first candidate for the first reminder. If it falls on a statutory holiday in the client's region, then the next business day that is not a statutory holiday is used.

- Reminder gaps are configured as per-email minimum business-day spans in the `ReminderGaps` client field.

- Any other days (like weekend days or statutory holidays) are not eligible reminder send days and do not count toward minimum gaps.

## Minimum-Gap Reminder Sequences

- The reminder sequence uses per-email minimum business-day gaps from the previously sent reminder.

- Example `ReminderGaps`: `[0, 3, 2, 2]`.
  - The first reminder has no preceding email. Its `0` gap is relative to the first valid sequence day of the period.
  - The second reminder may be sent only after at least 3 eligible business days have passed since the first reminder was actually sent.
  - The third and fourth reminders may each be sent only after at least 2 eligible business days have passed since their respective predecessor was actually sent.

- Eligible gap days are Monday through Friday, excluding statutory holidays in the client's region. Dates are evaluated in UTC for now.

- Missed app runs should delay the sequence, not skip it. If the first valid sequence day is missed, the first reminder can still be sent on the next valid run.

- A delayed reminder delays all following reminders because each minimum gap is measured from the actual successful send time of the preceding reminder.

- Failed sends should be logged for debugging but must not advance the reminder sequence.

- Only successful reminder sends should be recorded as sequence-advancing send history.

- The app should send at most one reminder per client per daily run, even if multiple reminders would appear overdue after missed runs.

- If a whole period is missed, the new period supersedes the old period. The app should alert an administrator through a dedicated core port, record that the missed period was dealt with, and then continue processing reminders for the current period.

- Administrator alerts for missed periods are not limited by business days or holidays. They should be sent as soon as the app detects that a period ended with no reminders sent.

- The persisted dealt-with state should include a free-text reason or description that explains how the period was dealt with. For example, a missed period may be dealt with by sending an administrator alert, while periods before or during onboarding for a newly added client may be marked as dealt with for an onboarding/baseline reason without alerting.

- If a period has no successful reminder sends because its completion verdict is already `CompletionComplete`, it should be marked as dealt with without sending an administrator alert.

- `CompletionUndecided` means a data upload has happened and a completion verdict is pending. While the verdict is pending, the reminder sequence should pause.

- `CompletionVerdictNotRequested` means no data upload has triggered a verdict yet. In that state, reminders continue according to the minimum-gap schedule.

- This app uses the https://raw.githubusercontent.com/pcraig3/hols/refs/heads/main/reference/Canada-Holidays-API.v1.yaml OpenAPI spec for the https://canada-holidays.ca/api API to find out which regions (provinces or territories) observe which statutory holidays. Please use this API with plenty of caching time, a default of 1 lookup per month is sufficient. 
