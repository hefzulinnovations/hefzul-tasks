#!/usr/bin/env bash
# Restores a backup written by backup.sh. Stop the app first:
#   docker compose stop app
#   docker compose run --rm backup restore.sh 2026-10-14T0230
#   docker compose run --rm backup restore.sh --from-offsite 2026-10-14T0230
#   docker compose start app
# Without a name it lists the available backups.
set -euo pipefail

BACKUP_ROOT="${BACKUP_ROOT:-/backups}"
FILES_DIR="${FILES_DIR:-/data/files}"

from_offsite=false
if [ "${1:-}" = "--from-offsite" ]; then
	from_offsite=true
	shift
fi
name="${1:-}"

if [ -z "$name" ]; then
	echo "Local backups:"
	ls -1 "$BACKUP_ROOT" | grep -v '\.partial$' || true
	if [ -n "${OFFSITE_REMOTE:-}" ]; then
		echo "Off-site backups (${OFFSITE_REMOTE}):"
		rclone lsf --dirs-only "$OFFSITE_REMOTE" || true
	fi
	echo "Usage: restore.sh [--from-offsite] <backup-name>"
	exit 1
fi

case "$name" in
	*/* | *..*) echo "Invalid backup name: $name" >&2; exit 1 ;;
esac

source_dir="${BACKUP_ROOT}/${name}"
if $from_offsite; then
	: "${OFFSITE_REMOTE:?OFFSITE_REMOTE is not set}"
	source_dir=$(mktemp -d)
	echo "Downloading ${OFFSITE_REMOTE}/${name}"
	rclone copy "${OFFSITE_REMOTE}/${name}" "$source_dir"
fi

if [ ! -f "${source_dir}/db.dump" ]; then
	echo "No backup found at ${source_dir}" >&2
	exit 1
fi

echo "Verifying checksums"
(cd "$source_dir" && sha256sum -c SHA256SUMS)

echo "Restoring database ${PGDATABASE} (existing data is replaced)"
pg_restore --clean --if-exists --no-owner --dbname="$PGDATABASE" "${source_dir}/db.dump"

echo "Restoring files"
find "$FILES_DIR" -mindepth 1 -delete
tar -C "$FILES_DIR" -xzf "${source_dir}/files.tar.gz"
chown -R 1000:1000 "$FILES_DIR"

echo "Restore of ${name} complete. Start the app with: docker compose start app"
