#!/bin/sh
set -eu

VERSION=$(git describe --tags --abbrev=0 2>/dev/null || printf '%s' dev)
mkdir -p ./build

for GOARCH in amd64 arm64; do
  echo "Building for linux/$GOARCH..."
  GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -trimpath \
      -ldflags="-X github.com/komari-monitor/komari-agent/update.CurrentVersion=${VERSION}" \
      -o "./build/komari-agent-linux-${GOARCH}"
done

printf '%s\n' "Binaries are in ./build"
