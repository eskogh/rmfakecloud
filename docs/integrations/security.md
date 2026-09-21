# Automation security

New automation administration, history, tests, rules and manual sends require an
administrator session. Integrations are bound to one user and the router checks
ownership before subscriptions or rules. Secret values are never returned by
these APIs: token/signing/endpoint configured flags replace them, and all custom
header values are blanked. Endpoints are hidden because paths and query strings
can themselves contain credentials. Error responses, delivery logs, and history
do not include destination response bodies or network error strings containing URLs.

Settings are stored under `DATADIR/automation/integrations.json`; atomic writes
use mode 0600 and the directory is created with mode 0700. Secrets are plaintext
on disk, not encrypted at rest. Restrict filesystem and backup access. Exact
`${VARIABLE}` references work for bearer tokens, bot tokens, signing secrets and
header values. Missing variables fail the delivery without revealing values.
They must be present in the rmfakecloud process/container environment.

HTTPS is the default. `RM_AUTOMATION_ALLOW_HTTP=true` allows plaintext HTTP.
`RM_AUTOMATION_ALLOW_PRIVATE_NETWORKS=true` allows internal destinations. Both
are necessary for HTTP services on the Compose network. Enabling these settings
lets administrators send requests into trusted networks; use narrowly scoped
integration credentials and network egress controls where appropriate.

The default transport blocks loopback, private, link-local, non-global and
shared-address-space IPs, including IPv6. DNS is checked at connection time and
the validated IP is dialed directly; any blocked address rejects the entire DNS
answer. Redirects and environment proxies are disabled. TLS verification remains
enabled. These checks are defense in depth, not a replacement for an egress firewall.
Native Telegram always uses the official Bot API hostname. Event-provided URLs
are never fetched automatically. Discord disables mentions for document text.

The existing tablet Messaging providers retain their configuration and payload
contract, including their established destination/network behavior. The new
network restrictions apply to automation adapters, not legacy providers. SMTP
configuration and mail delivery are unchanged. Legacy APIs retain their existing
credential contract; new automation secrets must not be placed there.

History keeps only the last 100 delivery records per integration, without files,
request bodies, headers or secrets. Settings allow at most 100 integrations and
100 rules. The event bus is bounded and best effort, so a full queue or restart
can lose events. Retrying a timed-out request may duplicate a native message.

The newly added legacy Messaging signing secret and header values are also
masked; blank edits retain saved values. Older provider fields keep their
existing API behavior for compatibility.
