#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
python_bin="${PYTHON:-python3}"
cd "$repo_root"
exec "$python_bin" -m unittest scripts.tests.test_ci_contract -v
