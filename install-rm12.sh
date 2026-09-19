#!/usr/bin/env bash
set -euo pipefail

RM_HOST="${1:-10.11.99.1}"
UPSTREAM="${2:-}"

if [[ -z "$UPSTREAM" ]]; then
    echo "Usage: $0 <remarkable-ip> <rmfakecloud-url>"
    exit 1
fi

echo "Connecting to $RM_HOST..."

ssh "root@$RM_HOST" 'uname -a'

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "Downloading latest rmfakecloud proxy installer..."

curl -L \
  -o "$TMP/installer-rm12.sh" \
  "https://github.com/ddvk/rmfakecloud-proxy/releases/latest/download/installer-rm12.sh"

echo "Uploading installer..."

scp "$TMP/installer-rm12.sh" "root@$RM_HOST:/tmp/installer-rm12.sh"

echo "Installing..."

ssh "root@$RM_HOST" "
    chmod +x /tmp/installer-rm12.sh
    /tmp/installer-rm12.sh install '$UPSTREAM'
"

echo
echo "Testing proxy..."

ssh "root@$RM_HOST" "
    systemctl status rmfakecloud-proxy --no-pager || true
    wget -qO- https://local.appspot.com || true
"

echo
echo "Done."
echo "Now pair the tablet with your rmfakecloud account."
