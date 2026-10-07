#!/bin/sh
# Restaura un respaldo de Caresia (base de datos y, si se indica, archivos adjuntos).
#
#   scripts/restore.sh [--yes] [--latest | ARCHIVO.dump] [ARCHIVO-uploads.tar.gz]
#
# Destino:  RESTORE_DATABASE_URL (si no existe, DATABASE_URL). Para PROBAR un respaldo sin tocar producción,
#           apunta RESTORE_DATABASE_URL a una base vacía distinta y RESTORE_UPLOADS_DIR a una carpeta temporal.
# Peligro:  la restauración REEMPLAZA los datos de la base destino. Pide escribir RESTAURAR para continuar;
#           --yes salta la pregunta (solo para automatizar pruebas).
# Códigos:  0 bien · 1 falló · 4 uso incorrecto o cancelado
set -eu

BACKUP_DIR="${BACKUP_DIR:-./backups}"
TARGET="${RESTORE_DATABASE_URL:-${DATABASE_URL:-}}"
UPLOADS_TARGET="${RESTORE_UPLOADS_DIR:-${UPLOADS_DIR:-data/uploads}}"
yes=0; dump=""; uploads=""

while [ $# -gt 0 ]; do
  case "$1" in
    --yes) yes=1 ;;
    --latest)
      # shellcheck disable=SC2012
      dump="$(ls -1t "$BACKUP_DIR"/caresia-db-*.dump 2>/dev/null | head -n 1 || true)"
      [ -n "$dump" ] || { echo "No hay respaldos en $BACKUP_DIR" >&2; exit 4; }
      up="$(echo "$dump" | sed 's/caresia-db-/caresia-uploads-/; s/\.dump$/.tar.gz/')"
      [ -f "$up" ] && uploads="$up"
      ;;
    -h|--help) sed -n '2,11p' "$0"; exit 0 ;;
    *.dump) dump="$1" ;;
    *.tar.gz) uploads="$1" ;;
    *) echo "Argumento no reconocido: $1" >&2; exit 4 ;;
  esac
  shift
done

[ -n "$dump" ] && [ -f "$dump" ] || { echo "Indica el archivo .dump o usa --latest." >&2; exit 4; }
[ -n "$TARGET" ] || { echo "Falta RESTORE_DATABASE_URL (o DATABASE_URL)." >&2; exit 4; }
pg_restore --list "$dump" >/dev/null 2>&1 || { echo "El archivo $dump no es un respaldo válido." >&2; exit 1; }

# Se muestra el destino sin contraseña.
shown="$(printf '%s' "$TARGET" | sed 's#//[^@/]*@#//***@#')"
echo "Respaldo:  $dump"
[ -n "$uploads" ] && echo "Adjuntos:  $uploads  ->  $UPLOADS_TARGET"
echo "Destino:   $shown"
echo "Esto reemplaza los datos de la base destino."
if [ "$yes" -ne 1 ]; then
  printf 'Escribe RESTAURAR para continuar: '
  read -r answer
  [ "$answer" = "RESTAURAR" ] || { echo "Cancelado."; exit 4; }
fi

echo "Restaurando la base de datos…"
pg_restore --clean --if-exists --no-owner --exit-on-error --dbname="$TARGET" "$dump" || { echo "pg_restore falló." >&2; exit 1; }

if [ -n "$uploads" ]; then
  echo "Restaurando adjuntos…"
  mkdir -p "$UPLOADS_TARGET"
  tar -xzf "$uploads" -C "$UPLOADS_TARGET" || { echo "No se pudieron extraer los adjuntos." >&2; exit 1; }
fi

echo "Verificación rápida (conteos):"
psql "$TARGET" -At -F ': ' -c "
  SELECT 'clínicas', count(*) FROM clinics UNION ALL
  SELECT 'usuarios', count(*) FROM users UNION ALL
  SELECT 'pacientes', count(*) FROM patients UNION ALL
  SELECT 'bitácora', count(*) FROM encounters UNION ALL
  SELECT 'recetas', count(*) FROM prescriptions UNION ALL
  SELECT 'archivos', count(*) FROM attachments UNION ALL
  SELECT 'migraciones', count(*) FROM schema_migrations" || echo "(no se pudo consultar; revisa la base manualmente)"
echo "Restauración terminada. Reinicia la aplicación para que tome los datos nuevos."
