#!/usr/bin/env bash
set -euo pipefail

# Restore rmfakecloud-proxy on a reMarkable after an OS update.
#
# Supported by the current upstream rmfakecloud-proxy installer:
#   - reMarkable 1
#   - reMarkable 2
#   - reMarkable Paper Pro
#   - reMarkable Paper Pro Move
#
# Run this on your Mac/Linux computer. It detects the tablet model, downloads
# the correct installer from ddvk/rmfakecloud-proxy, copies it to the tablet,
# installs it, configures your cloud URL, and verifies the proxy/TLS setup.
#
# Usage:
#   ./restore-rmfakecloud-after-update.sh --cloud https://rm.example.com
#
# Optional:
#   ./restore-rmfakecloud-after-update.sh \
#       --cloud https://rm.example.com \
#       --host 10.11.99.1 \
#       --user root
#
# Environment variables can also be used:
#   CLOUD_URL=https://rm.example.com RM_HOST=10.11.99.1 ./restore-rmfakecloud-after-update.sh
#
# Defaults:
#   RM_HOST=10.11.99.1   (standard reMarkable USB address)
#   RM_USER=root

RM_HOST="${RM_HOST:-10.11.99.1}"
RM_USER="${RM_USER:-root}"
CLOUD_URL="${CLOUD_URL:-}"

GITHUB_RELEASE_BASE="https://github.com/ddvk/rmfakecloud-proxy/releases/latest/download"

usage() {
    cat <<EOF
Usage:
  $0 --cloud URL [--host HOST] [--user USER]

Options:
  --cloud URL    rmfakecloud URL, for example https://rm.example.com
  --host HOST    reMarkable SSH address (default: ${RM_HOST})
  --user USER    SSH user (default: ${RM_USER})
  -h, --help     Show this help

Environment variables:
  CLOUD_URL      Same as --cloud
  RM_HOST        Same as --host
  RM_USER        Same as --user

Examples:
  $0 --cloud https://rm.example.com
  $0 --cloud https://rm.example.com --host 192.168.1.50
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --cloud)
            [[ $# -ge 2 ]] || { echo "ERROR: --cloud requires a value" >&2; exit 2; }
            CLOUD_URL="$2"
            shift 2
            ;;
        --host)
            [[ $# -ge 2 ]] || { echo "ERROR: --host requires a value" >&2; exit 2; }
            RM_HOST="$2"
            shift 2
            ;;
        --user)
            [[ $# -ge 2 ]] || { echo "ERROR: --user requires a value" >&2; exit 2; }
            RM_USER="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "ERROR: Unknown argument: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

if [[ -z "$CLOUD_URL" && -t 0 ]]; then
    read -r -p "rmfakecloud URL (for example https://rm.example.com): " CLOUD_URL
fi

if [[ ! "$CLOUD_URL" =~ ^https?://[^[:space:]]+$ ]]; then
    echo "ERROR: Supply a valid http:// or https:// cloud URL with --cloud." >&2
    exit 2
fi

CLOUD_URL="${CLOUD_URL%/}"

if [[ ! "$RM_USER" =~ ^[a-zA-Z0-9_][a-zA-Z0-9_.-]*$ || "$RM_HOST" == -* || "$RM_HOST" == *[[:space:]]* || -z "$RM_HOST" ]]; then
    echo "ERROR: Invalid SSH user or host." >&2
    exit 2
fi
SSH_TARGET="${RM_USER}@${RM_HOST}"
# Quote arguments for the tablet's shell, independently of the local shell.
shell_quote() {
    printf "'%s'" "${1//\'/\'\\\'\'}"
}
CLOUD_URL_Q="$(shell_quote "$CLOUD_URL")"

WORKDIR="$(mktemp -d "${TMPDIR:-/tmp}/rmfakecloud-restore.XXXXXX")"
INSTALLER_FILE="${WORKDIR}/installer.sh"
CONTROL_SOCKET="${WORKDIR}/ssh"

cleanup() {
    ssh -S "$CONTROL_SOCKET" -O exit "$SSH_TARGET" >/dev/null 2>&1 || true
    rm -rf "$WORKDIR"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

echo
echo "============================================================"
echo " reMarkable rmfakecloud recovery"
echo "============================================================"
echo " Tablet : $SSH_TARGET"
echo " Cloud  : $CLOUD_URL"
echo

echo "[1/8] Connecting to the tablet..."
echo "      You may be asked for the reMarkable root password."

ssh \
    -M \
    -S "$CONTROL_SOCKET" \
    -o ControlPersist=180 \
    -o ServerAliveInterval=15 \
    -o ServerAliveCountMax=4 \
    -fnNT \
    "$SSH_TARGET"

SSH=(ssh -S "$CONTROL_SOCKET" "$SSH_TARGET")
SCP=(scp -O -o "ControlPath=$CONTROL_SOCKET")

echo "      Connected."

echo
echo "[2/8] Detecting reMarkable model..."

MACHINE="$("${SSH[@]}" 'cat /sys/devices/soc0/machine 2>/dev/null' | tr -d '\r')"

case "$MACHINE" in
    "reMarkable 1.0"|"reMarkable Prototype 1")
        DEVICE_NAME="reMarkable 1"
        INSTALLER_ASSET="installer-rm12.sh"
        ;;
    "reMarkable 2.0")
        DEVICE_NAME="reMarkable 2"
        INSTALLER_ASSET="installer-rm12.sh"
        ;;
    "reMarkable Ferrari")
        DEVICE_NAME="reMarkable Paper Pro"
        INSTALLER_ASSET="installer-rmpro.sh"
        ;;
    "reMarkable Chiappa")
        DEVICE_NAME="reMarkable Paper Pro / Move family"
        INSTALLER_ASSET="installer-rmpro.sh"
        ;;
    *)
        echo "ERROR: Unsupported or unknown device reported as:" >&2
        echo "       '$MACHINE'" >&2
        echo >&2
        echo "The current upstream rmfakecloud-proxy installer does not list this model." >&2
        exit 1
        ;;
esac

echo "      $DEVICE_NAME ($MACHINE)"
echo "      Installer: $INSTALLER_ASSET"
REMOTE_INSTALLER="/home/root/$INSTALLER_ASSET"
REMOTE_INSTALLER_Q="$(shell_quote "$REMOTE_INSTALLER")"

echo
echo "[3/8] Checking access to the cloud from the tablet..."

if "${SSH[@]}" "wget -T 10 -q -O /dev/null $CLOUD_URL_Q"; then
    echo "      Reachable."
else
    echo "      WARNING: The tablet cannot reach the cloud yet; continuing USB recovery." >&2
    echo "      The computer downloads the installer, so tablet DNS is not required to reinstall." >&2
    echo "      USB provides SSH access, not automatically internet access. Keep tablet Wi-Fi connected." >&2
fi

echo
echo "[4/8] Downloading the latest upstream installer..."

INSTALLER_URL="${GITHUB_RELEASE_BASE}/${INSTALLER_ASSET}"
curl -fL --progress-bar "$INSTALLER_URL" -o "$INSTALLER_FILE"

if [[ ! -s "$INSTALLER_FILE" ]] || ! LC_ALL=C grep -aq '^__ARCHIVE__$' "$INSTALLER_FILE"; then
    echo "ERROR: Downloaded file does not look like an rmfakecloud-proxy installer." >&2
    exit 1
fi

echo "      Downloaded: $INSTALLER_ASSET"

echo
echo "[5/8] Copying installer to the tablet..."

"${SCP[@]}" "$INSTALLER_FILE" "${SSH_TARGET}:${REMOTE_INSTALLER}"
"${SSH[@]}" "chmod 700 $REMOTE_INSTALLER_Q"

echo "      Copied."

echo
echo "[6/8] Checking installer tools on the tablet..."

"${SSH[@]}" 'set -e
for tool in bash openssl systemctl update-ca-certificates gunzip tail awk; do
    command -v "$tool" >/dev/null || { echo "ERROR: Missing required tool: $tool" >&2; exit 1; }
done
'
# The upstream installer prepares the filesystem itself, including Paper Pro.

echo "      Ready."

echo
echo "[7/8] Installing rmfakecloud-proxy..."
echo

if ! "${SSH[@]}" "$REMOTE_INSTALLER_Q install $CLOUD_URL_Q"; then
    echo "      Direct install failed; retrying install with its URL prompt, then setcloud." >&2
    # Supply the cloud URL and blank optional client-certificate answers. No tablet internet needed.
    if ! printf '%s\n\n\n' "$CLOUD_URL" | "${SSH[@]}" "$REMOTE_INSTALLER_Q install"; then
        echo "ERROR: Both installation attempts failed. See installer output above." >&2
        exit 1
    fi
fi

# Explicitly set the cloud even if installation succeeded, avoiding stale upstream configuration.
"${SSH[@]}" "$REMOTE_INSTALLER_Q setcloud $CLOUD_URL_Q"

echo
echo "[8/8] Verifying installation..."
echo

"${SSH[@]}" "bash -s -- $CLOUD_URL_Q" <<'REMOTE'
set -e
cloud_url=$1

echo '--- Service ---'
if ! systemctl is-active --quiet rmfakecloud-proxy.service; then
    echo 'ERROR: rmfakecloud-proxy.service is not active.'
    systemctl status rmfakecloud-proxy.service --no-pager -l || true
    exit 1
fi
systemctl status rmfakecloud-proxy.service --no-pager --lines=5

echo
echo '--- Configured upstream ---'
if ! systemctl cat rmfakecloud-proxy.service | grep -F -- "$cloud_url" >/dev/null; then
    echo "ERROR: Service is not configured for $cloud_url"
    exit 1
fi
printf '%s\n' "$cloud_url"

echo
echo '--- Host interception ---'
if ! awk '$1 == "127.0.0.1" {for (i=2; i<=NF; i++) if ($i == "local.appspot.com") found=1} END {exit !found}' /etc/hosts; then
    echo 'ERROR: local.appspot.com is not redirected to 127.0.0.1.'
    exit 1
fi
echo 'local.appspot.com -> 127.0.0.1'

echo
echo '--- TLS interception ---'
VERIFY=$(
    echo Q |
    openssl s_client \
        -connect 127.0.0.1:443 \
        -servername local.appspot.com \
        -verify_hostname local.appspot.com \
        -CAfile /etc/ssl/certs/ca-certificates.crt \
        2>&1 |
    grep 'Verify return code:' |
    tail -1 || true
)
printf '%s\n' "$VERIFY"
case "$VERIFY" in
    *'0 (ok)'*) ;;
    *) echo 'ERROR: Local proxy certificate verification failed.'; exit 1 ;;
esac

echo
echo '--- Cloud path through proxy ---'
if ! wget -T 15 -q -O /dev/null https://local.appspot.com; then
    echo 'Proxy, local host entries, and certificates were restored, but cloud access still fails.' >&2
    echo 'Checking direct cloud access for a more specific error:' >&2
    wget -T 15 -O /dev/null "$cloud_url" || true
    echo 'If wget reports bad address, connect tablet Wi-Fi and check its DNS or VPN/split-DNS configuration.' >&2
    echo 'USB SSH access alone does not provide internet. Certificates cannot fix hostname resolution.' >&2
    echo 'If it resolves but fails TLS/HTTP, check the cloud certificate and reverse proxy.' >&2
    exit 1
fi
echo 'Proxy request successful.'
REMOTE

echo
echo "============================================================"
echo " rmfakecloud proxy restored successfully"
echo "============================================================"
echo
echo "Device : $DEVICE_NAME"
echo "Cloud  : $CLOUD_URL"
echo
echo "You can now use Check sync on the reMarkable."
