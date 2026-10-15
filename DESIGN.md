# DESIGN.md — sistema de diseño de Caresia

Obligatorio para cualquier cambio de interfaz en `frontend/`. Se extrajo del código existente (`frontend/tailwind.config.ts`, `frontend/src/app.css`, `frontend/src/lib/components/ui/*`). **No se inventa un estilo nuevo**: si algo no está aquí, copia el patrón de una pantalla vecina y, si de verdad hace falta algo nuevo, agrégalo aquí en el mismo commit.

Carácter: **editorial clínico**. Papel azulado, tinta azul marino, titulares serif grandes con un acento en cursiva, etiquetas en mono mayúsculas, píldoras redondas y tarjetas muy redondeadas. Calmado, sin ruido, sin colores fuera de la paleta.

## 1. Color (solo tokens `app-*`, nunca hex)
Los valores viven en variables CSS en `app.css` y cambian solos con el tema claro/oscuro.

| Token | Uso |
|---|---|
| `app-ink` | Texto principal, fondo del botón primario. Para trazos y divisores: `border-app-ink/10`, `divide-app-ink/8`, hover `bg-app-ink/5` |
| `app-muted` | Texto secundario, etiquetas, metadatos |
| `app-surface` | Fondo de página |
| `app-panel` | Fondo de tarjetas y campos |
| `app-elevated` | Hover de botón secundario, fondos elevados |
| `app-primary` (azul) | Acento, enlaces, estado "info", cursiva del titular |
| `app-accent` (menta) | Estado correcto / en curso |
| `app-warning` | Atención y avisos |
| `app-danger` | Errores y acciones destructivas |

Opacidades válidas: 6, 8, 10, 12, 14 y las estándar de Tailwind. Estados semánticos: `pill-info`, `pill-ok`, `pill-warn`, `pill-bad` o el componente `Pill`.

## 2. Tipografía
- **Titulares y cifras protagonistas:** clase `display` (Instrument Serif, tracking −0.02em). Título de página: `display text-[2.25rem] leading-[1] sm:text-5xl` (así lo hace `PageHeader`). Título de tarjeta: `display text-3xl`. Nombre protagonista (paciente en "Ahora"): `display text-4xl sm:text-5xl leading-none`.
- **Acento en cursiva:** una sola palabra del titular, `<em class="italic text-app-primary">…</em>`, y cierra con punto. Ejemplo: «Tu consultorio, *Ana.*».
- **Etiqueta de sección:** clase `section-title` (JetBrains Mono 11px, mayúsculas, tracking 0.14em, `app-muted`), siempre encima del titular, en una línea corta.
- **Cuerpo:** Instrument Sans, 15px (`text-[15px]`) en descripciones, `text-sm` en listas.
- **Números de hora/cantidades:** `tabular-nums`.
- Prohibido: `text-xl font-semibold` como titular de página o de tarjeta, mayúsculas fuera de `section-title`/`th`.

## 3. Formas y superficies
- Tarjeta: clase `card` (radio 22px, borde `app-ink/10`, sombra `shadow-app`). Relleno `p-5 sm:p-6` (hero `p-6 sm:p-8`). No uses `rounded-xl` suelto para tarjetas.
- Campos: clase `field` (min-h 44px, `rounded-xl`).
- Botones: **píldora** (`rounded-full`). `btn-primary` (tinta, pasa a azul en hover), `btn-secondary` (panel con anillo), `btn-ghost`, `btn-danger`. Altura mínima 40px; acciones principales `min-h-12`; objetivos táctiles ≥ 44px.
- Estados/etiquetas: `pill`, `badge` (redondos). Avisos: `note` (warning) o `alert` (error).
- Vacío: `EmptyState` o `card-empty`. Carga: esqueleto con `card` + `animate-pulse bg-app-ink/5` (respetando `motion-safe`).
- Espacio: separación entre tarjetas `gap-4`/`space-y-4`; entre cabecera y contenido `mb-7` (PageHeader).

## 4. Componentes y patrones que se reutilizan
`PageHeader`, `Pill`, `Icon`, `EmptyState`, `Alert`, `Avatar`, `Segmented`, `Combobox`, `Modal`, `ConfirmModal`, `Spinner`, `Guard`. Antes de escribir markup nuevo busca aquí y en `frontend/src/lib/components/`.
- Una fila de lista: hora o fecha a la izquierda (`text-lg font-semibold tabular-nums w-14`), nombre `font-semibold truncate`, detalle `text-sm text-app-muted`, estado en `Pill` a la derecha, separadores `divide-y divide-app-ink/8`.
- Cabecera de tarjeta: `display text-3xl` a la izquierda, enlace `text-sm font-semibold text-app-primary hover:underline` a la derecha.
- Punto de estado junto a una etiqueta: `h-2 w-2 rounded-full bg-app-accent` (en curso) o `bg-app-warning` (esperando).

## 5. Movimiento
Sobrio. Entrada `rise` (500 ms, `out-strong`), `transition` de 150–200 ms, `active:scale` en botones. Siempre respeta `prefers-reduced-motion`. Sin animaciones decorativas en pantallas de trabajo.

## 6. Accesibilidad y móvil
Mobile first a 390 px; objetivos táctiles ≥ 44px; foco visible (el global de `app.css`); `aria-live="polite"` en datos que se actualizan solos; contraste con los tokens, sin colores propios; texto siempre en español es-MX, breve.

## 7. Inicio del consultorio (`frontend/src/routes/+page.svelte`, `frontend/src/lib/components/home/*`)
Objetivo: en 3 segundos, "¿a quién atiendo ahora y qué sigue?". Orden fijo:
1. **Cabecera editorial compacta:** `section-title` con el saludo («Buenos días»), titular `display` «Tu consultorio, *Nombre.*» a tamaño de página (§2, no más grande) y debajo el nombre de la clínica con `badge` del giro. Sin teléfono ni dirección.
2. **Barra de acciones:** buscador de pacientes (`field`, `/` o Ctrl/⌘+K) + `btn-secondary` «Nueva cita» y «Consulta sin cita», siempre visibles según permisos. Nada que repita el menú lateral.
3. **Tarjeta «Ahora»** (`card`, hero): `section-title` con punto de estado, nombre `display` grande, una sola acción primaria `btn-primary min-h-12`. Si es temprano, en vez de botón gris: píldora «Disponible a las HH:MM» (`rounded-full bg-app-primary/10 text-app-primary`) y «Expediente» activo.
4. **Requiere atención:** solo si hay algo, máximo 3, como `card` con `section-title` y enlaces con conteo en `pill pill-warn`. Nunca bloques de color propios.
5. **Agenda de hoy** (`card`, título `display text-3xl`): filas del §4, una sola acción (botón) en la cita atendible, el resto abre la cita; atendidas colapsadas en «Atendidas (n)».
6. **Mañana:** `card` con título `display text-3xl`, máximo 3 filas y «Ver agenda».
No volver a poner: tarjeta «Cobros» (duplica el menú), chips de navegación, banner de cookies dentro de la app.

## 8. Lista de verificación antes de entregar
- [ ] Solo tokens `app-*`; ningún hex ni `rounded-xl` donde corresponde `card`/píldora.
- [ ] Titulares con `display`; etiquetas con `section-title`; acento en cursiva solo en el titular de página.
- [ ] Botones con `btn-*`; estados con `pill`/`Pill`; vacío con `EmptyState`.
- [ ] Se parece visualmente a las pantallas vecinas (abre `/agenda`, `/pacientes` y compara).
- [ ] Capturas en 1440 px y 390 px, tema claro y oscuro.
- [ ] `npm run check`, `npm run knip` y las e2e que toquen la pantalla.
