#!/usr/bin/env bash
# Arma el archivo con las variables que se van a copiar al .env del servidor (un KEY='valor' por línea).
# Lee las variables del entorno; las vacías se omiten. Nunca imprime los valores.
set -euo pipefail
out="${1:-new.env}"
: > "$out"
for k in SMTP_HOST SMTP_PORT SMTP_SECURE SMTP_USER SMTP_PASS MAIL_FROM APP_URL GEMINI_API_KEY GEMINI_MODEL; do
  v="${!k:-}"
  # sin espacios al inicio o al final
  v="$(printf '%s' "$v" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
  case "$k" in
    # la contraseña de aplicación de Gmail se copia con espacios (abcd efgh ...): se quitan
    SMTP_USER | SMTP_PASS | SMTP_PORT | SMTP_SECURE) v="$(printf '%s' "$v" | tr -d '[:space:]"'"'")" ;;
    APP_URL) v="${v%/}" ;;
  esac
  if [ -z "$v" ]; then
    echo "- $k: no está en GitHub, se omite"
    continue
  fi
  case "$v" in
    *\'* | *$'\n'*)
      echo "::error::$k contiene comillas simples o saltos de línea"
      exit 1
      ;;
  esac
  printf "%s='%s'\n" "$k" "$v" >> "$out"
  echo "- $k: listo"
done
[ -s "$out" ] || { echo "::error::No hay ninguna variable para enviar: agrega los secretos en GitHub primero."; exit 1; }
