# Pruebas e2e (Playwright)

Suite de punta a punta: un navegador real contra el servidor Go sirviendo el frontend ya construido, con SMTP y Mercado Pago falsos.
Es un paquete aparte (no forma parte del frontend ni del backend).

## Qué cubre

| Archivo | Flujo |
| --- | --- |
| `tests/01-auth.spec.ts` | registro de un consultorio, cierre e inicio de sesión, contraseña incorrecta, ruta protegida |
| `tests/02-patients.spec.ts` | alta de paciente persona y de paciente animal |
| `tests/03-consulta.spec.ts` | consulta en la bitácora, adenda y receta |
| `tests/04-agenda.spec.ts` | cita que choca con otra del mismo profesional (aviso de conflicto) |
| `tests/05-pos.spec.ts` | apertura de caja, venta con efectivo y cambio, cierre de caja |
| `tests/06-factura.spec.ts` | solicitud de factura de una venta |
| `tests/07-recuperar.spec.ts` | recuperación de contraseña con el correo capturado por el SMTP falso |
| `tests/08-reportes.spec.ts` | pantalla `/reportes` (operación y pacientes) |

## Correr en local

Requisitos: Node 22, Go, PostgreSQL en marcha y el cliente `psql`.

```bash
# 1. base de pruebas (se vacía en cada corrida; no uses una con datos)
psql postgres://caresia:caresia@localhost:5432/postgres -c 'CREATE DATABASE caresia_e2e'
# 2. frontend construido
npm ci --prefix frontend && npm run build --prefix frontend
# 3. suite
cd e2e
npm ci
npx playwright install --with-deps chromium   # una vez
npm test
```

`npm test` arranca solo todo lo necesario (`support/stack.mjs`): reinicia la base, levanta el SMTP y Mercado Pago falsos, compila y arranca el servidor Go en el puerto 8099.
La sesión del consultorio de ejemplo (`dra`) se guarda una vez en `.tmp/state.json` (ver `support/global-setup.ts`) y los tests la reutilizan; los de acceso y recuperación empiezan sin sesión.

Variables opcionales: `E2E_PORT` (8099), `E2E_SMTP_PORT` (2526), `E2E_MP_PORT` (9199), `E2E_DATABASE_URL`
(`postgres://caresia:caresia@localhost:5432/caresia_e2e?sslmode=disable`), `E2E_STATIC_DIR` (`frontend/build`),
`E2E_SERVER_BIN` (binario ya compilado), `E2E_CHROMIUM_PATH` (usar un Chromium instalado en vez del de Playwright).

Para depurar: `npm run test:headed`, o abrir una traza con `npx playwright show-trace test-results/<prueba>/trace.zip`.

## Cómo está organizada

- `support/app.ts`: helpers y **todos los selectores que dependen del marcado** (`ui`). Si cambia una pantalla, se ajusta ahí.
- `support/fakesmtp.mjs`: SMTP mínimo que guarda los correos en `.tmp/mails.txt`; `waitForMail` los lee.
- `support/fakemp.mjs`: API falsa de Mercado Pago (preferencias, OAuth, terminal Point, órdenes, pagos). El servidor ya apunta a ella; hoy ningún test la ejercita (falta un flujo de cobro con enlace o terminal).
- Los tests son deterministas: un solo worker, nombres únicos por corrida y datos de apoyo creados por API cuando no son el objeto de la prueba.

## CI

El job `e2e` de `.github/workflows/ci.yml` corre después de `backend` y `frontend`, con un servicio `postgres:16`. Sube `e2e/test-results` y `e2e/playwright-report` (capturas, trazas) como artefacto cuando falla.
Por ahora es informativo (`continue-on-error: true` y no está en el `needs` del deploy); cuando se estabilice, se quita esa línea y se agrega `e2e` a `needs` de `deploy-backend`.
