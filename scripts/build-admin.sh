#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

FRONTEND_DIR="${ROOT_DIR}/frontend"
TARGET_DIR="${ROOT_DIR}/web/public/defaultTheme"

echo "==> Building Komari Admin Frontend (React + Vite)..."
cd "${FRONTEND_DIR}"

if command -v npm >/dev/null 2>&1; then
    if [ ! -d "node_modules" ]; then
        echo "node_modules not found, running npm ci..."
        npm ci
    fi
    npm run build
elif command -v docker >/dev/null 2>&1; then
    if [ ! -d "node_modules" ]; then
        echo "node_modules not found, running npm ci via docker..."
        docker run --rm -v "${FRONTEND_DIR}:/workspace" -w /workspace node:22-alpine npm ci
    fi
    docker run --rm -v "${FRONTEND_DIR}:/workspace" -w /workspace node:22-alpine npm run build
else
    echo "Error: neither npm nor docker is available" >&2
    exit 1
fi

echo "==> Packaging admin frontend into ${TARGET_DIR}/admin-dist.tar.zst..."
mkdir -p "${TARGET_DIR}"
rm -f "${TARGET_DIR}/admin-dist.tar.zst"
tar -C "${FRONTEND_DIR}/dist" --zstd -cf "${TARGET_DIR}/admin-dist.tar.zst" .

echo "==> Admin frontend built and packaged successfully."
