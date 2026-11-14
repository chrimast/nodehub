#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname "$0")/../../.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir "$tmp/bin"
printf '%s\n' '#!/bin/sh' 'printf "%s\n" "$*" >> "$DOCKER_LOG"' 'case "$1" in inspect) exit 1;; esac' > "$tmp/bin/docker"
chmod +x "$tmp/bin/docker"
export PATH="$tmp/bin:$PATH" DOCKER_LOG="$tmp/docker.log"
export NODEHUB_AGENT_DIR="$tmp/state"

sh "$root/cmd/nodehub-agent/install.sh" --image example/agent:test --http > "$tmp/output"
test "$(stat -c %a "$tmp/state/token")" = 600
test "$(wc -c < "$tmp/state/token")" -ge 32
grep -F -- "$(cat "$tmp/state/token")" "$tmp/output" >/dev/null
grep -F -- '-allow-http' "$tmp/docker.log" >/dev/null
grep -F -- '--restart unless-stopped' "$tmp/docker.log" >/dev/null
grep -F -- '127.0.0.1:9443' "$tmp/docker.log" >/dev/null
token=$(cat "$tmp/state/token")
sh "$root/cmd/nodehub-agent/install.sh" --image example/agent:test --http > /dev/null
test "$(cat "$tmp/state/token")" = "$token"
if sh "$root/cmd/nodehub-agent/install.sh" --image example/agent:test > /dev/null 2>&1; then
  printf '%s\n' 'HTTPS without certificate unexpectedly succeeded' >&2
  exit 1
fi
printf '%s\n' cert > "$tmp/cert"
printf '%s\n' key > "$tmp/key"
sh "$root/cmd/nodehub-agent/install.sh" --image example/agent:test --tls-cert "$tmp/cert" --tls-key "$tmp/key" > "$tmp/https-output"
grep -F -- '-tls-cert /var/lib/nodehub-agent/tls-cert.pem' "$tmp/docker.log" >/dev/null
grep -F -- '-tls-key /var/lib/nodehub-agent/tls-key.pem' "$tmp/docker.log" >/dev/null
grep -F -- 'https://127.0.0.1:9443' "$tmp/https-output" >/dev/null
printf '%s\n' 'installer tests passed'