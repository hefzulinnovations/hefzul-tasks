#!/usr/bin/env bash
# Runs backup.sh once a day at BACKUP_TIME (HH:MM, in TZ).
set -euo pipefail

BACKUP_TIME="${BACKUP_TIME:-02:30}"

while true; do
	now=$(date +%s)
	next=$(date -d "today ${BACKUP_TIME}" +%s)
	if [ "$next" -le "$now" ]; then
		next=$(date -d "tomorrow ${BACKUP_TIME}" +%s)
	fi
	echo "Next backup at $(date -d "@${next}" '+%Y-%m-%d %H:%M %Z')"
	sleep $((next - now))
	if ! /usr/local/bin/backup.sh; then
		echo "Backup FAILED at $(date '+%Y-%m-%d %H:%M %Z')" >&2
	fi
done
