#!/usr/bin/env bash
set -Eeuo pipefail

# Create a consistent, traceable SQLite database backup.
KOMARI_ROOT="${KOMARI_ROOT:-/opt/komari}"
KOMARI_DATA_DIR="${KOMARI_DATA_DIR:-${KOMARI_ROOT}/data}"
KOMARI_BACKUP_DIR="${KOMARI_BACKUP_DIR:-${KOMARI_ROOT}/backups/database}"
KOMARI_COMPOSE_FILE="${KOMARI_COMPOSE_FILE:-${KOMARI_ROOT}/docker-compose.yml}"
RETENTION_COUNT="${RETENTION_COUNT:-7}"

umask 077
command -v sqlite3 >/dev/null 2>&1 || { printf 'sqlite3 is required\n' >&2; exit 1; }
[[ -d "$KOMARI_DATA_DIR" ]] || { printf 'data directory does not exist: %s\n' "$KOMARI_DATA_DIR" >&2; exit 1; }
[[ "$RETENTION_COUNT" =~ ^[1-9][0-9]*$ ]] || { printf 'RETENTION_COUNT must be a positive integer\n' >&2; exit 1; }

mkdir -p "$KOMARI_BACKUP_DIR"
lock_file="${KOMARI_BACKUP_DIR}/.lock"
exec 9>"$lock_file"
flock -n 9 || { printf 'another backup is already running\n' >&2; exit 1; }

stamp="$(date -u +%Y%m%dT%H%M%SZ)"
work_dir="$(mktemp -d "${KOMARI_BACKUP_DIR}/.${stamp}.XXXXXX")"
target_dir="${KOMARI_BACKUP_DIR}/${stamp}"
cleanup() { rm -rf "$work_dir"; }
trap cleanup EXIT
mkdir -p "$work_dir"

# A previous run that died hard (SIGKILL, host power loss) leaves its hidden
# staging directory behind. They are harmless to delete: the backup they were
# building was never published, and a published snapshot is always a
# timestamp-named directory. Match every hidden directory that is not our own
# current staging dir or the lock file, not just this run's stamp prefix.
for stale_work in "${KOMARI_BACKUP_DIR}"/.*; do
  # Skip glob literals and the parent directory: never operate on . or ..
  [[ -d "$stale_work" ]] || continue
  case "$stale_work" in */"."|*/"..") continue ;; esac
  [[ "$stale_work" == "$work_dir" ]] && continue
  [[ -f "${KOMARI_BACKUP_DIR}/.lock" && "$stale_work" == "${KOMARI_BACKUP_DIR}/.lock" ]] && continue
  printf 'removing stale staging directory from an interrupted run: %s\n' "$stale_work" >&2
  rm -rf -- "$stale_work"
done

backup_database() {
  local source="$1" target="$2"
  [[ -f "$source" ]] || { printf 'database does not exist: %s\n' "$source" >&2; return 1; }
  sqlite3 "$source" ".backup '$target'"
  [[ "$(sqlite3 "$target" 'PRAGMA quick_check;')" == 'ok' ]] || { printf 'quick_check failed: %s\n' "$target" >&2; return 1; }
}

backup_database "${KOMARI_DATA_DIR}/komari.db" "${work_dir}/komari.db"
backup_database "${KOMARI_DATA_DIR}/metrics.db" "${work_dir}/metrics.db"

if [[ -r "$KOMARI_COMPOSE_FILE" ]]; then
  cp -- "$KOMARI_COMPOSE_FILE" "${work_dir}/docker-compose.yml"
else
  printf 'optional compose file is not readable; continuing without it: %s\n' "$KOMARI_COMPOSE_FILE" >&2
fi
if [[ -d "${KOMARI_DATA_DIR}/theme" ]]; then
  tar -C "${KOMARI_DATA_DIR}" -czf "${work_dir}/theme.tar.gz" theme
fi
{
  printf 'created_at=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'data_dir=%s\n' "$KOMARI_DATA_DIR"
  printf 'compose_file=%s\n' "$KOMARI_COMPOSE_FILE"
  if command -v docker >/dev/null 2>&1; then
    docker inspect komari --format 'image={{.Config.Image}} image_id={{.Image}}' 2>/dev/null || true
  fi
} > "${work_dir}/backup.info"
files=(komari.db metrics.db backup.info)
[[ -f "${work_dir}/docker-compose.yml" ]] && files+=(docker-compose.yml)
[[ -f "${work_dir}/theme.tar.gz" ]] && files+=(theme.tar.gz)
(cd "$work_dir" && sha256sum -- "${files[@]}") > "${work_dir}/SHA256SUMS"

mv -- "$work_dir" "$target_dir"
trap - EXIT

mapfile -t old_dirs < <(find "$KOMARI_BACKUP_DIR" -mindepth 1 -maxdepth 1 -type d -regextype posix-extended -regex '.*/[0-9]{8}T[0-9]{6}Z' -printf '%p\n' | sort -r)
if (( ${#old_dirs[@]} > RETENTION_COUNT )); then
  for old_dir in "${old_dirs[@]:RETENTION_COUNT}"; do
    rm -rf -- "$old_dir"
  done
fi

# Prune directories this script does not produce. Retention above only counts
# timestamp-named snapshots, so ad-hoc snapshots (pre-*, retention*-*, etc.)
# used to accumulate forever and are never rotated away. A directory that is
# not a timestamp-named snapshot and not hidden (hidden names are this script's
# own staging dirs, already handled above) is an abandoned snapshot.
mapfile -t foreign_dirs < <(find "$KOMARI_BACKUP_DIR" -mindepth 1 -maxdepth 1 \
  -type d -regextype posix-extended \
  ! -regex '.*/[0-9]{8}T[0-9]{6}Z' ! -name '.*' -printf '%p\n' | sort -r)
if (( ${#foreign_dirs[@]} > 0 )); then
  for foreign in "${foreign_dirs[@]}"; do
    printf 'pruning unmanaged backup dir: %s\n' "$foreign" >&2
    rm -rf -- "$foreign"
  done
fi
printf '%s\n' "$target_dir"
