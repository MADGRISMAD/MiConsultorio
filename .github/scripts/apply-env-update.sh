#!/usr/bin/env bash
# Se ejecuta EN EL SERVIDOR: mezcla /tmp/caresia-new.env en ~/miconsultorio/.env y reinicia la app.
set -euo pipefail
cd ~/miconsultorio
new=/tmp/caresia-new.env
cp .env ".env.bak-$(date +%Y%m%d%H%M%S)"
[ -z "$(tail -c1 .env)" ] || echo >> .env # el archivo debe terminar en salto de línea
# Por defecto solo se agrega lo que falta (o está vacío); con OVERWRITE=1 también se reemplaza lo que ya existe.
: > "$new.todo"
while IFS= read -r line; do
  key="${line%%=*}"
  current="$(grep -m1 "^${key}=" .env | cut -d= -f2- | tr -d "'\" " || true)"
  if [ -n "$current" ] && [ "${OVERWRITE:-0}" != "1" ]; then
    echo "  = $key: ya estaba en el servidor, se conserva"
    continue
  fi
  grep -v "^${key}=" .env > .env.tmp || true
  mv .env.tmp .env
  printf '%s\n' "$line" >> "$new.todo"
  if [ -n "$current" ]; then echo "  ~ $key: reemplazada"; else echo "  + $key: agregada"; fi
done < "$new"
cat "$new.todo" >> .env
chmod 600 .env
rm -f "$new" "$new.todo"
docker compose up -d
