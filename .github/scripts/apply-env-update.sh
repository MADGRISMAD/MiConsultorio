#!/usr/bin/env bash
# Se ejecuta EN EL SERVIDOR: mezcla /tmp/caresia-new.env en ~/miconsultorio/.env y reinicia la app.
set -euo pipefail
cd ~/miconsultorio
new=/tmp/caresia-new.env
cp .env ".env.bak-$(date +%Y%m%d%H%M%S)"
[ -z "$(tail -c1 .env)" ] || echo >> .env # el archivo debe terminar en salto de línea
while IFS= read -r line; do
  key="${line%%=*}"
  grep -v "^${key}=" .env > .env.tmp || true
  mv .env.tmp .env
done < "$new"
cat "$new" >> .env
chmod 600 .env
echo "Variables actualizadas en el servidor:"
cut -d= -f1 "$new" | sed 's/^/  - /'
rm -f "$new"
docker compose up -d
