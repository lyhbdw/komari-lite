#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

THEME_DIR="${ROOT_DIR}/theme"
TARGET_DIR="${ROOT_DIR}/web/public/defaultTheme"

echo "==> Building Komari Lite Theme (Vue 3 + Vite)..."
cd "${THEME_DIR}"

if command -v npm >/dev/null 2>&1; then
    if [ ! -d "node_modules" ]; then
        echo "node_modules not found, running npm ci..."
        npm ci
    fi
    npm run build
elif command -v docker >/dev/null 2>&1; then
    if [ ! -d "node_modules" ]; then
        echo "node_modules not found, running npm ci via docker..."
        docker run --rm -v "${THEME_DIR}:/workspace" -w /workspace node:22-alpine npm ci
    fi
    docker run --rm -v "${THEME_DIR}:/workspace" -w /workspace node:22-alpine npm run build
else
    echo "Error: neither npm nor docker is available" >&2
    exit 1
fi

echo "==> Packaging theme into ${TARGET_DIR}/dist.tar.zst..."
mkdir -p "${TARGET_DIR}"
rm -f "${TARGET_DIR}/dist.tar.zst"
tar -C "${THEME_DIR}/dist" --zstd -cf "${TARGET_DIR}/dist.tar.zst" .
cp -f "${THEME_DIR}/komari-theme.json" "${TARGET_DIR}/"

echo "==> Theme built and packaged successfully."

if [ "${1:-}" = "--sync" ] && [ -d "/opt/komari/data/theme/Lite" ]; then
    echo "==> Syncing to live /opt/komari/data/theme/Lite/dist..."
    mkdir -p /opt/komari/data/theme/Lite/dist
    rm -rf /opt/komari/data/theme/Lite/dist/*
    cp -r "${THEME_DIR}/dist/"* /opt/komari/data/theme/Lite/dist/
    chown -R 986:986 /opt/komari/data/theme/Lite
    echo "==> Live theme cache updated."
fi
