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
#    JWT_SECRET (openssl rand -hex 32) y ADMIN_PASSWORD
cp .env.example .env      # en PowerShell: copy .env.example .env

# 2. Instala todo y arranca backend + frontend con un solo comando (desde la raíz)
npm install
npm run dev
```

Abre <http://localhost:5173> e inicia sesión con `ADMIN_EMAIL`, `ADMIN_USERNAME` y `ADMIN_PASSWORD`.
En el primer arranque el backend crea solo la base de datos, las tablas y la clínica con su administrador.
El frontend espera a que el backend esté listo; si el backend falla (por ejemplo, PostgreSQL apagado), ambos se detienen y verás el motivo.

Otros comandos desde la raíz: `npm run build` (compila la interfaz), `npm start` (compila y sirve todo en <http://localhost:8080>, como en producción), `npm run test:api` y `npm run check`.

Para crear más clínicas: `cd backend && go run ./cmd/createclinic -name "Otra Clínica" -email otra@ejemplo.com -username admin`.

## Producción con Docker

```bash
cp .env.example .env     # define POSTGRES_PASSWORD, JWT_SECRET y los datos ADMIN_*
docker compose up --build
```

La clínica y el administrador se crean solos en el primer arranque.

Una sola imagen sirve la API (`/api/*`) y el frontend compilado en `:8080`. Pon un proxy con HTTPS delante (la cookie de sesión es `Secure`).

## Pruebas

```bash
cd backend && TEST_DATABASE_URL="postgres://usuario:clave@localhost:5432/caresia_test?sslmode=disable" go test ./...
cd frontend && npm run check
```

Las pruebas del backend **borran el esquema `public`** de la base indicada: usa una base exclusiva para tests.

## API

Todas las rutas viven bajo `/api`, hablan JSON y devuelven errores como `{"message": "..."}`. La sesión es una cookie `HttpOnly` firmada; la clínica sale siempre de la sesión (nunca de la URL), por lo que cada clínica solo ve sus propios datos.

| Ruta | Permiso requerido |
| --- | --- |
| `POST /login`, `POST /logout`, `GET /session`, `GET /clinic` | — / sesión |
| `GET/POST /users`, `PUT /users/permissions`, `PUT/DELETE /users/{username}` | `adminUsers` |
| `GET /expedients`, `GET /expedients/{curp}` | `navHistorials` o `adminHistorials` |
| `POST/PUT/DELETE /expedients[/{curp}]` | `adminHistorials` |
| `GET /appointments`, `GET /appointments/{id}` | `navAppointments` o `adminAppointments` |
| `POST/PUT/DELETE /appointments[/{id}]` | `adminAppointments` |

Los permisos se leen de la base de datos en cada petición: quitar un permiso o eliminar un usuario surte efecto de inmediato.

## Despliegue

Este repositorio ya no despliega a Vercel: el workflow `ci.yml` solo compila y prueba. Despliega la imagen del `Dockerfile` donde prefieras (Fly.io, Railway, un VPS, etc.) junto con un PostgreSQL gestionado.

## Origen

La idea inicial nació de [MedX](https://github.com/JulyAn1234/MedX), de [Julian (JulyAn1234)](https://github.com/JulyAn1234).

Caresia es un producto independiente. A 3 de octubre de 2026, la interfaz, la experiencia de uso y el backend se están reconstruyendo para convertirlo en una solución comercial propia, ya lejos del proyecto original. Este reconocimiento no implica afiliación, sociedad ni relación comercial con MedX ni con sus autores.
