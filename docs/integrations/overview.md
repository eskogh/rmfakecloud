# Integration and automation architecture

Automation is optional and separate from tablet storage and Messaging providers.
`internal/automation` defines versioned events, integrations, filters, a registry,
and a bounded in-process bus. Existing Messaging webhooks retain their multipart
payload; generic webhooks receive a JSON envelope with `version`, `id`, `event`,
`timestamp`, and typed `data` fields.

Each subscriber has a queue of 64 events and one worker. Publishing does not wait
for delivery. A slow or failed subscriber cannot block another subscriber or a
core request. Queue overflow drops the newest event and logs a warning. This is
best-effort delivery: queues are not durable, and a process restart loses pending
work. Handlers must respect context cancellation. A noncooperative handler can
stall its own worker but does not create unlimited goroutines.

Generic deliveries accept only 2xx, reject redirects, and retry network failures,
408, 429, and 5xx when configured (at most three retries). Default delays are 5s,
30s, and 2m. The handler budget is four minutes. A delivery ID stays the same
across retries; receivers should deduplicate by that ID. A timeout may happen
after a receiver accepts a request, so exactly-once delivery is not guaranteed.
SMTP and manual Messaging sends are not automatically retried.

Filters use typed fields and `equals`, `not_equals`, `contains`, `starts_with`, or
`exists`. All filters must match. Subscriptions accept exact event names, a
prefix such as `document.*`, or `*`. User ownership is checked before delivery.
There is no workflow expression language or mandatory external queue.

## Event sources

Legacy sync emits document events after metadata writes and completed deletes.
Sync 1.5 emits events from differences between immutable root trees after a
successful root write (including web library edits). Comparing trees happens on
a background worker. Failed writes and standalone blob uploads emit no document
events. `sync.completed` / `sync.failed` describe the result of sync completion or
v3 root-update requests; they do not claim to detect offline tablet failures.
No document content or unauthenticated download URLs are included by default.

## Administration and adapters

Administrators configure destinations and routing rules in the Integrations
page. Settings and the last 100 deliveries per destination persist under
`DATADIR/automation`. Generic webhooks, Telegram, and Discord share delivery
accounting, conservative retries and network policy. Manual sends use a separate
admin endpoint and the `Action` / `Attachment` abstraction.

See [webhooks and API](webhooks.md), [security](security.md),
[optional n8n](n8n.md), and [developer guidance](development.md).

Implemented event producers remain document creation/update/deletion and sync
completion/failure. `integration.test` is explicit and marked; manual sends
record `document.send_requested`. Page, device, user and automatic export events
are extension points, not claimed as implemented producers. No full document
content is automatically exported to a destination.
