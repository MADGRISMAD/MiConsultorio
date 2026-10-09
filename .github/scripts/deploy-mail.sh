#!/usr/bin/env bash
# Avisa por correo cómo terminó el deploy a producción. Lo llama el workflow "CI" (job notify-deploy).
# Variables: SMTP_USER SMTP_PASS NOTIFY_TO [SMTP_HOST SMTP_PORT MAIL_FROM] RESULT SHA ACTOR AUTHOR COMMIT_MSG RUN_URL
#            REPO BRANCH APP_URL. Con SMTP_INSECURE=1 no exige TLS (solo para pruebas locales).
set -euo pipefail

if [ -z "${SMTP_USER:-}" ] || [ -z "${SMTP_PASS:-}" ] || [ -z "${NOTIFY_TO:-}" ]; then
  echo "Faltan SMTP_USER, SMTP_PASS o DEPLOY_NOTIFY_TO: no se envía correo."
  exit 0
fi

# Las contraseñas de aplicación de Gmail se muestran con espacios; el servidor las quiere juntas.
SMTP_PASS="$(printf '%s' "$SMTP_PASS" | tr -d '[:space:]"'"'")"
SMTP_USER="$(printf '%s' "$SMTP_USER" | tr -d '[:space:]"'"'")"
HOST="${SMTP_HOST:-smtp.gmail.com}"
PORT="${SMTP_PORT:-587}"
FROM="${MAIL_FROM:-Caresia <$SMTP_USER>}"
FROM="$(printf '%s' "$FROM" | tr -d '\r\n')"

# Destinatarios: aceptan comas, espacios o saltos de línea. Nunca van en el cuerpo ni en las cabeceras
# visibles: el mensaje se envía "a" la propia cuenta y los demás reciben copia oculta.
mapfile -t RCPT < <(printf '%s' "$NOTIFY_TO" | tr ',;\r\n' '\n\n\n\n' | tr -d ' "'"'" | sed '/^$/d' | grep -E '^[^@<>]+@[^@<>]+$' || true)
if [ "${#RCPT[@]}" -eq 0 ]; then
  echo "::warning::DEPLOY_NOTIFY_TO no tiene ningún correo válido."
  exit 0
fi

html_escape() { sed -e 's/&/\&amp;/g' -e 's/</\&lt;/g' -e 's/>/\&gt;/g' -e 's/"/\&quot;/g'; }
MONTHS=(ene feb mar abr may jun jul ago sep oct nov dic)
ts="$(TZ=America/Tijuana date '+%-d|%-m|%Y|%H:%M|')"
IFS='|' read -r d m y hm _ <<<"$ts"
WHEN="$d ${MONTHS[$((m-1))]} $y · $hm h · Tijuana"

if [ "${RESULT:-}" = "success" ]; then
  SUBJECT="✅ Caresia está en producción"
  TITLE="Caresia ya está en el aire"
  PILL="EN PRODUCCIÓN"; PILL_BG="#0F9E8E"
  INTRO="El servicio quedó publicado y la API respondió. Revisa que la agenda, los expedientes y los cobros funcionen como esperas."
else
  SUBJECT="❌ Caresia: el deploy falló"
  TITLE="El deploy no terminó bien"
  PILL="REQUIERE ATENCIÓN"; PILL_BG="#C2410C"
  INTRO="La publicación no se completó. La versión anterior sigue en producción. Abre el registro para ver en qué paso falló."
fi

SHORT="${SHA:0:7}"
MSG_HTML="$(printf '%s' "${COMMIT_MSG:-}" | html_escape | sed ':a;N;$!ba;s/\n/<br>/g')"
row() { printf '<tr><td style="padding:11px 0;border-bottom:1px solid #dfe8ef;font:600 10px/1.2 Menlo,Consolas,monospace;letter-spacing:.14em;text-transform:uppercase;color:#6b7f90;width:34%%">%s</td><td style="padding:11px 0;border-bottom:1px solid #dfe8ef;font:600 15px/1.35 Segoe UI,Helvetica,Arial,sans-serif;color:#0B2540">%s</td></tr>' "$1" "$2"; }
ROWS="$(row Servicio Caresia)$(row Repositorio "$(printf '%s' "${REPO:-}" | html_escape)")$(row Rama "$(printf '%s' "${BRANCH:-main}" | html_escape)")$(row SHA "$SHORT")$(row Autor "$(printf '%s' "${AUTHOR:-$ACTOR}" | html_escape)")$(row Publicó "$(printf '%s' "${ACTOR:-}" | html_escape)")$(row Fecha "$WHEN")"
APP_URL="${APP_URL:-https://caresia.mx}"
APP="$(printf '%s' "${APP_URL:-}" | html_escape)"
RUN="$(printf '%s' "${RUN_URL:-}" | html_escape)"

HTML="$(cat <<EOF
<!doctype html><html lang="es"><head><meta name="viewport" content="width=device-width,initial-scale=1"><style>@media only screen and (max-width:480px){.px{padding-left:18px!important;padding-right:18px!important}.ttl{font-size:19px!important}}</style></head><body style="margin:0;background:#e9f0f6;font-family:Segoe UI,Helvetica,Arial,sans-serif;color:#0B2540">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#e9f0f6;padding:28px 10px"><tr><td align="center">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:620px;border-radius:18px;overflow:hidden;background:#F4F8FB">
  <tr><td class="px" style="background:#0B2540;padding:26px 28px 22px">
    <div style="font:600 10px/1 Menlo,Consolas,monospace;letter-spacing:.2em;color:#7fb4ec">SISTEMA INTERNO</div>
    <table role="presentation" cellpadding="0" cellspacing="0" style="margin-top:14px"><tr>
      <td style="padding:0 12px 0 0;vertical-align:middle"><img src="cid:caresia-logo" width="44" height="44" alt="" style="display:block;border:0;border-radius:11px"></td>
      <td style="vertical-align:middle;font:600 28px/1 Georgia,'Times New Roman',serif;color:#F4F8FB;letter-spacing:-.01em">Caresia<span style="color:#56A4F0">.</span></td>
    </tr></table>
    <div style="width:42px;height:3px;background:#1673D1;margin:16px 0 14px;border-radius:2px"></div>
    <div class="ttl" style="font:600 22px/1.25 Georgia,'Times New Roman',serif;color:#ffffff">${TITLE}</div>
  </td></tr>
  <tr><td class="px" style="background:${PILL_BG};padding:9px 28px;font:700 11px/1 Menlo,Consolas,monospace;letter-spacing:.16em;color:#ffffff">${PILL}</td></tr>
  <tr><td class="px" style="padding:24px 28px 8px;font-size:15px;line-height:1.55;color:#33475b">${INTRO}</td></tr>
  <tr><td class="px" style="padding:8px 28px 6px"><table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#e4edf5;border-radius:12px;padding:4px 18px"><tr><td>
    <table role="presentation" width="100%" cellpadding="0" cellspacing="0">${ROWS}</table>
  </td></tr></table></td></tr>
  <tr><td class="px" style="padding:18px 28px 4px">
    <div style="font:600 10px/1 Menlo,Consolas,monospace;letter-spacing:.16em;color:#6b7f90;margin-bottom:8px">MENSAJE DEL COMMIT</div>
    <div style="border-left:3px solid #1673D1;background:#ffffff;border-radius:0 10px 10px 0;padding:12px 14px;font-size:14px;line-height:1.5;color:#33475b">${MSG_HTML}</div>
  </td></tr>
  <tr><td class="px" style="padding:22px 28px 6px">
    <a href="${APP}" style="display:inline-block;background:#0B2540;color:#ffffff;text-decoration:none;font:700 14px/1 Segoe UI,Helvetica,Arial,sans-serif;padding:14px 22px;border-radius:12px;margin:0 8px 8px 0">Abrir producción</a>
    <a href="${RUN}" style="display:inline-block;background:#dbe7f2;color:#0B2540;text-decoration:none;font:700 14px/1 Segoe UI,Helvetica,Arial,sans-serif;padding:14px 22px;border-radius:12px;margin:0 0 8px">Ver el registro</a>
  </td></tr>
  <tr><td class="px" style="padding:20px 28px 26px"><div style="border-top:1px solid #dfe8ef;padding-top:14px;font-size:12px;line-height:1.5;color:#6b7f90">Aviso automático del equipo de Caresia. Tus pacientes y clientes no reciben este correo.<div style="margin-top:8px;font:600 15px/1 Georgia,'Times New Roman',serif;color:#0B2540">Caresia<span style="color:#1673D1">.</span></div><div style="font:10px/1.4 Menlo,Consolas,monospace;letter-spacing:.1em;color:#8da0b0;margin-top:3px">operación · ${y}</div></div></td></tr>
</table></td></tr></table></body></html>
EOF
)"

TEXT="${TITLE}

${INTRO}

Servicio: Caresia
Repositorio: ${REPO:-}
Rama: ${BRANCH:-main}
SHA: ${SHORT}
Autor: ${AUTHOR:-$ACTOR}
Publicó: ${ACTOR:-}
Fecha: ${WHEN}

Commit: $(printf '%s' "${COMMIT_MSG:-}" | head -n1)

Abrir producción: ${APP_URL:-}
Ver el registro: ${RUN_URL:-}
"

b64() { base64 -w0 | fold -w 76; }
BOUNDARY="caresia-$(date +%s)-$RANDOM"
SUBJECT_B64="=?UTF-8?B?$(printf '%s' "$SUBJECT" | base64 -w0)?="
FROM_NAME="${FROM%%<*}"; FROM_ADDR="$(printf '%s' "$FROM" | sed -n 's/.*<\(.*\)>.*/\1/p')"; FROM_ADDR="${FROM_ADDR:-$FROM}"
if [ -n "$(printf '%s' "$FROM_NAME" | tr -d '[:space:]')" ] && [ "$FROM_NAME" != "$FROM" ]; then
  FROM_HDR="=?UTF-8?B?$(printf '%s' "$FROM_NAME" | sed 's/[[:space:]]*$//' | base64 -w0)?= <${FROM_ADDR}>"
else
  FROM_HDR="$FROM_ADDR"
fi

{
  printf 'From: %s\r\n' "$FROM_HDR"
  printf 'To: %s\r\n' "$FROM_ADDR"            # los destinatarios reales van en copia oculta
  printf 'Subject: %s\r\n' "$SUBJECT_B64"
  printf 'MIME-Version: 1.0\r\n'
  printf 'Content-Type: multipart/alternative; boundary="%s"\r\n\r\n' "$BOUNDARY"
  printf -- '--%s\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n%s\r\n' "$BOUNDARY" "$(printf '%s' "$TEXT" | b64)"
  REL="caresia-rel-$RANDOM"
  printf -- '--%s\r\nContent-Type: multipart/related; boundary="%s"\r\n\r\n' "$BOUNDARY" "$REL"
  printf -- '--%s\r\nContent-Type: text/html; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n%s\r\n' "$REL" "$(printf '%s' "$HTML" | b64)"
  printf -- '--%s\r\nContent-Type: image/png; name="caresia.png"\r\nContent-Transfer-Encoding: base64\r\nContent-ID: <caresia-logo>\r\nContent-Disposition: inline; filename="caresia.png"\r\n\r\n%s\r\n' "$REL" "$(b64 < "$(dirname "$0")/caresia-logo.png")"
  printf -- '--%s--\r\n' "$REL"
  printf -- '--%s--\r\n' "$BOUNDARY"
} > mail.eml

args=(--url "smtp://$HOST:$PORT" --mail-from "$SMTP_USER" --upload-file mail.eml -sS -f)
[ "${SMTP_INSECURE:-}" = "1" ] || args+=(--ssl-reqd --user "$SMTP_USER:$SMTP_PASS")
for r in "${RCPT[@]}"; do args+=(--mail-rcpt "$r"); done

if ! curl "${args[@]}"; then
  echo "::warning title=No se pudo enviar el correo del deploy::El servidor SMTP rechazó el acceso o la conexión. Revisa SMTP_USER (correo completo) y SMTP_PASS (contraseña de APLICACIÓN de Gmail, no la normal). El deploy en sí no se afectó."
  exit 1
fi
echo "Correo enviado a ${#RCPT[@]} destinatario(s)."
