#!/bin/sh
# Builds release agents and optionally signs them for verified self-update.
#
# The agent trusts releases by a PUBLIC key baked into the source, so a normal
# build needs no flags at all and auto-update works out of the box. Signing is
# what makes a release *installable*: without a manifest next to the binary,
# agents refuse to update (fail closed).
#
# Usage:
#   ./build_all.sh [version]
#   KOMARI_RELEASE_KEY=/path/to/release.key ./build_all.sh [version]
#
# Signing runs AFTER UPX, because compression changes the binary and the
# manifest must describe the exact bytes agents will download.
set -eu

VERSION="${1:-$(git describe --tags --abbrev=0 2>/dev/null || printf '%s' dev)}"
CLEAN_VERSION="${VERSION#v}"
KEY="${KOMARI_RELEASE_KEY:-}"
KEY_ID="${KOMARI_RELEASE_KEY_ID:-komari-release-2026-10}"

if [ -z "$KEY" ]; then
  for candidate in "/root/.komari-release/release.key" "${HOME}/.komari-release/release.key" "./release.key"; do
    if [ -f "$candidate" ]; then
      KEY="$candidate"
      echo "Auto-detected release key: $KEY"
      break
    fi
  done
fi

if [ -n "$KEY" ] && [ ! -f "$KEY" ]; then
  printf '%s\n' "build_all: KOMARI_RELEASE_KEY=$KEY does not exist" >&2
  exit 1
fi

mkdir -p ./build

for GOARCH in amd64 arm64; do
  echo "Building for linux/$GOARCH..."
  GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -trimpath \
      -ldflags="-s -w -X github.com/lyhbdw/komari-lite/agent/version.Current=${VERSION}" \
      -o "./build/komari-agent-linux-${GOARCH}"
  if command -v upx >/dev/null 2>&1; then
    echo "Compressing with UPX for linux/$GOARCH..."
    upx --lzma "./build/komari-agent-linux-${GOARCH}" || true
  fi
  (cd ./build && sha256sum "komari-agent-linux-${GOARCH}" > "komari-agent-linux-${GOARCH}.sha256")
  cp "./build/komari-agent-linux-${GOARCH}.sha256" "./build/komari-agent-linux-${GOARCH}.bin.sha256"

  if [ -n "$KEY" ]; then
    echo "Signing linux/$GOARCH..."
    # -out uses O_EXCL: re-running never silently overwrites a manifest.
    rm -f "./build/komari-agent-linux-${GOARCH}.sig.json" "./build/komari-agent-linux-${GOARCH}.bin.sig.json"
    # The signer runs on the host architecture: it hashes and signs files, it
    # never executes the target binary. GOOS/GOARCH must be reset explicitly
    # because the loop still exports the target values.
    GOOS="$(go env GOHOSTOS)" GOARCH="$(go env GOHOSTARCH)" CGO_ENABLED=0 \
      go run ./cmd/release-sign \
      -private-key "$KEY" \
      -input "./build/komari-agent-linux-${GOARCH}" \
      -out "./build/komari-agent-linux-${GOARCH}.sig.json" \
      -version "$CLEAN_VERSION" \
      -asset "komari-agent-linux-${GOARCH}" \
      -key-id "$KEY_ID"
    cp "./build/komari-agent-linux-${GOARCH}.sig.json" "./build/komari-agent-linux-${GOARCH}.bin.sig.json"
  fi
done

if [ -z "$KEY" ]; then
  printf '%s\n' "" "NOTE: no release key found, so no manifests were signed." \
    "Agents will REFUSE to update to this build (fail closed)." \
    "To make it installable, place key at /root/.komari-release/release.key or pass:" \
    "KOMARI_RELEASE_KEY=/path/to/release.key ./build_all.sh $VERSION"
fi

printf '%s\n' "" "Binaries are in ./build"
