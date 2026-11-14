#!/bin/sh
set -eu
umask 077

IMAGE=${NODEHUB_AGENT_IMAGE:-}
NAME=${NODEHUB_AGENT_NAME:-nodehub-agent}
STATE=${NODEHUB_AGENT_DIR:-/var/lib/nodehub-agent}
LISTEN=${NODEHUB_AGENT_LISTEN:-127.0.0.1:9443}
MODE=https
CERT=
KEY=

usage() {
    printf '%s\n' 'Usage: sh install.sh --image IMAGE [--name NAME] [--state-dir DIR] [--listen HOST:PORT] [--http | --tls-cert FILE --tls-key FILE]'
    printf '%s\n' 'Defaults: HTTPS, 127.0.0.1:9443, /var/lib/nodehub-agent. Existing containers are never replaced.'
}
while [ "$#" -gt 0 ]; do
    case "$1" in
        --image|--name|--state-dir|--listen|--tls-cert|--tls-key)
            [ "$#" -ge 2 ] || { echo "missing value for $1" >&2; exit 2; }
            case "$1" in
                --image) IMAGE=$2;; --name) NAME=$2;; --state-dir) STATE=$2;;
                --listen) LISTEN=$2;; --tls-cert) CERT=$2;; --tls-key) KEY=$2;;
            esac
            shift 2;;
        --http) MODE=http; shift;;
        -h|--help) usage; exit 0;;
        *) echo "unknown option: $1" >&2; exit 2;;
    esac
done
[ -n "$IMAGE" ] || { echo 'Specify --image with your built or published agent image.' >&2; exit 2; }
case "$STATE" in /*) ;; *) echo 'State directory must be an absolute path.' >&2; exit 2;; esac
case "$STATE" in *,*) echo 'State directory cannot contain a comma.' >&2; exit 2;; esac
if [ "$MODE" = https ]; then
    [ -n "$CERT" ] && [ -n "$KEY" ] && [ -r "$CERT" ] && [ -r "$KEY" ] || {
        echo 'HTTPS requires readable --tls-cert and --tls-key; use --http only on a trusted network.' >&2; exit 2;
    }
    case "$CERT:$KEY" in /*:/*) ;; *) echo 'TLS paths must be absolute.' >&2; exit 2;; esac
    case "$CERT$KEY" in *,*) echo 'TLS paths cannot contain a comma.' >&2; exit 2;; esac
else
    [ -z "$CERT$KEY" ] || { echo 'TLS options cannot be used with --http.' >&2; exit 2; }
    echo 'WARNING: HTTP exposes the token and configuration. Use a trusted encrypted network.' >&2
fi
command -v docker >/dev/null 2>&1 || { echo 'Docker is required.' >&2; exit 1; }
if docker inspect "$NAME" >/dev/null 2>&1; then
    echo "Container $NAME already exists; remove or rename it explicitly before installing." >&2
    exit 1
fi
if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
    docker pull "$IMAGE" >/dev/null
fi
mkdir -p "$STATE"
chmod 700 "$STATE"
if [ ! -e "$STATE/token" ]; then
    if command -v openssl >/dev/null 2>&1; then
        token=$(openssl rand -hex 32)
    else
        token=$(od -An -N32 -tx1 /dev/urandom | tr -d ' \n')
    fi
    [ "${#token}" -eq 64 ] || { echo 'Failed to generate a 256-bit token.' >&2; exit 1; }
    (set -C; printf '%s\n' "$token" > "$STATE/token") || { echo 'Token creation failed.' >&2; exit 1; }
fi
[ -f "$STATE/token" ] && [ ! -L "$STATE/token" ] || { echo 'Token must be a regular file.' >&2; exit 1; }
chmod 600 "$STATE/token"
run_user=10001:10001
if [ "$(id -u)" = 0 ]; then
    chown 10001:10001 "$STATE" "$STATE/token"
else
    run_user=$(id -u):$(id -g)
fi
if [ "$MODE" = https ]; then
    cp "$CERT" "$STATE/tls-cert.pem"
    cp "$KEY" "$STATE/tls-key.pem"
    chmod 600 "$STATE/tls-cert.pem" "$STATE/tls-key.pem"
    [ "$(id -u)" = 0 ] && chown 10001:10001 "$STATE/tls-cert.pem" "$STATE/tls-key.pem"
fi

# Host networking exposes Xray's dynamically configured inbound ports as well.
set -- docker run -d --name "$NAME" --restart unless-stopped --network host \
    --read-only --tmpfs /tmp:rw,noexec,nosuid,size=64m --cap-drop ALL \
    --security-opt no-new-privileges --pids-limit 128 --user "$run_user" \
    --mount "type=bind,src=$STATE,dst=/var/lib/nodehub-agent"
set -- "$@" "$IMAGE" -listen "$LISTEN" \
    -token-file /var/lib/nodehub-agent/token \
    -identity-file /var/lib/nodehub-agent/identity \
    -config-file /var/lib/nodehub-agent/config.json -xray /usr/local/bin/xray
if [ "$MODE" = https ]; then
    set -- "$@" -tls-cert /var/lib/nodehub-agent/tls-cert.pem -tls-key /var/lib/nodehub-agent/tls-key.pem
else
    set -- "$@" -allow-http
fi
"$@" >/dev/null
printf 'Agent container started. Endpoint: %s://%s\n' "$MODE" "$LISTEN"
printf 'Token for the panel: %s\n' "$(cat "$STATE/token")"
printf 'Token stored at %s/token (mode 0600). Verify startup with: docker logs %s\n' "$STATE" "$NAME"
