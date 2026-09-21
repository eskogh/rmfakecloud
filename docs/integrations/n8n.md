# Optional n8n automation

rmfakecloud works without n8n. The optional Compose profile runs it separately
with persistent settings and credentials in `n8n_data`. The editor port binds
only to localhost by default. This follows the [n8n Docker installation model](https://docs.n8n.io/hosting/installation/docker).

1. Set up `.env` as described in the Docker installation guide. For the internal
   HTTP connection, add `RM_AUTOMATION_ALLOW_HTTP=true` and
   `RM_AUTOMATION_ALLOW_PRIVATE_NETWORKS=true`.
2. Start `docker compose --profile automation up -d`.
3. Open `http://localhost:5678` (use an SSH tunnel for a remote host) and complete
   n8n's owner setup.
4. Import `integrations/templates/n8n-document-event.json`. In its Webhook node,
   create a Header Auth credential named `Authorization` with a value such as
   `Bearer YOUR_RANDOM_TOKEN`. The workflow ships inactive and without credentials.
5. In rmfakecloud, add a Webhook integration with endpoint
   `http://n8n:5678/webhook/rmfakecloud-events`, bearer authentication using the
   same token, and the document events you want. Publish/activate the workflow
   in n8n before using this production URL.
6. Click Test in rmfakecloud. Check delivery history and the workflow execution.
   The event is available as `$json.body`; a test has
   `$json.body.event === "integration.test"` and `$json.body.data.test === true`.

The template ends at a no-op node so importing it cannot send data to another
service. Replace that node with your destination or processing steps. n8n uses
ordinary generic webhooks, with no proprietary rmfakecloud protocol.

A future OCR workflow can receive an event, fetch an export through an
authenticated rmfakecloud API, run OCR, turn the result into Markdown and upload
it to Nextcloud. The template does not provision credentials, download files,
perform OCR, or install paid/cloud services. Add these steps and their credentials
in your own workflow. Events currently contain document metadata only.

The example acknowledges reception immediately: rmfakecloud delivery success
means n8n accepted the webhook, not that subsequent workflow steps succeeded.
Use n8n execution/error handling for downstream failures. Header Auth protects
the supplied template; it does not verify HMAC. If you enable signing, add raw-body
verification before processing, following the webhook documentation.

For deployments behind HTTPS, set `N8N_SECURE_COOKIE=true` and configure the
external webhook URL/proxy in n8n. The localhost example uses HTTP cookies only
for local setup. Pin `N8N_IMAGE` to your tested version or digest and back up the
volume before upgrades. Stopping n8n does not stop rmfakecloud; failed deliveries
follow the configured retry limits.
