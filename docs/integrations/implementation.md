# Implementation handoff

This continues the existing core framework and sync-event commits, plus the
in-progress administration work. Changes remain in the working tree. Unrelated
pre-existing deletions and `.dockerignore` edits were preserved.

## Files added in the continued feature

- `internal/automation/manager.go`, `manager_test.go`: persisted settings,
  masked views, test events, bounded persistent delivery history.
- `internal/automation/native.go`, `native_test.go`: Telegram/Discord notifications
  and manual streamed-source attachments, shared delivery accounting.
- `internal/automation/rules.go`, `rules_test.go`: typed routing rules, persistence,
  validation, ownership and explicit manual action entry point.
- `internal/ui/automation_handlers.go`, `automation_handlers_test.go`: admin API,
  tests/history, rules and manual sends.
- `internal/ui/integration_secrets.go`, `integration_secrets_test.go`: mask newly
  added legacy webhook secrets while retaining them through edits.
- `internal/integrations/webhook_test.go`: multipart compatibility, signing,
  status/timeout handling, response bound and safe errors.
- `ui/src/pages/Integrations/AutomationPanel.jsx`, `RulesPanel.jsx`: destination
  editor, enable/disable/test/delete/history and rule editor.
- `integrations/templates/n8n-document-event.json`, `receipt-rule.json`: inactive,
  credential-free workflow and placeholder routing example.
- `docs/integrations/webhooks.md`, `n8n.md`, `security.md`, `development.md`,
  `implementation.md`: API, operation, security, extension and handoff docs.

## Files modified in the continued feature

- `internal/automation/{config,event,registry,webhook}.go`: adapter validation,
  targeted tests, routing hooks, lifecycle and shared delivery loop.
- `internal/app/app.go`, `internal/config/config.go`: manager startup/shutdown,
  configurable HTTP/private-network policy and graceful automation load failure.
- `internal/ui/routes.go`, `handlers.go`: admin automation routes and legacy
  secret masking/preservation.
- `internal/model/user.go`, `internal/integrations/webhook.go`: compatible legacy
  timeout/header/signature options, 2xx-only success and bounded response reads.
- `ui/src/pages/Integrations/{index.tsx,IntegrationModal.jsx}`,
  `ui/src/services/api.service.js`: expose automation and preserve legacy options.
- `docker-compose.yml`, `.env.example`: optional n8n profile and policy settings.
- `docs/integrations/overview.md`, `docs/usage/integrations.md`, `mkdocs.yml`:
  documentation entry points and compatibility details.

## Decisions and compatibility

The versioned event architecture remains independent of n8n. Each integration has
one worker and a bounded queue. Rules and subscriptions are alternative matches
inside the same route, avoiding duplicate deliveries. Each route remains scoped
to one user. No external broker or Go dependency was introduced. Native messages
use the same retry and accounting loop as webhooks. Attachment sources reopen
per attempt and multipart bodies spool to private temporary files.

The tablet Messaging multipart contract and SMTP behavior remain unchanged.
Automatic sync events still publish only after committed changes. Generic events
are separate from legacy Messaging payloads; their signing formats are documented
separately. Generic integration administration uses `/ui/api/automation` to avoid
changing existing `/ui/api/integrations` endpoints. Disabled automation loading
never disables core sync. Legacy message response IDs are now bounded at 64 KiB.

New automation secrets and newly added legacy signing/header secrets are masked.
New destinations default to HTTPS with private-network restrictions, validation
at dial time, no redirects, and no environment proxy bypass. Settings/history are
written atomically with private permissions. Secrets are not encrypted at rest.

## Validation

- Baseline and final `go test ./...`.
- `go vet ./...`.
- `go test -race ./internal/automation ./internal/ui ./internal/integrations ./internal/storage/fs`.
- `npm --prefix ui run build`.
- `npm --prefix ui run test:documents` (13 tests).
- `docker compose --profile automation config -q` with a placeholder JWT secret.
- Template JSON, connection targets, inactive state and absence of credentials.
- `git diff --check`.

The frontend build reports existing Sass deprecations and bundle-size warnings.
Native API tests use local servers/fake transports. No live Telegram/Discord
credentials were used, and no n8n container was started or workflow executed.
No browser-driven UI test was performed.

## Remaining recommended work

Perform live adapter and n8n import checks in a test deployment. Add page/device/
user/export event producers when their semantics are defined. Add authenticated
export retrieval to real OCR/backup workflows; the supplied workflow intentionally
stops before any external destination. Nextcloud/OAuth provisioning and one-click
workflow installation remain future extensions. Consider a durable queue only
if loss on overflow/restart becomes unacceptable. Current delivery is best effort;
remote side effects can be duplicated after ambiguous network failures.
