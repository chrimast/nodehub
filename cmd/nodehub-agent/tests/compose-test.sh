#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/../../.." && pwd)
compose="$root/cmd/nodehub-agent/docker-compose.yml.example"
command -v docker >/dev/null 2>&1 || { echo 'Docker is required.' >&2; exit 1; }

sh -n "$root/cmd/nodehub-agent/install.sh"
sh -n "$0"

https=$(mktemp)
http=$(mktemp)
override=$(mktemp)
trap 'rm -f "$https" "$http" "$override"' EXIT

docker compose -f "$compose" --profile https config >"$https"
docker compose -f "$compose" --profile http config >"$http"
NODEHUB_AGENT_IMAGE=example/agent:test \
NODEHUB_AGENT_DIR=/srv/nodehub-agent \
NODEHUB_AGENT_LISTEN=10.0.0.8:9443 \
docker compose -f "$compose" --profile https config >"$override"

grep -F 'image: ghcr.io/chrimast/nodehub-xui-agent:latest' "$https" >/dev/null
grep -F 'network_mode: host' "$https" >/dev/null
grep -F 'read_only: true' "$https" >/dev/null
grep -F 'no-new-privileges:true' "$https" >/dev/null
grep -F 'source: /var/lib/nodehub-agent' "$https" >/dev/null
grep -F 'target: /var/lib/nodehub-agent' "$https" >/dev/null
grep -F -- '-token-file' "$https" >/dev/null
grep -F -- '-identity-file' "$https" >/dev/null
grep -F -- '-config-file' "$https" >/dev/null
grep -F -- '/var/lib/nodehub-agent/config.json' "$https" >/dev/null
grep -F -- '-xray' "$https" >/dev/null
grep -F -- '/usr/local/bin/xray' "$https" >/dev/null
grep -F -- '-tls-cert' "$https" >/dev/null
grep -F -- '-tls-key' "$https" >/dev/null
grep -F -- '-allow-http' "$http" >/dev/null
if grep -F -- '-tls-cert' "$http" >/dev/null; then
    echo 'HTTP profile unexpectedly includes TLS flags' >&2
    exit 1
fi
grep -F 'image: example/agent:test' "$override" >/dev/null
grep -F 'source: /srv/nodehub-agent' "$override" >/dev/null
grep -F '10.0.0.8:9443' "$override" >/dev/null

printf '%s\n' 'compose static tests passed'
