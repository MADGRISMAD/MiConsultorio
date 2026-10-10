# Sucursales y reportes consolidados

Un dueño puede tener varias sucursales bajo una misma organización, entrar a cada una con su propio usuario y ver reportes juntos. **El modelo de aislamiento no cambia**: la unidad de aislamiento sigue siendo el consultorio (`clinic_id`) y cada consulta sigue filtrando por él.

## Modelo

- `organizations` (id, name, owner_user_id, matrix_clinic_id, created_at). Una clínica sin organización (`clinics.organization_id` NULL) funciona exactamente igual que antes.
- Una **sucursal es una clínica independiente**: pacientes, citas, expedientes, archivos, inventario, caja y equipo son propios. Solo comparte `organization_id` con las demás.
- La **matriz** es la clínica donde el dueño fundó la organización. El **dueño** es el primer administrador de esa clínica (el más antiguo, no una cuenta de sucursal) y queda en `organizations.owner_user_id`. Un segundo administrador de la matriz no es dueño y no puede crear sucursales ni entrar a ellas.
- La organización se crea sola al agregar la primera sucursal (`POST /org/branches`), siempre que el plan lo permita.

## Cambio de sucursal: cuenta de administrador vinculada

Se eligió **una cuenta de administrador propia de cada sucursal, vinculada al dueño** (`users.linked_owner_id`), en lugar de membresías con "clínica activa" por petición. Razón: el `Principal` sigue teniendo exactamente un `clinic_id` verificado desde la base de datos en cada petición, así que **ninguna ruta existente cambia ni puede olvidarse de comprobar una membresía**. Todo el código y las pruebas de aislamiento actuales siguen valiendo.

- Al crear la sucursal se crea su usuario `sucursal-<id>`: rol `admin`, `linked_owner_id = dueño`, **sin correo** (restricción CHECK) y con contraseña aleatoria que nadie conoce. No ocupa lugar del plan (`seatUsage` lo excluye) y los administradores de la sucursal no pueden editarlo, desactivarlo ni cambiarle la contraseña (`loadMember`).
- No se puede iniciar sesión con él: `login` ignora las cuentas vinculadas (aunque alguien les ponga contraseña a mano), `forgot` no tiene correo al que escribir.
- `POST /org/switch {branch_id}`: en esa misma petición se verifica en la base de datos que (1) el solicitante es administrador activo y es el dueño de la organización (directamente o desde una sesión de sucursal suya), (2) la sucursal pertenece a esa organización, (3) no está de baja y (4) existe la cuenta vinculada con ese dueño. Solo entonces firma una sesión nueva como esa cuenta (o como el propio dueño si el destino es la matriz). Cualquier otro caso: 403 `NOT_ORG_OWNER`, 404 (sucursal ajena, inexistente o id inválido, indistinguibles) o 409.
- Las sesiones de una cuenta vinculada llevan además la versión de sesión del dueño (`otv`). En cada petición se compara con `users.token_version` del dueño: si el dueño cambia su contraseña o su rol, se desactiva o deja de ser administrador, **todas sus sesiones de sucursal mueren en el acto**. Un token sin `otv`, con otro valor, o un token de cuenta normal que lo traiga, se rechaza.
- Como en el resto del producto, el soporte de plataforma ve lo mismo que hoy (lista y ficha de cada clínica, sin datos clínicos); las rutas `/org/*` le responden 403 como cualquier ruta de clínica.
- Auditoría: `org_created`, `branch_created`, `branch_switch` (quién entró a qué sucursal, con el nombre del dueño real), `branch_suspended`, `branch_reactivated`. El expediente y `record_access` siguen registrándose en la sucursal donde ocurre el acceso.
- 2FA: la cuenta vinculada hereda el estado de verificación en dos pasos del dueño (el dueño ya la pasó para obtener su sesión).

## Plan, suscripción y lugares

- **Una sola suscripción por organización: la de la matriz.** Un trigger de base de datos copia `plan`, `billing_status`, `trial_ends_at`, `current_period_end` y los campos de suspensión de la matriz a todas sus sucursales cuando cambian (los escriba plataforma, Mercado Pago o un pago manual), y una sucursal no puede conservar valores distintos. Así `requireCobros`, `requireSubscription`, los límites de lugares y el portal funcionan sin cambios.
- **Los lugares (`seats`) se cuentan por sucursal**, con los límites del plan de la matriz (cada sucursal es una clínica con su equipo). La cuenta vinculada no consume lugar.
- **Sucursales por plan** (`Plan.MaxBranches`, incluyendo la matriz): Básico 1, Crecimiento 1, Pro 10. Se cuentan las sucursales *en servicio*; al llegar al tope, `POST /org/branches` y la reactivación responden 409 con código `BRANCH_LIMIT`. En Básico ni siquiera se crea la organización.
- El pago del plan se hace desde la matriz: `/billing/checkout` desde una sucursal responde 409, y bajar de plan por debajo de las sucursales en servicio responde `BRANCH_LIMIT`. Si la plataforma baja el plan a mano con sucursales de más, estas siguen funcionando pero no se pueden crear ni reactivar otras.
- Limitación conocida: suspender o cambiar el plan de una sucursal individual desde el panel de plataforma no tiene efecto (el trigger lo reemplaza con los valores de la matriz); se administra la matriz.

## Baja lógica

`POST /org/branches/{id}/suspend` marca `clinics.branch_suspended_at`. La sucursal se bloquea igual que una clínica sin suscripción (`SUBSCRIPTION_REQUIRED`), deja de aceptar reservas en línea, portal y lista de espera, y **no se borra ningún dato**. No se puede dar de baja la matriz ni la sucursal en la que se está. Sigue apareciendo en los reportes. `POST /org/branches/{id}/reactivate` la devuelve (sujeta al límite del plan).

## Reportes consolidados (solo dueño)

`GET /org/reports/summary?from&to` y `/org/reports/summary.csv` (mismas reglas de rango que `/reports`: máximo 2 años). Las consultas usan `clinic_id = ANY(<ids de la organización>)`, leídos de la base en esa petición; **el cliente no manda ningún id**. Por sucursal y total: ventas pagadas, ticket promedio y cancelaciones del POS (las sucursales de planes sin cobros salen sin ventas), citas por estado, ausentismo (no asistió / citas pasadas no canceladas), consultas (`encounters.kind = 'consulta'`), pacientes nuevos y una serie diaria. Son solo conteos y sumas: no se lee ningún campo cifrado (`encfields.go` no se toca).

## Rutas

| Ruta | Quién |
|---|---|
| `GET /org` | cualquier cuenta de clínica (dueño: lista de sucursales; resto: `organization: null`) |
| `PUT /org` | dueño |
| `POST /org/branches` | dueño (o primer administrador, al fundar) |
| `POST /org/branches/{id}/suspend`, `/reactivate` | dueño |
| `POST /org/switch` | dueño |
| `GET /org/reports/summary[.csv]` | dueño |

## Pruebas

`backend/internal/api/org_test.go`: creación y límites por plan, cambio de contexto, intentos cruzados en pacientes, expediente, citas, ventas y archivos (lectura y escritura) con sesiones de matriz, sucursal, otra organización y médicos locales, tokens manipulados, bajas, suscripción compartida y reportes que no incluyen clínicas ajenas.
