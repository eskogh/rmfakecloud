Integration is the [feature added in reMarkable
2.10](https://support.remarkable.com/hc/en-us/articles/4406214540945)
that allows to browse, download and upload document from location
outside of the tablet.

You can edit your integrations using the Integration tab in the UI.


## WebDAV

It can be used with any WebDAV services, for example a Nextcloud/Owncloud instance.

Add this to your [`.userprofile`](userprofile.md):

```yaml
integrations:
  - provider: webdav
    id: [generate some uuid]
    name: [some name]
    username: [username]
    password: [password]
    address: [webdavaddrss]
    insecure: [true/false] (to skip certificate checks)
```


For example:

```yaml
integrations:
  - provider: webdav
    id: fLAME8YBm5uFJ89GKRAFkGjk7hJw0heow045kfhc
    name: Home Nextcloud
    address: https://home.example.com/remote.php/dav/files/user42/
    username: user42
    password: password4242
```



## Local File System

!!! warning
    Experimental and not suited for multiple users yet

You can share a dedicated path on your system. This can be a simple directory or a mount point using FUSE or whatever.

Add this to your [`.userprofile`](userprofile.md):

```yaml
integrations:
  - provider: localfs
    id: [generate some uuid]
    name: [some name]
    path: /some/path/with/files
```



## Messaging webhook

Messaging are a type of integration added in software 3.17.

Originally designed for Slack, this feature allows you to send your current sheet as an attachment to a Slack Canvas. The default behavior uses AI to transcribe the handwritten content on your sheet and posts both the text and the image to Slack.

The Webhook integration extends this capability by sending your sheet to an external automation platform (like [n8n](https://n8n.io/), [Make.com](https://www.make.com/), ...) or custom service. This is especially useful if you want to: use your own AI pipeline or don't want AI to be involved at all, or store and process sheets in a custom backend, ...

The webhook gives you full control: you decide what happens with your data.

Configure the webhook in your [`.userprofile`](userprofile.md):

```yaml
integrations:
  - provider: webhook
    id: [generate some uuid]
    name: My webhook
    endpoint: https://example.com/webhook
    timeout: 30s
    headers:
      Authorization: "Bearer your-token"
      X-API-Key: "your-api-key"
    hmacsecret: "your-shared-signing-secret"
    hmacheader: X-Webhook-Signature
```

Only `2xx` responses count as successful sends. The response body is returned to
the tablet as the message ID (an empty body gives an empty ID). Redirects are
not followed. The timeout covers the HTTP request and reading its response;
it defaults to `30s` when omitted or zero and must not be negative.

`headers`, `hmacsecret`, and `hmacheader` are optional. Headers can carry bearer
tokens or other authentication secrets. The generated multipart `Content-Type`
always takes precedence over custom headers.

When `hmacsecret` is set, rmfakecloud computes HMAC-SHA256 over the exact raw
multipart request body, including boundaries and the PNG attachment. The signature
header defaults to `X-Webhook-Signature`, with value `sha256=<lowercase hex digest>`.
`hmacheader` changes the header name; it cannot be `Content-Type`. The generated
signature takes precedence over a custom header of the same name. Receivers should
verify the raw body before parsing multipart data and compare signatures in
constant time. This signature authenticates the body but does not prevent replay.
Keep header credentials and signing secrets private and use HTTPS endpoints.

New signing secrets and custom header values are masked by the dashboard API.
Blank values on edit preserve saved values; remove a header key to delete it.
Edit the profile directly to remove a saved legacy HMAC secret. Messaging
responses are limited to 64 KiB, and network errors omit credential-bearing URLs.

## Event-driven automation

Administrators can also add generic JSON webhooks, Telegram and Discord in the
Integrations page, with subscriptions, filters, routing rules, tests and delivery
history. These are separate from tablet Messaging providers. See the
[automation overview](../integrations/overview.md) and
[optional n8n setup](../integrations/n8n.md).
