#!/bin/sh
set -eu

VERSION=$(git describe --tags --abbrev=0 2>/dev/null || printf '%s' dev)
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
done

printf '%s\n' "Binaries are in ./build"
