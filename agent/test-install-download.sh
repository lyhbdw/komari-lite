#!/bin/sh
set -eu

script=${1:-./install.sh}

require() {
    pattern=$1
    description=$2
    if ! grep -F -- "$pattern" "$script" >/dev/null; then
        printf 'missing installer invariant: %s\n' "$description" >&2
        exit 1
    fi
}

require 'download_base="${AGENT_DOWNLOAD_BASE:-}"' 'environment default for controller download base'
require '--download-base)' 'download-base argument parser'
require 'https://*) ;;' 'HTTPS-only download base allow rule'
require 'log_error "--download-base must use HTTPS"' 'non-HTTPS rejection'
require 'download_url="${download_base%/}/${file_name}"' 'fixed filename appended to download base'
require 'checksum_url="${download_url}.sha256"' 'same-origin per-file checksum URL'
require 'SHA256 verification failed' 'mandatory checksum verification'

printf 'installer download invariants: ok\n'
