## Run the local source with Compose

The repository's `docker-compose.yml` builds the current checkout with its
Dockerfile, including the redesigned UI. Run commands from the repository root.
Docker Engine/Desktop and the Docker Compose plugin are required.

### First start

Create `.env` once with a random signing secret:

```sh
(umask 077; set -C; printf 'JWT_SECRET_KEY=%s\n' "$(openssl rand -hex 32)" > .env)
```

For additional settings, copy the relevant entries from `.env.example` into
`.env`. Keep the signing secret unchanged across rebuilds and restarts so paired
clients and login tokens remain valid. Compose rejects a missing or empty secret.
Local `.env`, `data/`, and `certs/` are excluded from Git and Docker build context.

```sh
docker compose build
docker compose run --rm rmfakecloud setuser -u admin -a -s
docker compose up -d
```

The second command creates a sync-1.5 administrator and prints a generated
password for a new account. Create this account before starting the server so
its first-user registration state is initialized correctly. Open <http://localhost:3000> to sign in. The server is
also reachable on your host's LAN address. Pair the tablet using the
[device setup guide](../remarkable/setup.md).

### Persistent storage

`./data` is mounted at `/data`. It holds users, documents, SQLite library data,
snapshots, and local backups (`./data/library/backups`). Restarting or replacing
the container preserves this directory. No separate database container is needed.
If you already have data, put it in `./data` or change the source of the volume
mount to the existing directory before starting.

Local backup files on the same disk do not protect against disk failure. Stop the
service before copying the entire data directory for a consistent offline backup,
and preserve `.env` separately.

### Configuration and HTTPS

`.env` is passed to the application, so supported SMTP, OCR, and other
[configuration settings](configuration.md) can be added there. Compose fixes the
container's `PORT=3000` and `DATADIR=/data`; use `HTTP_PORT` to change the host port.

The default connection is HTTP for LAN use or behind a reverse proxy. For a proxy
running on the host, set `BIND_ADDRESS=127.0.0.1`. Configure the proxy to support
WebSocket upgrades and forward to port 3000. For HTTPS access, set
`RM_HTTPS_COOKIE=true`; set `RM_TRUST_PROXY=true` only when requests reach the
application through your trusted proxy. See the [nginx](reverse-proxy/nginx.md)
and [Apache](reverse-proxy/apache.md) guides.

Leave `STORAGE_URL` unset unless your installation needs it. On firmware 3.15 and
later, a configured URL must use HTTPS without an explicit port, for example
`https://cloud.example.com`.

For direct TLS, place your certificate chain and key in `./certs`, add
`TLS_CERT=/certs/fullchain.pem` and `TLS_KEY=/certs/privkey.pem` to `.env`, and create
`docker-compose.override.yml` with:

```yaml
services:
  rmfakecloud:
    volumes:
      - ./certs:/certs:ro
    ports:
      - "${BIND_ADDRESS:-0.0.0.0}:8883:8883"
```

With a certificate configured, the main listener becomes HTTPS on the same
container port 3000. Set `HTTP_PORT=443` for standard HTTPS on the host, then use
`https://your-hostname`. The additional TCP port 8883 is for MQTT screen sharing
on older firmware. Direct TLS enables it with the same certificate. Restart the
container after renewing the certificate. REST signaling on firmware 3.27+ does
not require this additional port; see [screen sharing](configuration.md#screen-sharing).

### Operations

```sh
docker compose logs --tail=100 -f rmfakecloud
docker compose ps
docker compose stop
docker compose start
```

Rebuild after pulling source changes:

```sh
git pull
RMFAKECLOUD_VERSION="$(git describe --tags --always)" docker compose up -d --build
```

The build version appears on the administrator Health page; it defaults to
`local` if not provided. Logs rotate at 10 MB with three files. The service
restarts unless explicitly stopped and gets 60 seconds for shutdown.

The final Docker image uses `scratch` and contains no shell or HTTP probe tool.
Compose therefore does not define a nonfunctional curl/wget healthcheck. Inspect
`/health` in the web UI as an administrator and use your external monitoring for
HTTP availability.

### Use the upstream published image

For upstream releases without the changes in this checkout, the existing image
can still be run directly:

```sh
docker run --rm -p 3000:3000 -v "$(pwd)/data:/data" --env-file .env ddvk/rmfakecloud
```
