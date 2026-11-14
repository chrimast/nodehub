# nodehub-agent Docker deployment

This standalone binary supports the panel's agent probe but is not compatible
with the 3x-ui remote API. The Docker image and installer in this directory
package the existing CLI without changing other modules. The agent does **not**
implement traffic quotas; those remain unsupported.

## Build

From the repository root:

```sh
docker build -f cmd/nodehub-agent/Dockerfile -t nodehub-agent:local .
```

The image pins Xray `26.10.31` and builds `cmd/nodehub-agent` from this checkout.
Do not put panel tokens in Dockerfiles, image layers, shell history, or Git.

## Remote install

Copy `install.sh` to the remote host, inspect it, then run it with an image
you built locally or published yourself. The suggested Compose image name is a
placeholder and publication is not guaranteed:

```sh
sh install.sh --image ghcr.io/your-org/nodehub-agent:VERSION \
  --tls-cert /etc/nodehub-agent/tls/fullchain.pem \
  --tls-key /etc/nodehub-agent/tls/privkey.pem
```

Defaults are conservative: HTTPS, `127.0.0.1:9443`, state at
`/var/lib/nodehub-agent` (mode `0700`), and a generated 256-bit token at
`state/token` (mode `0600`). The script refuses to replace an existing container;
upgrades are explicit. It uses Docker `unless-stopped`, a read-only root filesystem,
a tmpfs `/tmp`, dropped capabilities, and `no-new-privileges`. Host networking is
intentional because Xray may open inbound ports from submitted configuration.

The script generates a bearer token on first install and prints it on each successful
install for entry in the panel. Re-running with the same state directory preserves it. The token
is read with `-token-file`, not passed as an environment variable. Treat it as an
administrative credential.

Installation prints container creation, not application readiness. Check
`docker logs nodehub-agent` and probe the authenticated `/v1/status` endpoint.
The script copies the TLS certificate and key into the private state directory;
certificate renewal requires updating those copies and restarting the container.
The state directory and its token are owned by container UID 10001 when installed
as root. Use a dedicated, trusted directory, never a source checkout.

HTTPS is required unless `--http` is explicitly supplied. HTTP is only suitable for
a trusted encrypted/private network because it exposes the token and configuration:

```sh
sh install.sh --image ghcr.io/your-org/nodehub-agent:VERSION --http
```

Use `--listen 0.0.0.0:9443` only with an appropriate firewall and TLS policy.

## Compose example

See the [Chinese deployment instructions](../../README.zh_CN.md#远程-nodehub-agent实验性) for
state initialization, image build/transfer, and panel connection. The example
requires an explicit `https` or `http` profile; never run both against the same
state directory or listener. It does not generate a token or copy certificates.

## Supported agent API

The real CLI flags are `-listen`, `-allow-http`, `-tls-cert`, `-tls-key`, `-token-file`,
`-identity-file`, `-config-file`, and `-xray`. The API includes authenticated
`GET /v1/status`, `GET /v1/capabilities`, `POST /v1/config`, and `POST /v1/disable`.
`POST /v1/config` accepts `{ "config_version": N, "config": { ... } }` and `POST /v1/disable` accepts `{ "config_version": N }`. `config_version` is a positive signed 64-bit integer allocated monotonically by the panel per agent; comparison is numeric. A lower version, or the current version paired with a different operation/payload, returns HTTP 409. An identical successfully applied request returns the same success without reapplying Xray; an identical pending request is retried. The agent persists the version and request fingerprint before activation to fence delayed older requests even after an interrupted operation. Therefore validation or activation failure also reserves that version: retry the identical request or send a higher version. State lives in `<config-file>.version` (mode 0600) and survives restart. During migration, requests without a version are accepted only until the first versioned request; thereafter they are rejected. Existing installations need no state-file initialization. The panel uses fencing only when the agent advertises `config_version: true`; older agents retain their legacy API and cannot prevent delayed requests from overwriting newer configuration. The endpoint validates with `xray run -test`, atomically persists the accepted configuration, and starts the managed Xray process. Status reports agent readiness and
child-process state, not a traffic quota or functional proxy health check.

Run the shell installer and Compose static tests with:
`sh cmd/nodehub-agent/tests/install-test.sh` and
`sh cmd/nodehub-agent/tests/compose-test.sh`.

For direct CLI use, it requires an operator-provided TLS certificate and key, a private bearer token
file containing at least 32 random characters, a private identity file path,
an absolute Xray binary path, and a dedicated Xray config path. The key, token,
and existing identity must be regular files inaccessible to group and others;
the generated identity is 32 random bytes stored with mode 0600. Keep all paths
distinct in an operator-owned directory inaccessible to untrusted users. Default
listen address is `127.0.0.1:9443`.

Use `-allow-http` to explicitly serve unencrypted HTTP instead, without TLS
certificate flags. HTTP exposes bearer credentials and configuration to anyone
who can observe or modify network traffic; use it only on a trusted network or
behind a trusted encrypted tunnel. HTTPS remains the default and recommended mode.

Run `nodehub-agent -help` for flags. HTTPS clients must verify the TLS certificate and
send an `Authorization: Bearer <token>` header on every request. Authenticated
`GET /v1/status` returns a persistent node ID, agent readiness and `xray_status`
(`running` or `stopped`), reflecting whether the managed child has exited, not
a functional health check. `GET /v1/capabilities` advertises config saving and
activation, explicit disable, but not runtime health. `POST /v1/config` accepts a JSON object of the form
`{"config":{"outbounds":[{"protocol":"freedom"}]}}`. It rejects unknown
envelope fields and unknown top-level Xray sections, limits the request to 1 MiB,
serializes changes, writes a 0600 temporary file beside the target, and runs
the configured binary as `xray run -test -c <temporary-file>` with a 15-second
timeout. It stops the managed child, starts Xray with the candidate, waits 500ms
for an early exit, and then atomically renames the candidate into place. On
activation or rename failure it attempts to restart the previous config, which
remains on disk. Recovery can itself fail, leaving Xray stopped. Operations are
serialized in this agent process. An existing private config is validated and
started at agent startup; SIGINT/SIGTERM causes graceful child shutdown, with a
bounded wait and forced termination. An unexpected child exit is reflected in
status; there is no automatic restart, but a later apply or agent restart can
start it again. Xray output is discarded to avoid logging config secrets.

`POST /v1/disable` accepts `{"config_version":N}` and atomically creates a private
`<config-file>.disabled` marker and syncs its directory before stopping Xray;
startup honors that marker even if the last config remains on disk. Failed
marker persistence does not stop the current process. A later successful
`POST /v1/config` activates the candidate, replaces the saved config, then
removes the marker, so invalid configs retain the previous state. This endpoint
is the only supported empty-scope operation; empty `inbounds` in `/v1/config`
is not interpreted as a disable request.

The bearer token grants full authority to submit Xray configuration, including
network and filesystem effects that Xray supports. Treat it as an administrative
credential, run the agent and Xray with least privilege, use trusted directory
ownership, and do not expose it publicly without additional access controls.
Xray validates config semantics; nested options are not independently audited
by this agent. Unknown nested Xray options may be ignored by Xray itself.

This is experimental, not production-ready. A 500ms survival window does not
prove readiness or prevent later crashes; rollback cannot guarantee recovery if
resources become unavailable. There is downtime during replacement. There is no
automatic crash supervision, durable backup, enrollment, credential rotation,
rate limiting, multi-process locking, or panel config sync. Agent SIGKILL or
host failure is not graceful shutdown and may leave a child running. Atomic
rename does not guarantee directory durability across power loss, and a crash
around activation can leave runtime and disk temporarily divergent. Use a
dedicated trusted directory and a single agent instance. Relative asset paths
and Xray's geo data resolution depend on the runtime environment.

Run tests with `go test ./cmd/nodehub-agent`; to exercise a real Xray executable,
set `XRAY_E2E_BINARY` to its absolute path before running the tests.