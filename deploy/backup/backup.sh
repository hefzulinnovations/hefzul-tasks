#!/usr/bin/env bash
# Writes one backup to /backups/<timestamp>/: db.dump (pg_dump custom format),
# files.tar.gz (attachments and backgrounds) and SHA256SUMS. Then prunes old
# local backups and, if OFFSITE_REMOTE is set, copies the backup off-site.
set -euo pipefail

BACKUP_ROOT="${BACKUP_ROOT:-/backups}"
FILES_DIR="${FILES_DIR:-/data/files}"
KEEP_DAYS="${BACKUP_KEEP_DAYS:-14}"

stamp=$(date +%Y-%m-%dT%H%M)
target="${BACKUP_ROOT}/${stamp}"
partial="${target}.partial"

rm -rf "$partial"
mkdir -p "$partial"

echo "[$(date -Is)] Dumping database ${PGDATABASE}"
pg_dump --format=custom --no-owner --file="${partial}/db.dump"

echo "[$(date -Is)] Archiving files"
tar -C "$FILES_DIR" -czf "${partial}/files.tar.gz" .

(cd "$partial" && sha256sum db.dump files.tar.gz > SHA256SUMS)
mv "$partial" "$target"
echo "[$(date -Is)] Backup written to ${target} ($(du -sh "$target" | cut -f1))"

find "$BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -mtime +"$KEEP_DAYS" -print -exec rm -rf {} +

if [ -n "${OFFSITE_REMOTE:-}" ]; then
	echo "[$(date -Is)] Copying to ${OFFSITE_REMOTE}/${stamp}"
	rclone copy "$target" "${OFFSITE_REMOTE}/${stamp}"
	rclone delete --min-age "${KEEP_DAYS}d" "$OFFSITE_REMOTE"
	rclone rmdirs --leave-root "$OFFSITE_REMOTE"
fi

echo "[$(date -Is)] Done"
