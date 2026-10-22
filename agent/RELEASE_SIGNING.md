# Agent release authentication

The self-updater accepts `agent.update` with the existing `{version, sha256?}`
wire format. It fetches `<binary-download-URL>.sig.json` before the binary.
The controller serves both `komari-agent-linux-{amd64,arm64}.sig.json` and
`.bin.sig.json` (the latter falls back to the canonical sidecar). Existing
controller, ghfast.top, ghproxy.net and direct GitHub candidates remain in order.
Every candidate needs an authentic manifest, including mirrors. Binary downloads
remain streamed, capped at 100 MiB, with a same-directory staging file, SHA256
verification, bounded `--help`, then rename. Manifest reads are capped at 4096
bytes and 30 seconds. Binary downloads retain the existing five-minute deadline.

A manifest is JSON containing only `version`, `asset`, `sha256`, `key_id`, and
`signature`. The signature is standard-base64 Ed25519 over these exact bytes:

```text
komari-agent-release-v1\n<version>\n<asset>\n<lowercase-sha256>\n
```

Version strings are exact: `1.1.8` and `v1.1.8` are distinct signature identities.
The signed asset name is canonical without the `.bin` transport alias. Sign the
final binary **after** compression. `key_id` must match the pinned ID; it cannot
select a key downloaded from a mirror. Rotation requires an authorized binary
carrying the replacement pin. The updater does not trust event-provided hashes,
mirror public keys, or unsigned `.sha256` files as release authorization.

## Trust anchor: baked into the source

The trusted **public** key ships as a source default in
`agent/server/selfupdate_signature.go`:

```go
const (
	defaultTrustedReleasePublicKey = "bDyG4MO5R8KC4fLlC/e2od6O/M9LIvCKi170fDqwtJg="
	defaultTrustedReleaseKeyID     = "komari-release-2026-10"
)
```

A public key is safe to publish, so a plain `go build` — including the existing
`build_all.sh` with no extra flags — already verifies official releases. There is
nothing to inject and nothing to forget. Rotation is a source change plus an
authorized release; `-ldflags -X` still works if you need to pin a different key
ahead of a release:

```sh
-ldflags="-X .../agent/server.TrustedReleasePublicKey=${KEY_B64} -X .../agent/server.TrustedReleaseKeyID=${KEY_ID}"
```

Only the **private** key is secret. Keep it offline, in a file, never in the
repository, never in CI logs. If it leaks, an attacker can sign arbitrary
"authorized" updates — worse than having no signatures, because agents trust it.

## Signing a release

`build_all.sh` signs automatically when you point it at the key:

```sh
KOMARI_RELEASE_KEY=/secure/offline/release-key.pem ./build_all.sh 1.1.8
```

It signs **after** UPX, because compression changes the binary and the manifest
must describe the exact bytes agents download. It emits
`komari-agent-linux-{amd64,arm64}.bin.sig.json` next to each binary.

Without a key the build still succeeds, prints a warning, and produces no
manifests — agents then **refuse** to install it (fail closed). That is the
intended default: an unsigned release is never silently accepted.

To sign one binary by hand, from `agent/`:

```sh
GOMAXPROCS=2 go run -p 1 ./cmd/release-sign \
  -private-key /secure/offline/release-key.pem \
  -input komari-agent-linux-amd64 \
  -out komari-agent-linux-amd64.bin.sig.json \
  -version 1.1.8 -asset komari-agent-linux-amd64 -key-id "komari-release-2026-10"
```

Output must not already exist; the tool cannot overwrite its key or input, and
never prints key material. Publish the sidecar alongside its final binary. Keep
legacy `.sha256` files for bootstrap and old clients. To support both bare and
v-prefixed target events, sign separate manifests in their respective target
version directories; copying one signature to both versions fails.

## Compatibility and verification boundary

The already-installed **1.1.7 updater remains SHA-only for the 1.1.7 -> 1.1.8
bootstrap**. This change cannot retrofit signature verification into old code.
Operators must authenticate that bootstrap through an independent trusted path.
The shell installer remains SHA-only and is not claimed to be covered by Ed25519.
No production assets, GitHub release or fleet broadcast are performed here.

WebSocket messages are limited to 64 KiB including expanded compressed data.
Ping admission permits at most eight active jobs, with no pending queue. Capacity
is acquired before spawning, and held through probe execution and result upload.
Saturation returns rejection, does not ACK or cache the event ID, and permits a
later retry. Successfully admitted duplicate event IDs remain deduplicated. A
normal three-task-per-second workload is exercised for 60 seconds (180 real HTTP
probes and result uploads). Cross builds prove compilation, not native runtime
behavior on ARM, Windows or macOS.
