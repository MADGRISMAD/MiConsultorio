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
| **Recepción** | Ve y administra la agenda; sin acceso a expedientes |
| **Cajero** | Ve la agenda (los cobros llegan con el punto de venta) |
| **Administrador de plataforma** | Negocios, suscripciones, pagos y equipo de plataforma |
| **Soporte** | Consulta negocios y su actividad, sin cambiar nada |

Reglas (igual que en MiTiendita): siempre queda al menos un administrador activo; nadie cambia su propio rol ni se desactiva; desactivar, cambiar de rol o restablecer una contraseña **cierra las sesiones de esa persona al instante**; los lugares dependen del plan; y todo cambio queda en la bitácora de actividad.

Se entra con **correo o usuario** (ambos únicos en todo el sistema).

## Planes y suscripciones

Cada consultorio tiene un plan (Consultorio, Clínica, Empresarial) y un estado: **prueba** (14 días), **activo**, **pago atrasado** o **suspendido**. Cuando no está activo, sus datos se bloquean (la cuenta sigue entrando para ver el aviso). Un periodo pagado vencido pasa solo a "pago atrasado" tras 3 días de gracia. El administrador de plataforma cambia plan y estado, suspende y registra pagos manuales desde **Negocios**.

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
| `GET /platform/overview\|plans\|clinics[/{id}]` | personal de plataforma |
| `PATCH /platform/clinics/{id}`, `POST …/suspend\|reactivate\|payments`, `/platform/staff…`, `GET /platform/activity` | administrador de plataforma |

## Despliegue

Este repositorio ya no despliega a Vercel: el workflow `ci.yml` solo compila y prueba. Despliega la imagen del `Dockerfile` donde prefieras (Fly.io, Railway, un VPS, etc.) junto con un PostgreSQL gestionado.

## Origen

La idea inicial nació de [MedX](https://github.com/JulyAn1234/MedX), de [Julian (JulyAn1234)](https://github.com/JulyAn1234).

Caresia es un producto independiente. A 3 de octubre de 2026, la interfaz, la experiencia de uso y el backend se están reconstruyendo para convertirlo en una solución comercial propia, ya lejos del proyecto original. Este reconocimiento no implica afiliación, sociedad ni relación comercial con MedX ni con sus autores.
