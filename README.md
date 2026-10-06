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

## Empezar (desarrollo)

Necesitas Go 1.26+, Node 22+ y un PostgreSQL accesible.

```bash
# 1. Base de datos
createdb caresia

# 2. API (migra el esquema al arrancar)
cd backend
export DATABASE_URL="postgres://usuario:clave@localhost:5432/caresia?sslmode=disable"
export JWT_SECRET="$(openssl rand -hex 32)"
export COOKIE_SECURE=false          # solo en desarrollo con http
export ALLOWED_ORIGINS=http://localhost:5173
go run ./cmd/server                 # escucha en :8080

# 3. Da de alta una clínica y su primer administrador
go run ./cmd/createclinic -name "Mi Clínica" -email clinica@ejemplo.com -username admin
#   (pide la contraseña, o usa CARESIA_ADMIN_PASSWORD)

# 4. Frontend (en otra terminal; hace proxy de /api hacia :8080)
cd frontend
npm install
npm run dev                         # http://localhost:5173
```

Inicia sesión con el **correo de la clínica**, el usuario y la contraseña.

## Producción con Docker

```bash
cp .env.example .env     # define POSTGRES_PASSWORD y JWT_SECRET
docker compose up --build
docker compose run --rm --entrypoint /app/createclinic app -name "Mi Clínica" -email clinica@ejemplo.com -username admin
```

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
