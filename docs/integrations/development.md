# Developing integrations

`internal/automation` owns typed event payloads, the bus, registry, destination
configuration, network policy, delivery accounting and rule routing. Core sync
code publishes after committed mutations. Never depend on successful automation
delivery to commit a document or reply to a tablet.

Implement `Integration` (`ID`, `Name`, `Type`, `Enabled`, `SubscribedEvents`, and
`HandleEvent(context.Context, Event) error`). Register it through `Registry` with
its owning-user configuration so ownership, subscriptions and filters apply.
The manager's `newIntegration` factory builds persisted adapters. Add type
validation and UI configuration when extending that factory. Handlers must
honor cancellation, close streams, and never log raw errors containing secrets.

```go
func (i *MyIntegration) HandleEvent(ctx context.Context, e automation.Event) error {
    // Deliver with ctx, a bounded client timeout and sanitized errors.
    return i.deliver(ctx, e)
}
```

Each subscription gets a bounded queue and one worker; panic recovery and handler
error logging live in the bus. Config updates cancel the old worker. Do not spawn
unbounded work from a handler. Queue durability, replay, multi-process coordination
and workflow orchestration are intentionally outside this implementation.

`Native` shares the webhook delivery loop but formats destination-specific
requests. It implements `Action.Send` for explicit sends. `Attachment.Open` must
return a fresh cancellable read-closer each attempt; callers close it. The adapter
spools multipart data into a private temporary file, enforces 25 MiB and size
consistency, and removes the spool when finished. Actual provider file limits may
be lower. Future exporters can use this abstraction for EPUB, SVG or Markdown.
The manager's manual send entry point has an overall per-action timeout equal to
the configured timeout; retries share that budget. Event deliveries instead have
the bus's four-minute overall budget and a per-request timeout.

Telegram uses bot token plus chat ID, `sendMessage` for notifications, `sendPhoto`
for PNG, and `sendDocument` for other files; it checks the JSON `ok` response.
See the [Telegram Bot API](https://core.telegram.org/bots/api).
Discord sends content with mentions disabled and requests confirmation using
`wait=true`. Attachments use multipart `files[0]` and `payload_json`.
See [Discord webhooks](https://docs.discord.com/developers/resources/webhook#execute-webhook).
Home Assistant uses the generic webhook adapter; Nextcloud and OAuth-heavy
services can be handled externally or by a future adapter without changing events.

Templates live under `integrations/templates/`. Keep workflow credentials absent,
workflows inactive, and example placeholders explicit. Avoid promising exports
or authentication that a template does not implement.

Validation: `go test ./...`, `go test -race ./internal/automation ./internal/ui
./internal/integrations ./internal/storage/fs`, `npm --prefix ui run build`, and
`npm --prefix ui run test:documents`. Use local test servers or fake transports,
not real destination credentials, in automated tests.
