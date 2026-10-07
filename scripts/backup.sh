#!/bin/sh
# Respaldo de Caresia: base de datos (pg_dump, formato custom comprimido) + archivos adjuntos (tar.gz).
# POSIX sh: corre igual en el contenedor postgres:16-alpine que en un servidor con cron.
#
# Variables (todas opcionales salvo la conexión a la base):
#   DATABASE_URL           conexión a PostgreSQL (o las variables PGHOST/PGUSER/PGPASSWORD/PGDATABASE)
#   BACKUP_DIR             dónde guardar los respaldos (por defecto ./backups)
#   UPLOADS_DIR            carpeta de archivos adjuntos (por defecto data/uploads; si no existe se omite)
#   BACKUP_RETENTION_DAYS  días que se conservan los respaldos (por defecto 14)
#   BACKUP_KEEP            cantidad mínima de respaldos que nunca se borran (por defecto 7)
#   BACKUP_RCLONE_REMOTE   destino de copia externa, p. ej. "r2:caresia-respaldos" (requiere rclone configurado)
#   BACKUP_STATUS_FILE     archivo JSON con el resultado del último respaldo (lo lee /api/health)
#
# Códigos de salida: 0 todo bien · 1 falló el volcado · 2 el respaldo no pasó la verificación
#                    3 el respaldo local está bien pero falló la copia externa · 4 configuración inválida
set -eu

BACKUP_DIR="${BACKUP_DIR:-./backups}"
UPLOADS_DIR="${UPLOADS_DIR:-data/uploads}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"
KEEP="${BACKUP_KEEP:-7}"
REMOTE="${BACKUP_RCLONE_REMOTE:-}"
STATUS_FILE="${BACKUP_STATUS_FILE:-}"

log() { printf '%s backup: %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"; }

case "$RETENTION_DAYS$KEEP" in *[!0-9]*) log "BACKUP_RETENTION_DAYS y BACKUP_KEEP deben ser números enteros"; exit 4 ;; esac
if [ -z "${DATABASE_URL:-}" ] && [ -z "${PGHOST:-}" ] && [ -z "${PGDATABASE:-}" ]; then
  log "falta DATABASE_URL (o PGHOST/PGDATABASE)"; exit 4
fi

started="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
report_file=""; db_file=""; up_file=""; db_size=0; total=0; step="inicio"; uploaded="false"

write_status() { # $1 estado (ok|warn|failed) · $2 mensaje
  [ -n "$STATUS_FILE" ] || return 0
  tmp="$STATUS_FILE.tmp"
  printf '{"status":"%s","started_at":"%s","finished_at":"%s","file":"%s","size_bytes":%s,"uploaded":%s,"message":"%s"}\n' \
    "$1" "$started" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$report_file" "$total" "$uploaded" "$2" >"$tmp" 2>/dev/null \
    && mv "$tmp" "$STATUS_FILE" || true
}

fail() { # $1 código · $2 mensaje
  log "ERROR: $2"
  [ -n "$db_file" ] && rm -f "$db_file.partial"
  [ -n "$up_file" ] && rm -f "$up_file.partial"
  write_status failed "$2"
  exit "$1"
}

mkdir -p "$BACKUP_DIR"
stamp="$(date -u +%Y%m%d-%H%M%S)"
db_file="$BACKUP_DIR/caresia-db-$stamp.dump"

# 1. Base de datos. Se escribe a .partial y solo se renombra cuando pasó la verificación.
step="pg_dump"
log "volcando la base de datos…"
if [ -n "${DATABASE_URL:-}" ]; then
  pg_dump --format=custom --compress=6 --no-owner --file="$db_file.partial" "$DATABASE_URL" || fail 1 "pg_dump falló"
else
  pg_dump --format=custom --compress=6 --no-owner --file="$db_file.partial" || fail 1 "pg_dump falló"
fi
step="verificación"
pg_restore --list "$db_file.partial" >/dev/null 2>&1 || fail 2 "el volcado no se puede leer con pg_restore --list"
[ "$(wc -c <"$db_file.partial")" -gt 1024 ] || fail 2 "el volcado es sospechosamente pequeño"
mv "$db_file.partial" "$db_file"
db_size="$(wc -c <"$db_file")"; report_file="$(basename "$db_file")"
total="$db_size"

# 2. Archivos adjuntos (ya están cifrados en disco; el tar no los toca).
up_file=""
if [ -d "$UPLOADS_DIR" ] && [ -n "$(ls -A "$UPLOADS_DIR" 2>/dev/null)" ]; then
  up_file="$BACKUP_DIR/caresia-uploads-$stamp.tar.gz"
  log "empaquetando archivos adjuntos…"
  tar -czf "$up_file.partial" -C "$UPLOADS_DIR" . || fail 1 "tar de adjuntos falló"
  tar -tzf "$up_file.partial" >/dev/null 2>&1 || fail 2 "el tar de adjuntos no se puede leer"
  mv "$up_file.partial" "$up_file"
  total=$((total + $(wc -c <"$up_file")))
else
  log "sin carpeta de adjuntos con contenido en $UPLOADS_DIR: se omite"
fi
log "respaldo local listo: $(basename "$db_file") ($db_size bytes)"

# 3. Rotación: se borra lo que pasa de BACKUP_RETENTION_DAYS, pero siempre quedan los BACKUP_KEEP más recientes.
rotate() { # $1 patrón
  # shellcheck disable=SC2012
  ls -1t "$BACKUP_DIR"/$1 2>/dev/null | tail -n +$((KEEP + 1)) | while read -r old; do
    if [ -n "$(find "$old" -mtime +"$RETENTION_DAYS" 2>/dev/null)" ]; then
      rm -f "$old" && log "rotado: $(basename "$old")"
    fi
  done
}
rotate 'caresia-db-*.dump'
rotate 'caresia-uploads-*.tar.gz'

# 4. Copia externa (opcional). Un fallo aquí no invalida el respaldo local, pero sale con código 3.
if [ -n "$REMOTE" ]; then
  step="copia externa"
  if ! command -v rclone >/dev/null 2>&1; then
    log "AVISO: BACKUP_RCLONE_REMOTE está configurado pero rclone no está instalado"
    write_status warn "respaldo local correcto; rclone no está instalado"; exit 3
  fi
  if rclone copy "$db_file" "$REMOTE" && { [ -z "$up_file" ] || rclone copy "$up_file" "$REMOTE"; }; then
    uploaded="true"
    log "copia externa lista en $REMOTE"
  else
    log "AVISO: la copia externa falló"
    write_status warn "respaldo local correcto; falló la copia externa"; exit 3
  fi
fi

write_status ok ""
log "listo"
