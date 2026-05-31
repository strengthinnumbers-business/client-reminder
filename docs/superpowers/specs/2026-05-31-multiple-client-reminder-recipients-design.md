# Multiple Client Reminder Recipients Design

## Goal

Allow a client reminder to be delivered as one email addressed to multiple `To`
recipients while preserving the existing reminder-sequence behavior.

## Domain Model

Replace `entities.Client.Email string` with `entities.Client.Emails []string`.
There is no compatibility layer for the old scalar field.

A reminder remains one logical send. One SMTP transaction addressed to all
client recipients produces one successful or failed send-log entry. Only a
successful transaction advances the reminder sequence.

## Client Configuration

Production JSON clients use a structured array:

```json
{
  "Emails": [
    "ops@acme.example",
    "finance@acme.example"
  ]
}
```

The JSON adapter preserves array values exactly. It does not split or trim
them. It rejects a client with no configured recipients.

The Notion adapter renames its mapped field to `Emails` and defaults the
Notion property name to `Contact Emails`. This text-like property may contain
addresses separated by commas, newlines, or both. The adapter trims
whitespace around each address, discards blank entries, and rejects a client
when no recipients remain.

Neither adapter performs full email-address format validation. SMTP remains
responsible for rejecting malformed addresses.

## Email Delivery

Change the core `EmailSender` port to accept `[]string`. The SMTP adapter uses
that slice as the envelope-recipient list passed to `smtp.SendMail` and renders
a comma-separated `To` header. The mock sender records the slice for tests.

Administrator alerts still have one configured address and call the port with
`[]string{adminEmail}`.

## Generated Demo Code

`demo-cleaned-code` is generated output. Edit source files under `internal/...`
and regenerate the tracked output with `make demo-clean-code`.

## Testing

Add focused tests for:

- strict JSON array loading and empty-array rejection;
- Notion mixed-delimiter parsing, whitespace trimming, blank removal, and
  empty-recipient rejection;
- service delivery to multiple recipients as one logical reminder;
- SMTP envelope recipients and comma-separated `To` header;
- administrator alert wrapping its configured address as a one-item slice;
- regeneration of `demo-cleaned-code` followed by the full offline suite.
