# Respaldos y restauración

Un respaldo que nunca se ha probado es solo una esperanza. Esta guía explica qué se respalda, cómo restaurar paso a paso y, sobre todo, **cómo probar una restauración** sin tocar los datos reales.

## Qué se respalda

| Qué | Dónde vive | Cómo se respalda |
| --- | --- | --- |
| Base de datos (pacientes, bitácora, recetas, ventas, usuarios…) | PostgreSQL | `pg_dump` en formato custom comprimido (`caresia-db-AAAAMMDD-HHMMSS.dump`) |
| Archivos adjuntos (radiografías, laboratorio, consentimientos) | Carpeta `UPLOADS_DIR` (volumen `uploads` en Docker) | `tar.gz` (`caresia-uploads-AAAAMMDD-HHMMSS.tar.gz`) |

Dos cosas que conviene entender:

- Los adjuntos están **cifrados en disco**, y la llave sale de `TOKEN_ENC_KEY` (o, si no la definiste, de `JWT_SECRET`). **Sin esa llave, un respaldo de adjuntos es inservible.** Guarda `JWT_SECRET` y `TOKEN_ENC_KEY` en un gestor de contraseñas, **fuera** del servidor y fuera del respaldo. Lo mismo vale para el secreto de la verificación en dos pasos, que también se cifra con esa llave.
- Cada respaldo se verifica al terminar (`pg_restore --list` y `tar -t`). Si falla, no se conserva y el script sale con un código distinto de cero.

## Con Docker Compose (lo normal)

El servicio `backup` de `docker-compose.yml` arranca solo, sin configurar nada: hace un respaldo al iniciar (si no hay uno de las últimas 24 horas) y otro cada día a las 03:00 (hora del contenedor), y los guarda en el volumen `backups`. Rotación por defecto: 14 días, y nunca baja de los 7 más recientes.

Variables opcionales (en `.env`):

| Variable | Qué hace | Por defecto |
| --- | --- | --- |
| `BACKUP_CRON_HOUR` | Hora del respaldo diario (0 a 23) | `3` |
| `BACKUP_RETENTION_DAYS` | Días que se conservan los respaldos | `14` |
| `BACKUP_KEEP` | Cantidad mínima que nunca se borra, sin importar la edad | `7` |
| `BACKUP_RCLONE_REMOTE` | Destino de la copia externa (ver abajo) | vacío (sin copia externa) |
| `BACKUP_STATUS_FILE` | Archivo JSON con el resultado del último respaldo; `/api/health` lo lee | `/backups/status.json` en Docker |
| `BUILD_COMMIT` | Versión que muestra `/api/health` | vacío |

Respaldo manual inmediato:

```sh
docker compose exec backup sh /scripts/backup.sh
docker compose exec backup ls -lh /backups
```

Para ver cómo le fue al último: `curl -s http://127.0.0.1:8080/api/health` muestra `backup.status`, `backup.age_hours` y marca `degraded` si el último respaldo falló o tiene más de 36 horas. Quien administra la plataforma ve más detalle en `GET /api/platform/health`.

## Sin Docker (cron)

```sh
# crontab -e   (todos los días a las 03:15)
15 3 * * * DATABASE_URL='postgres://usuario:clave@localhost:5432/caresia' BACKUP_DIR=/var/backups/caresia UPLOADS_DIR=/srv/caresia/uploads BACKUP_STATUS_FILE=/var/backups/caresia/status.json /ruta/al/repo/scripts/backup.sh >>/var/log/caresia-backup.log 2>&1
```

Necesita `pg_dump`, `pg_restore` y `tar` (paquete `postgresql-client`).

Códigos de salida de `scripts/backup.sh`: `0` bien · `1` falló el volcado · `2` no pasó la verificación · `3` el respaldo local está bien pero falló la copia externa · `4` configuración inválida. Haz que tu cron o monitor avise cuando no sea `0`.

## Copia externa (recomendado)

Un respaldo en el mismo servidor no te salva de un disco dañado, un robo o un error del proveedor. Usa `rclone` con cualquier almacenamiento (Backblaze B2, Cloudflare R2, S3, Google Drive, una carpeta de red…). Las credenciales **van en variables de entorno o en tu configuración de rclone, nunca en el repositorio**.

Ejemplo con un almacenamiento compatible con S3, todo en `.env`:

```sh
BACKUP_RCLONE_REMOTE=r2:caresia-respaldos
RCLONE_CONFIG_R2_TYPE=s3
RCLONE_CONFIG_R2_PROVIDER=Cloudflare
RCLONE_CONFIG_R2_ACCESS_KEY_ID=...
RCLONE_CONFIG_R2_SECRET_ACCESS_KEY=...
RCLONE_CONFIG_R2_ENDPOINT=https://<cuenta>.r2.cloudflarestorage.com
```

Y agrega esas variables `RCLONE_CONFIG_*` a la sección `environment` del servicio `backup` en `docker-compose.yml`. Con `BACKUP_RCLONE_REMOTE` definido, el contenedor instala `rclone` al arrancar. Si la copia falla, el respaldo local se conserva y el estado queda en `warn`.

Buenas prácticas: usa una credencial que **solo pueda escribir** (no borrar) en ese almacenamiento, y activa el versionado o la retención del lado del proveedor.

## Cómo restaurar (paso a paso)

> La restauración **reemplaza** los datos de la base destino. Hazla con calma y, si es producción, avisa al equipo: nadie debe estar usando el sistema.

1. **Elige el respaldo.** Lista los disponibles: `docker compose exec backup ls -lh /backups`. El `.dump` y el `.tar.gz` con la misma fecha van juntos.
2. **Detén la aplicación** para que no escriba mientras restauras: `docker compose stop app`.
3. **Conserva lo que hay** (por si acaso): `docker compose exec backup sh /scripts/backup.sh`.
4. **Restaura.** Con Docker, el contenedor `backup` ya tiene las herramientas y la conexión a la base; monta los adjuntos en modo escritura solo para esta operación:

   ```sh
   docker compose run --rm --no-deps -e RESTORE_UPLOADS_DIR=/restaurar-adjuntos -v NOMBRE_DEL_VOLUMEN_uploads:/restaurar-adjuntos backup \
     sh /scripts/restore.sh --latest        # o: sh /scripts/restore.sh /backups/caresia-db-AAAAMMDD-HHMMSS.dump /backups/caresia-uploads-AAAAMMDD-HHMMSS.tar.gz
   ```

   (Sustituye `NOMBRE_DEL_VOLUMEN_uploads` por el volumen real, que depende del nombre de tu carpeta: `docker volume ls | grep uploads`.) El script te pide escribir `RESTAURAR`, restaura la base, extrae los adjuntos y muestra un conteo de clínicas, usuarios, pacientes, bitácora, recetas y archivos. Sin Docker: `DATABASE_URL=... UPLOADS_DIR=... scripts/restore.sh --latest`.
5. **Revisa los conteos** contra lo que esperas.
6. **Arranca la aplicación:** `docker compose start app`. Las migraciones pendientes se aplican solas al iniciar.
7. **Comprueba:** entra con un usuario, abre un expediente reciente y abre un archivo adjunto (si abre, la llave de cifrado es la correcta).

## Cómo PROBAR una restauración (hazlo cada trimestre)

La prueba se hace en una base y una carpeta **distintas**, sin tocar producción:

```sh
# 1. Base vacía de pruebas (en el contenedor de la base)
docker compose exec db psql -U caresia -d postgres -c 'CREATE DATABASE caresia_prueba'

# 2. Restaurar el último respaldo ahí, con los adjuntos en una carpeta temporal
docker compose run --rm --no-deps \
  -e RESTORE_DATABASE_URL="postgres://caresia:${POSTGRES_PASSWORD}@db:5432/caresia_prueba" \
  -e RESTORE_UPLOADS_DIR=/tmp/adjuntos-prueba \
  backup sh /scripts/restore.sh --latest

# 3. Comparar conteos con producción
docker compose exec db psql -U caresia -d caresia -Atc 'SELECT count(*) FROM patients'
docker compose exec db psql -U caresia -d caresia_prueba -Atc 'SELECT count(*) FROM patients'

# 4. Limpiar
docker compose exec db psql -U caresia -d postgres -c 'DROP DATABASE caresia_prueba'
```

Qué verificar: los conteos coinciden (salvo lo que se capturó después del respaldo), el script terminó sin errores y, si puedes, que alguien levante una instancia de prueba apuntando a `caresia_prueba` con las mismas llaves y abra un expediente y un adjunto. Anota la fecha de la prueba y quién la hizo.

Los scripts se probaron localmente con el ciclo respaldo → restauración → verificación de conteos contra PostgreSQL 16; aun así, haz tu propia prueba en tu entorno.

## Cuánto tiempo conservar (NOM-004-SSA3-2012)

El expediente clínico debe conservarse **al menos 5 años** contados desde el último acto médico. Caresia nunca borra expedientes (solo los archiva), pero **el respaldo es lo que te protege si el servidor se pierde**. Una política razonable:

| Tipo de copia | Frecuencia | Retención sugerida |
| --- | --- | --- |
| Diaria, en el servidor (servicio `backup`) | Cada día | 14 a 30 días (`BACKUP_RETENTION_DAYS`) |
| Externa (`rclone`) | Cada día | 90 días de copias diarias en el proveedor, más **una copia mensual guardada 5 años o más** |
| Prueba de restauración | Cada trimestre | Registro con fecha y responsable |

Para la copia mensual de largo plazo, configura una regla de ciclo de vida en tu proveedor (o copia a mano el respaldo del día 1 a una carpeta aparte que no se rote). Los respaldos contienen datos de salud (datos sensibles, LFPDPPP): guárdalos con acceso restringido, en un lugar cifrado, y destrúyelos de forma segura cuando venza su plazo.

## Exportar el expediente de una persona (ARCO y portabilidad)

Para una solicitud de acceso o portabilidad, un administrador o un profesional con permiso de historial puede descargar el expediente completo en JSON desde el propio expediente (`GET /api/patients/{id}/export`). Incluye datos del paciente, bitácora, recetas, vacunas, planes, consentimientos, metadatos de archivos, citas y la bitácora de accesos. La exportación queda registrada en los accesos del expediente y en la actividad. Las notas privadas de otra persona aparecen sin contenido, y los archivos adjuntos se entregan por separado desde la pestaña de Archivos.

## Llave de cifrado

Además de los adjuntos, Caresia cifra dentro de la base el contenido clínico libre: interrogatorio, exploración, diagnóstico, plan y notas de la bitácora (también las privadas y las adendas), diagnóstico e indicaciones de las recetas, antecedentes y alergias del paciente y el detalle de las citas. Se guardan como `enc:v1:…` (en el perfil del paciente, como `{"_enc":"v1:…"}`). Cada valor queda ligado a su tabla, columna y registro: copiarlo a otro registro lo invalida. Motivo de consulta, mediciones, nombres, fechas, códigos CIE-10 y los medicamentos de la receta **no** se cifran porque los reportes y la agenda los usan.

La llave se deriva de `TOKEN_ENC_KEY` (o de `JWT_SECRET` si no la definiste); no hay una variable aparte. Consecuencias:

- **Un respaldo de la base sin esa llave no se puede leer en sus campos clínicos.** Guarda `JWT_SECRET` y `TOKEN_ENC_KEY` fuera del servidor, y verifica en cada prueba de restauración que el expediente abre con esas llaves.
- **No cambies `JWT_SECRET` ni `TOKEN_ENC_KEY` en un sistema con datos.** Si cambian, los registros cifrados quedan ilegibles: el sistema no muestra basura ni se cae, responde `DECRYPT_FAILED` (HTTP 500 con un mensaje claro) en las pantallas que los leen, y deja una línea `decrypt_failed` en la bitácora de actividad con la tabla, columna y registro afectados (nunca el contenido). Lo que no está cifrado (listas de pacientes, agenda sin detalles, caja) sigue funcionando, y lo que se capture a partir de entonces se cifra con la llave nueva. Para recuperar, vuelve a poner la llave anterior y reinicia; ese contenido no se puede reconstruir de otra forma.
- **Las llaves de adjuntos y tokens de pago dependen de la misma llave maestra**, así que un cambio las afecta igual.

Registros anteriores a esta función se leen en claro sin problema y se van cifrando cuando se vuelven a guardar. Para cifrar todo lo existente de una vez (conviene hacerlo con un respaldo reciente y fuera del horario de consulta):

```bash
cd backend
go run ./cmd/encryptfields --dry-run   # cuenta lo que cambiaría, no escribe nada
go run ./cmd/encryptfields             # cifra por lotes; se puede repetir sin riesgo
```

Con Docker Compose, el binario viene en la imagen: `docker compose exec app /app/encryptfields --dry-run` (y luego sin `--dry-run`), con las mismas variables de entorno del servicio. Un valor que no se pueda abrir con la llave actual no se toca y el comando lo reporta (código de salida 3). El formato lleva un identificador de llave para poder rotarla en el futuro sin perder lo anterior.
