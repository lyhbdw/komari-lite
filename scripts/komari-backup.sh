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
flock -n 9 || { printf 'another backup or restore is already running\n' >&2; exit 1; }

stamp="$(date -u +%Y%m%dT%H%M%SZ)"
work_dir="$(mktemp -d "${KOMARI_BACKUP_DIR}/.backup-staging-${stamp}.XXXXXX")"
target_dir="${KOMARI_BACKUP_DIR}/${stamp}"
cleanup() { rm -rf "$work_dir"; }
trap cleanup EXIT
mkdir -p "$work_dir"

# Only this script's staging namespace belongs to us. Legacy hidden directories,
# .restore-* unpacking dirs, manual snapshots and pre-restore-* are not ours.
# The shared lock guarantees no live backup is using this namespace.
for stale_work in "${KOMARI_BACKUP_DIR}"/.backup-staging-*; do
  [[ -d "$stale_work" && ! -L "$stale_work" ]] || continue
  [[ "$stale_work" == "$work_dir" ]] && continue
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

# Same-second reruns must never nest staging inside an existing snapshot or
# overwrite an archive. Leave the earlier valid backup intact.
[[ ! -e "$target_dir" && ! -e "${target_dir}.tar.zst" ]] || {
  printf 'snapshot already exists; retry after one second: %s\n' "$target_dir" >&2
  exit 1
}
mv -T -- "$work_dir" "$target_dir"
trap - EXIT

mapfile -t old_dirs < <(find "$KOMARI_BACKUP_DIR" -mindepth 1 -maxdepth 1 -type d -regextype posix-extended -regex '.*/[0-9]{8}T[0-9]{6}Z' -printf '%p\n' | sort -r)
if (( ${#old_dirs[@]} > RETENTION_COUNT )); then
  for old_dir in "${old_dirs[@]:RETENTION_COUNT}"; do
    rm -rf -- "$old_dir"
  done
fi

# Manual snapshots and rollback copies have a separate retention contract;
# never apply RETENTION_COUNT to anything outside timestamped backups.
# Compress snapshots beyond the newest one into single-file zstd archives.
# metrics.db is high-entropy float data: measured ratio is ~56%, not 5:1.
# The newest snapshot stays a plain directory so the common restore path
# needs no unpacking; older ones become <stamp>.tar.zst next to it.
mapfile -t all_snapshots < <(find "$KOMARI_BACKUP_DIR" -mindepth 1 -maxdepth 1 -type d -regextype posix-extended -regex '.*/[0-9]{8}T[0-9]{6}Z' -printf '%p\n' | sort -r)
if (( ${#all_snapshots[@]} > 1 )); then
  for old_snapshot in "${all_snapshots[@]:1}"; do
    archive="${KOMARI_BACKUP_DIR}/$(basename "$old_snapshot").tar.zst"
    [[ -f "$archive" ]] && continue
    printf 'compressing snapshot: %s\n' "$old_snapshot"
    tar -C "$old_snapshot" --zstd -cf "${archive}.tmp" .
    mv -- "${archive}.tmp" "$archive"
    rm -rf -- "$old_snapshot"
  done
fi

# Rotate compressed archives so total snapshots (1 dir + archives) stay at
# RETENTION_COUNT.
mapfile -t old_archives < <(find "$KOMARI_BACKUP_DIR" -mindepth 1 -maxdepth 1 -type f -regextype posix-extended -regex '.*/[0-9]{8}T[0-9]{6}Z\.tar\.zst' -printf '%p\n' | sort -r)
if (( ${#old_archives[@]} > RETENTION_COUNT - 1 )); then
  for old_archive in "${old_archives[@]:RETENTION_COUNT-1}"; do
    rm -f -- "$old_archive"
  done
fi

printf '%s\n' "$target_dir"
