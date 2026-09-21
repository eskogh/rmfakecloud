# Generic webhooks and administration

Open **Integrations → Add integration**, choose Webhook, enter the endpoint and
select subscriptions such as `document.updated` or `sync.*`. Use Test to queue a
marked `integration.test` event, even for a disabled integration. Test bypasses
filters/subscriptions and goes only to the selected destination. A queued test
is not proof of delivery; check History for its result.

Administrators use the existing authenticated `/ui/api` API. The legacy
`/integrations` routes remain dedicated to tablet providers; new routes are:

| Method | Path after `/ui/api` | Purpose |
| --- | --- | --- |
| GET / POST | `/automation` | List / create |
| GET / PUT / DELETE | `/automation/:id` | Read / replace / delete |
| POST | `/automation/:id/test` | Queue test (202) |
| GET | `/automation/:id/deliveries` | Last 100 deliveries |
| GET / PUT | `/automation/rules` | Read / replace all rules |
| POST | `/automation/:id/send` | Explicit manual send |

Use the same session authentication as other dashboard API calls. Example
creation body (JSON; duration values are strings):

```json
{
  "name": "Work notes",
  "type": "webhook",
  "user_id": "admin",
  "enabled": true,
  "endpoint": "https://example.com/hooks/remarkable",
  "events": ["document.created", "document.updated"],
  "headers": {"X-Service": "rmfakecloud"},
  "auth": {"type": "bearer", "token": "${WEBHOOK_TOKEN}"},
  "signing": {"enabled": true, "secret": "${WEBHOOK_SECRET}"},
  "timeout": "10s",
  "retry": {"count": 3, "delays": ["5s", "30s", "2m"]},
  "filters": [{"field": "document.name", "operator": "starts_with", "value": "Work"}]
}
```

The API supplies an ID when omitted. An omitted user defaults to the current
administrator. Each integration belongs to exactly one user; type and ownership
cannot change on update. Create a new integration to change them. PUT replaces
configuration, while blank endpoint, token, secret, and existing header values
keep saved secrets. Remove a header key to delete it; disable auth/signing to
stop using retained credentials. Delete the integration to erase stored settings.
Only POST delivery is supported. Retry count defaults to zero.

Payloads have `version: "1"`, a unique event `id`, `event`, UTC `timestamp`, and
`data` with `user`, optional `document`, `page`, `content`, `source`, and `test`.
Document events contain metadata, not full files. Unknown additive fields should
be ignored. Breaking schema changes require a new version.

## Signature verification

Headers identify the event and delivery:
`X-RMFakeCloud-Event`, `X-RMFakeCloud-Delivery`, `X-RMFakeCloud-Timestamp`, and
(optional) `X-RMFakeCloud-Signature`. The signature is `sha256=` followed by the
hexadecimal HMAC-SHA256 of `timestamp + "." + raw request body`, using the shared
secret. Verify the original bytes before decoding JSON. Compare in constant
time, reject stale timestamps, and deduplicate delivery IDs. The same ID is
retained across retries; each attempt has a fresh timestamp and signature.

Python verification core (called with raw bytes and header strings):

```python
import hashlib, hmac, time

def verify(raw_body, timestamp, signature, secret):
    try:
        if abs(time.time() - int(timestamp)) > 300:
            return False
    except (TypeError, ValueError):
        return False
    expected = "sha256=" + hmac.new(
        secret.encode(), timestamp.encode() + b"." + raw_body, hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(expected, signature)
```

Keep successful delivery IDs in a bounded receiver-side store for your retry
window. A 2xx acknowledges receipt; a failed response-body read does not cause
a generic webhook to replay an already accepted event. Normal 4xx and redirects
are permanent failures. 408, 429, 5xx and transport failures can be retried.
All delays and attempts share the bus's four-minute budget.

## Rules and manual sends

Rules contain `id`, `name`, `enabled`, `user_id`, `event`, `filters`, and
`integration_id`. All filters on a rule must match. Rule matches OR together with
direct subscriptions; destination filters and enabled state still apply. This
produces at most one delivery per integration for a published event, even when
multiple routes match. Set `events: []` for rule-only destinations. Rules cannot
route another user's data. Remove referencing rules before deleting a destination.
The UI edits rules, and `integrations/templates/receipt-rule.json` is an API
starting point; replace its placeholders before submitting it.

Manual send accepts `{"text":"Hello"}` as JSON, or multipart fields `text` and
`attachment` (up to 25 MiB). It runs synchronously on its own admin route with a
bounded timeout and records `document.send_requested`. Telegram/Discord support
attachments; generic webhooks support text events. Manual sends bypass routing
filters because the administrator explicitly chooses the destination, but reject
disabled integrations. This adds no unsupported tablet UI protocol.
