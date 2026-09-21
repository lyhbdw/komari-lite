#!/usr/bin/env bash
set -Eeuo pipefail

# Create a non-destructive, traceable Komari deployment backup.
# Override these variables when the installation uses another layout.
KOMARI_ROOT="${KOMARI_ROOT:-/opt/komari}"
KOMARI_DATA_DIR="${KOMARI_DATA_DIR:-${KOMARI_ROOT}/data}"
KOMARI_BACKUP_DIR="${KOMARI_BACKUP_DIR:-${KOMARI_ROOT}/backups}"
KOMARI_COMPOSE_FILE="${KOMARI_COMPOSE_FILE:-}"

umask 077

if [[ ! -d "$KOMARI_DATA_DIR" ]]; then
  printf 'data directory does not exist: %s\n' "$KOMARI_DATA_DIR" >&2
  exit 1
fi

mkdir -p "$KOMARI_BACKUP_DIR"
work_dir="$(mktemp -d "${KOMARI_BACKUP_DIR}/.komari-backup.XXXXXX")"
archive_path=''
cleanup() {
  rm -rf "$work_dir"
}
trap cleanup EXIT

payload_dir="${work_dir}/payload"
mkdir -p "$payload_dir/data" "$payload_dir/deployment"

# Preserve the complete data tree while excluding the backup output itself.
tar -C "$KOMARI_DATA_DIR" --exclude='./backup' -cf - . | tar -C "$payload_dir/data" -xf -

compose_files=()
if [[ -n "$KOMARI_COMPOSE_FILE" ]]; then
  IFS=: read -r -a requested_compose_files <<< "$KOMARI_COMPOSE_FILE"
  compose_files+=("${requested_compose_files[@]}")
else
  for candidate in compose.yaml compose.yml docker-compose.yaml docker-compose.yml; do
    [[ -f "${KOMARI_ROOT}/${candidate}" ]] && compose_files+=("${KOMARI_ROOT}/${candidate}")
  done
fi

for compose_file in "${compose_files[@]}"; do
  [[ -f "$compose_file" ]] || continue
  cp -- "$compose_file" "${payload_dir}/deployment/$(basename "$compose_file")"
done

if command -v docker >/dev/null 2>&1 && ((${#compose_files[@]} > 0)); then
  compose_args=()
  for compose_file in "${compose_files[@]}"; do
    compose_args+=( -f "$compose_file" )
  done
  if docker compose "${compose_args[@]}" config > "${payload_dir}/deployment/compose.rendered.yaml" 2>"${payload_dir}/deployment/compose.rendered.stderr"; then
    rm -f "${payload_dir}/deployment/compose.rendered.stderr"
  else
    mv "${payload_dir}/deployment/compose.rendered.stderr" "${payload_dir}/deployment/compose.rendered.error.log"
    printf '# docker compose config failed; see compose.rendered.error.log\n' > "${payload_dir}/deployment/compose.rendered.yaml"
  fi
  docker compose "${compose_args[@]}" images > "${payload_dir}/deployment/images.txt" 2>&1 || true
else
  printf 'docker compose metadata unavailable\n' > "${payload_dir}/deployment/images.txt"
fi

if command -v docker >/dev/null 2>&1; then
  {
    printf 'captured_at=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    docker version --format 'server={{.Server.Version}} client={{.Client.Version}}' 2>/dev/null || true
    docker ps --format '{{.Image}}\t{{.Names}}\t{{.ID}}' 2>/dev/null || true
  } > "${payload_dir}/deployment/docker.info"
else
  printf 'docker unavailable\n' > "${payload_dir}/deployment/docker.info"
fi

{
  printf 'created_at=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'komari_root=%s\n' "$KOMARI_ROOT"
  printf 'data_dir=%s\n' "$KOMARI_DATA_DIR"
  printf 'compose_files=%s\n' "${#compose_files[@]}"
} > "${payload_dir}/deployment/backup.info"

( cd "$payload_dir" && find . -type f -print0 | sort -z | xargs -0 sha256sum ) > "${payload_dir}/SHA256SUMS"

stamp="$(date -u +%Y%m%dT%H%M%SZ)"
archive_name="komari-backup-${stamp}-$$.tar.gz"
archive_path="${KOMARI_BACKUP_DIR}/${archive_name}"
temp_archive="${work_dir}/${archive_name}.tmp"
tar -C "$payload_dir" -czf "$temp_archive" .

# Publish atomically. The PID suffix and noclobber guard preserve existing archives.
if [[ -e "$archive_path" ]]; then
  printf 'backup archive already exists: %s\n' "$archive_path" >&2
  exit 1
fi
mv -- "$temp_archive" "$archive_path"
printf '%s  %s\n' "$(sha256sum "$archive_path" | awk '{print $1}')" "$archive_path" > "${archive_path}.sha256"
trap - EXIT
rm -rf "$work_dir"
printf '%s\n' "$archive_path"
