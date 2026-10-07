#!/bin/sh
# Corre scripts/backup.sh todos los días a BACKUP_CRON_HOUR (hora del contenedor, por defecto 3) y hace
# un primer respaldo al arrancar si no hay ninguno de las últimas 24 horas. Lo usa el servicio `backup`
# de docker-compose.yml; en un servidor sin Docker basta con cron llamando a backup.sh.
set -u

HOUR="${BACKUP_CRON_HOUR:-3}"
DIR="${BACKUP_DIR:-/backups}"
HERE="$(dirname "$0")"
case "$HOUR" in ''|*[!0-9]*) HOUR=3 ;; esac
[ "$HOUR" -le 23 ] || HOUR=3

if [ -z "$(find "$DIR" -name 'caresia-db-*.dump' -mtime -1 2>/dev/null | head -n 1)" ]; then
  sh "$HERE/backup.sh" || echo "backup-loop: el respaldo inicial terminó con error $?"
fi

while :; do
  now_h="$(date +%H | sed 's/^0//')"; now_m="$(date +%M | sed 's/^0//')"; now_h="${now_h:-0}"; now_m="${now_m:-0}"
  wait_min=$(( (HOUR * 60 - (now_h * 60 + now_m) + 1440) % 1440 ))
  [ "$wait_min" -gt 0 ] || wait_min=1440
  sleep $((wait_min * 60))
  sh "$HERE/backup.sh" || echo "backup-loop: el respaldo terminó con error $?"
done
