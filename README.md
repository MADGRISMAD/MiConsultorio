# Caresia

Plataforma de gestión para clínicas dentales y consultorios médicos: pacientes, citas e historiales clínicos.

| Capa | Tecnología |
| --- | --- |
| Frontend | [SvelteKit](https://svelte.dev) (Svelte 5, SPA) + Tailwind CSS |
| Backend | Go ([chi](https://github.com/go-chi/chi), [pgx](https://github.com/jackc/pgx)) |
| Base de datos | PostgreSQL 16 |

```
backend/    API en Go: autenticación, permisos, usuarios, expedientes y citas
frontend/   SvelteKit: landing comercial y panel de administración
```

## Empezar (sin Docker)

Necesitas instalados: **Go 1.26+**, **Node 22+** y **PostgreSQL 16** (anota la contraseña que le pongas al usuario `postgres`).

```bash
# 1. Configuración: copia el ejemplo y ajusta DATABASE_URL (tu contraseña de PostgreSQL),
#    JWT_SECRET (openssl rand -hex 32) y las contraseñas de PLATFORM_ADMIN_* y CLINIC_ADMIN_*
cp .env.example .env      # en PowerShell: copy .env.example .env

# 2. Instala todo y arranca backend + frontend con un solo comando (desde la raíz)
npm install
npm run dev
```

Abre <http://localhost:5173> e inicia sesión con el **correo o el usuario** del administrador de plataforma (`PLATFORM_ADMIN_*`) o del consultorio de ejemplo (`CLINIC_ADMIN_*`).
En el primer arranque el backend crea solo la base de datos, las tablas, el administrador de plataforma y, si lo configuraste, un consultorio de ejemplo.
El frontend espera a que el backend esté listo; si el backend falla (por ejemplo, PostgreSQL apagado), ambos se detienen y verás el motivo.

Otros comandos desde la raíz: `npm run build` (compila la interfaz), `npm start` (compila y sirve todo en <http://localhost:8080>, como en producción), `npm run test:api` y `npm run check`.

Cualquiera puede crear su propio consultorio (con 14 días de prueba) desde <http://localhost:5173/register> (el registro es mínimo; al entrar, un asistente de configuración te guía para elegir el giro —medicina general, odontología, pediatría, medicina interna, fisioterapia, nutrición, psicología, dermatología, ginecología, ortopedia, veterinaria o quiropráctica—, datos del negocio, horario y equipo; después se edita en Configuración). También puedes crear clínicas por línea de comandos: `cd backend && go run ./cmd/createclinic -name "Otra Clínica" -email otra@ejemplo.com -username admin`.

## Producción con Docker

```bash
cp .env.example .env     # define POSTGRES_PASSWORD, JWT_SECRET y los datos ADMIN_*
docker compose up --build
```

La clínica y el administrador se crean solos en el primer arranque.

Una sola imagen sirve la API (`/api/*`) y el frontend compilado en `:8080`. Pon un proxy con HTTPS delante (la cookie de sesión es `Secure`).

## Pantallas

- **Acceso:** `/login`, `/register` (crear consultorio), `/forgot` (recuperar contraseña) y los placeholders `/terminos` y `/privacidad`.
- **Panel:** inicio, citas, historiales, administración de citas/historiales y usuarios y permisos, con tema claro/oscuro.
- **Cobros (pendiente):** `/pos/cobros`, `/pos/caja`, `/pos/servicios`, `/pos/inventario`, `/pos/facturacion` y `/pos/reportes` ya están en el menú como ventanas "Próximamente"; su contenido está en `frontend/src/lib/pos.ts`.

## Pruebas

```bash
cd backend && TEST_DATABASE_URL="postgres://usuario:clave@localhost:5432/caresia_test?sslmode=disable" go test ./...
cd frontend && npm run check
```

Las pruebas del backend **borran el esquema `public`** de la base indicada: usa una base exclusiva para tests.

## Roles y permisos

El **rol** define lo que puede hacer cada persona (no hay casillas por usuario):

| Rol | Qué puede hacer |
| --- | --- |
| **Administrador** | Todo en su consultorio, incluido el equipo y los roles |
| **Médico / especialista** | Ve la agenda; lee y edita expedientes clínicos |
| **Recepción** | Ve y administra la agenda; sin acceso a expedientes; cobra en el punto de venta (planes con cobros) |
| **Cajero** | Ve la agenda; cobra, maneja la caja y ve reportes y facturas (planes con cobros) |
| **Administrador de plataforma** | Negocios, suscripciones, pagos y equipo de plataforma |
| **Soporte** | Consulta negocios y su actividad, sin cambiar nada |

Reglas (igual que en MiTiendita): siempre queda al menos un administrador activo; nadie cambia su propio rol ni se desactiva; desactivar, cambiar de rol o restablecer una contraseña **cierra las sesiones de esa persona al instante**; los lugares dependen del plan; y todo cambio queda en la bitácora de actividad.

Se entra con **correo o usuario** (ambos únicos en todo el sistema).

## Planes y suscripciones

Cada consultorio tiene un plan (Básico, Crecimiento, Pro; la sección de **Cobros** solo aparece en Crecimiento y Pro) y un estado: **prueba** (14 días), **activo**, **pago atrasado** o **suspendido**. Cuando no está activo, sus datos se bloquean (la cuenta sigue entrando para ver el aviso). Un periodo pagado vencido pasa solo a "pago atrasado" tras 3 días de gracia. El administrador de plataforma cambia plan y estado, suspende y registra pagos manuales desde **Negocios**.

## Cobros (punto de venta)

Incluido en los planes **Crecimiento** y **Pro**. Ventanas: **Punto de venta**, **Caja** (apertura, entradas/salidas, corte con diferencia), **Servicios y precios** (catálogo), **Inventario** (entradas, mermas, conteo físico, historial), **Facturación** (solicitudes de factura con datos fiscales; el timbrado del CFDI se hace con tu contador o PAC y aquí se marca emitida con su UUID), **Reportes** (ventas, utilidad, métodos de pago, CSV) y **Ajustes de cobros** (datos fiscales, ticket, reglas, métodos de pago, Mercado Pago e impresora).

- **Pagos:** efectivo (con cambio), tarjeta con terminal propia, transferencia, **Mercado Pago Point** y **liga/QR de Mercado Pago**. Funciona igual que en MiTiendita: el consultorio conecta su cuenta de Mercado Pago con OAuth (tokens cifrados con AES-GCM), elige su terminal (queda en modo PDV), el cobro se manda como *orden* a la terminal y se consulta hasta que Mercado Pago lo confirma; al cancelar una venta pagada con la terminal o con liga se **devuelve el dinero**.
- **Impresora térmica (58/80 mm):** desde el navegador con Web Serial, WebUSB o Web Bluetooth (Chrome/Edge sobre https o localhost), o cualquier impresora con el diálogo del navegador. Hay prueba de impresión, corte de papel y apertura de cajón.
- **Inventario Mágico / Precio Mágico:** con `GEMINI_API_KEY`, lee una lista pegada o una foto y propone artículos para revisar antes de guardarlos, y sugiere precios a partir del costo. Cuenta como "usos de magia" del plan (Crecimiento 150, Pro 500 al mes).
- **Suscripciones en línea:** el administrador paga su plan (mensual o anual) con Mercado Pago desde **Suscripción y plan**; un webhook firmado confirma el pago y extiende el periodo. Configura `MP_ACCESS_TOKEN`, `APP_URL` y `API_PUBLIC_URL` (ver `.env.example`).

Los precios en el backend van en centavos y los importes se calculan siempre en el servidor.

## Correos

Con `SMTP_*` y `MAIL_FROM` en el `.env` (ver `.env.example`; con Gmail usa una contraseña de aplicación), la app envía:

- **Recuperar contraseña** (`/forgot` → enlace a `/restablecer`): vale 1 hora, sirve una sola vez, cierra las sesiones abiertas y no revela si la cuenta existe. Los administradores permanentes de plataforma no lo usan: su clave la restablece otro administrador permanente.
- **Bienvenida** al registrarse, **aviso de pago recibido** a los administradores y **ticket por correo** desde el punto de venta.
- El correo sale en segundo plano: si el servidor SMTP falla, la venta o el registro no se detienen (el error queda en el log).

Sin SMTP todo lo demás funciona, y `/forgot` muestra los pasos manuales. El aviso de *deploy* por correo es aparte: lo manda GitHub Actions (secretos `SMTP_USER`, `SMTP_PASS`, `DEPLOY_NOTIFY_TO`).

## API

Todas las rutas viven bajo `/api`, hablan JSON y devuelven errores como `{"message": "..."}` (más un `code` en casos como `SUBSCRIPTION_REQUIRED` o `SEAT_LIMIT`). La sesión es una cookie `HttpOnly` firmada; el consultorio sale siempre de la sesión (nunca de la URL), por lo que cada uno solo ve sus propios datos. Los permisos se leen de la base de datos en cada petición.

| Ruta | Quién |
| --- | --- |
| `POST /login`, `POST /register`, `POST /logout` | público (el registro y el login tienen límites) |
| `GET /session`, `PUT /me`, `PUT /me/password` | cualquier sesión |
| `GET /clinic` | cuentas de consultorio |
| `GET/POST /team`, `PATCH /team/{id}`, `POST /team/{id}/password\|deactivate\|reactivate` | administrador del consultorio |
| `/expedients` | lectura: médico o administrador · escritura: médico o administrador |
| `/appointments` | lectura: todos los roles · escritura: recepción o administrador |
| `GET /billing`, `POST /billing/checkout`, `GET /billing/checkouts/{id}` | administrador del consultorio (funciona aun con la suscripción vencida) |
| `/pos/items`, `/pos/sales`, `/pos/cash/*`, `/pos/invoices`, `/pos/reports`, `/pos/settings`, `/pos/point/*`, `/pos/magic/*` | planes con cobros; permisos `pos`, `posReports`, `posManage` |
| `POST /forgot`, `POST /reset-password` | público (con límites por IP y cuenta) |
| `POST /pos/sales/{id}/email` | permiso `pos` |
| `POST /webhooks/mercadopago`, `GET /point/oauth/callback` | Mercado Pago (verificados por firma / estado firmado) |
| `GET /platform/overview\|plans\|clinics[/{id}]` | personal de plataforma |
| `PATCH /platform/clinics/{id}`, `POST …/suspend\|reactivate\|payments`, `/platform/staff…`, `GET /platform/activity` | administrador de plataforma |

## Despliegue

Este repositorio ya no despliega a Vercel: el workflow `ci.yml` solo compila y prueba. Despliega la imagen del `Dockerfile` donde prefieras (Fly.io, Railway, un VPS, etc.) junto con un PostgreSQL gestionado.

## Origen

La idea inicial nació de [MedX](https://github.com/JulyAn1234/MedX), de [Julian (JulyAn1234)](https://github.com/JulyAn1234).

Caresia es un producto independiente. A 3 de octubre de 2026, la interfaz, la experiencia de uso y el backend se están reconstruyendo para convertirlo en una solución comercial propia, ya lejos del proyecto original. Este reconocimiento no implica afiliación, sociedad ni relación comercial con MedX ni con sus autores.
