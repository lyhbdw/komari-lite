#!/usr/bin/env bash
set -Eeuo pipefail

# Restore a komari database backup produced by komari-backup.sh.
#
# Usage:
#   ./komari-restore.sh <backup-dir> [--yes]
#
# <backup-dir>  a timestamped snapshot directory under KOMARI_BACKUP_DIR
#               (or any directory containing komari.db / metrics.db + SHA256SUMS)
# --yes         skip the interactive confirmation
#
# Behavior:
#   1. Verify SHA256SUMS (integrity gate; mismatch aborts).
#   2. Run PRAGMA quick_check on both databases (corruption gate).
#   3. Stop the komari container (docker compose down or docker stop).
#   4. Copy current data files aside to a timestamped pre-restore dir.
#   5. Restore komari.db / metrics.db (+ theme/ if archived) into KOMARI_DATA_DIR.
#   6. Start the container again.
#
# Rollbacks live separately from ordinary snapshots. ROLLBACK_RETENTION_COUNT=0
# (default) keeps every rollback for manual recovery; positive values prune
# only this restore script's pre-restore-* copies after a successful restore.

KOMARI_ROOT="${KOMARI_ROOT:-/opt/komari}"
KOMARI_DATA_DIR="${KOMARI_DATA_DIR:-${KOMARI_ROOT}/data}"
KOMARI_BACKUP_DIR="${KOMARI_BACKUP_DIR:-${KOMARI_ROOT}/backups/database}"
KOMARI_COMPOSE_FILE="${KOMARI_COMPOSE_FILE:-${KOMARI_ROOT}/docker-compose.yml}"
KOMARI_ROLLBACK_DIR="${KOMARI_ROLLBACK_DIR:-${KOMARI_ROOT}/backups/rollbacks}"
ROLLBACK_RETENTION_COUNT="${ROLLBACK_RETENTION_COUNT:-0}"
CONTAINER_NAME="${KOMARI_CONTAINER_NAME:-komari}"

log()  { printf '[restore] %s\n' "$*"; }
die()  { printf '[restore] ERROR: %s\n' "$*" >&2; exit 1; }

[[ $# -ge 1 ]] || die "usage: $0 <backup-dir> [--yes]"
backup_dir="$1"
assume_yes=false
[[ "${2:-}" == "--yes" ]] && assume_yes=true

umask 077
[[ "$ROLLBACK_RETENTION_COUNT" =~ ^[0-9]+$ ]] || die "ROLLBACK_RETENTION_COUNT must be a non-negative integer (0 keeps all)"
mkdir -p "$KOMARI_BACKUP_DIR"
command -v flock >/dev/null 2>&1 || die "flock is required"
exec 9>"${KOMARI_BACKUP_DIR}/.lock"
flock -n 9 || die "another backup or restore is already running"
# Hold this lock across unpacking, verification, stop, rollback and start.

# Resolve relative paths against the backup root for convenience. Accepts
# either a plain snapshot directory or a compressed <stamp>.tar.zst archive
# (which is unpacked to a temp dir first).
if [[ ! "$backup_dir" = /* ]]; then
  if [[ -f "${KOMARI_BACKUP_DIR}/${backup_dir}.tar.zst" && ! -d "${KOMARI_BACKUP_DIR}/${backup_dir}" ]]; then
    backup_dir="${KOMARI_BACKUP_DIR}/${backup_dir}.tar.zst"
  else
    backup_dir="${KOMARI_BACKUP_DIR}/${backup_dir}"
  fi
fi
[[ -d "$backup_dir" || -f "$backup_dir" ]] || die "backup does not exist: $backup_dir"
if [[ "$backup_dir" == *.tar.zst ]]; then
  [[ -f "$backup_dir" ]] || die "archive does not exist: $backup_dir"
  unpack_dir="$(mktemp -d "${KOMARI_BACKUP_DIR}/.restore-XXXXXX")"
  trap 'rm -rf -- "$unpack_dir"' EXIT
  log "unpacking $backup_dir"
  tar --zstd -xf "$backup_dir" -C "$unpack_dir"
  backup_dir="$unpack_dir"
fi

[[ -f "$backup_dir/komari.db" ]] || die "komari.db not found in $backup_dir"
[[ -f "$backup_dir/metrics.db" ]] || die "metrics.db not found in $backup_dir"
[[ -f "$backup_dir/SHA256SUMS" ]] || die "SHA256SUMS not found in $backup_dir"
command -v sqlite3 >/dev/null 2>&1 || die "sqlite3 is required"
command -v docker >/dev/null 2>&1 || die "docker is required"
[[ -d "$KOMARI_DATA_DIR" ]] || die "data directory does not exist: $KOMARI_DATA_DIR"

# 1. Integrity gate.
log "verifying SHA256SUMS in $backup_dir"
(cd "$backup_dir" && sha256sum --quiet --check SHA256SUMS) || die "checksum verification failed; backup is corrupt or incomplete"

# 2. Corruption gate.
for db in komari.db metrics.db; do
  log "quick_check $db"
  [[ "$(sqlite3 "$backup_dir/$db" 'PRAGMA quick_check;')" == 'ok' ]] || die "quick_check failed for $db"
done

echo
echo "  backup:  $backup_dir"
if [[ -f "$backup_dir/backup.info" ]]; then
  sed 's/^/  /' "$backup_dir/backup.info"
fi
echo "  target:  $KOMARI_DATA_DIR"
echo "  This will STOP the container, replace the live databases, and restart it."
echo
if [[ "$assume_yes" != true ]]; then
  read -r -p "Type 'restore' to continue: " answer
  [[ "$answer" == "restore" ]] || die "aborted by user"
fi

# 3. Stop the container.
compose_down() {
  if [[ -r "$KOMARI_COMPOSE_FILE" ]] && docker compose -f "$KOMARI_COMPOSE_FILE" ps -q "$CONTAINER_NAME" >/dev/null 2>&1; then
    docker compose -f "$KOMARI_COMPOSE_FILE" stop "$CONTAINER_NAME"
  else
    docker stop "$CONTAINER_NAME"
  fi
}
compose_up() {
  if [[ -r "$KOMARI_COMPOSE_FILE" ]] && docker compose -f "$KOMARI_COMPOSE_FILE" ps -q "$CONTAINER_NAME" >/dev/null 2>&1; then
    docker compose -f "$KOMARI_COMPOSE_FILE" start "$CONTAINER_NAME"
  else
    docker start "$CONTAINER_NAME"
  fi
}

log "stopping container $CONTAINER_NAME"
compose_down

# 4. Side-aside copy of current data (never auto-deleted).
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$KOMARI_ROLLBACK_DIR"
pre_restore="$(mktemp -d "${KOMARI_ROLLBACK_DIR}/pre-restore-${stamp}.XXXXXX")"
for f in komari.db metrics.db; do
  if [[ -f "$KOMARI_DATA_DIR/$f" ]]; then
    cp -- "$KOMARI_DATA_DIR/$f" "$pre_restore/$f"
  fi
done
log "current databases copied to $pre_restore (kept for manual rollback)"

# 5. Restore.
for f in komari.db metrics.db; do
  log "restoring $f"
  install -m 0600 -- "$backup_dir/$f" "$KOMARI_DATA_DIR/$f"
done
if [[ -f "$backup_dir/theme.tar.gz" ]]; then
  log "restoring theme/"
  rm -rf -- "$KOMARI_DATA_DIR/theme"
  tar -C "$KOMARI_DATA_DIR" -xzf "$backup_dir/theme.tar.gz"
fi
# Stale WAL/SHM files from the previous database must not survive.
rm -f -- "$KOMARI_DATA_DIR/komari.db-wal" "$KOMARI_DATA_DIR/komari.db-shm" \
       "$KOMARI_DATA_DIR/metrics.db-wal" "$KOMARI_DATA_DIR/metrics.db-shm"

# 6. Start again.
log "starting container $CONTAINER_NAME"
compose_up

log "done. rollback copy: $pre_restore"

if (( ROLLBACK_RETENTION_COUNT > 0 )); then
  # Reserve one slot for the rollback just created: random suffix order is
  # not creation order when two restores occur in the same second.
  mapfile -t rollbacks < <(find "$KOMARI_ROLLBACK_DIR" -mindepth 1 -maxdepth 1 -type d \
    -regextype posix-extended -regex '.*/pre-restore-[0-9]{8}T[0-9]{6}Z\.[[:alnum:]]{6}' \
    ! -path "$pre_restore" -printf '%T@ %p\n' | sort -nr | cut -d ' ' -f 2-)
  for old_rollback in "${rollbacks[@]:ROLLBACK_RETENTION_COUNT-1}"; do
    rm -rf -- "$old_rollback"
  done
fi
