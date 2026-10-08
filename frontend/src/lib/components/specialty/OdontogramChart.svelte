<script lang="ts">
  import type { OdontogramData, Surface } from '$lib/types/specialty';
  import { CELL, glyph, isUpper, ROW_H, rowsFor, rowWidth, stateDef, SURFACE_NAMES, surfaceShapes, toothKind, toothSummary, toothX } from './odontoGeometry';

  interface Props {
    data: OdontogramData;
    selected?: number | null;
    highlight?: Set<number>;
    readonly?: boolean;
    /** a surface (or the whole tooth when surface is null) was activated */
    onpick?: (tooth: number, surface: Surface | null) => void;
    /** a number or clear key was pressed on a focused tooth */
    onkey?: (tooth: number, key: string) => void;
  }
  let { data, selected = null, highlight = new Set<number>(), readonly = false, onpick, onkey }: Props = $props();

  const rows = $derived(rowsFor(data.dentition));
  const width = $derived(Math.max(...rows.map(rowWidth)));
  const GAP_Y = 8;
  const height = $derived(rows.length * (ROW_H + GAP_Y));
  let svg = $state<SVGSVGElement>();
  let tab = $state<number | null>(null); // roving tabindex

  const order = $derived(rows.flatMap((r) => r.teeth));
  const stop = $derived(tab != null && order.includes(tab) ? tab : (order[0] ?? null));

  function label(n: number): string {
    const sum = toothSummary(data.teeth[String(n)]);
    const note = data.teeth[String(n)]?.note;
    return `Pieza ${n}, ${toothKind(n).toLowerCase()} ${isUpper(n) ? 'superior' : 'inferior'}${sum ? `: ${sum}` : ', sin hallazgos'}${note ? `. Nota: ${note}` : ''}`;
  }
  function focusTooth(n: number | undefined) {
    if (n == null) return;
    tab = n;
    svg?.querySelector<SVGGElement>(`[data-tooth="${n}"]`)?.focus();
  }
  function keydown(ev: KeyboardEvent, n: number) {
    const ri = rows.findIndex((r) => r.teeth.includes(n));
    const row = rows[ri];
    const i = row.teeth.indexOf(n);
    if (ev.key === 'ArrowRight') focusTooth(row.teeth[Math.min(i + 1, row.teeth.length - 1)]);
    else if (ev.key === 'ArrowLeft') focusTooth(row.teeth[Math.max(i - 1, 0)]);
    else if (ev.key === 'ArrowDown' && rows[ri + 1]) focusTooth(rows[ri + 1].teeth[Math.min(i, rows[ri + 1].teeth.length - 1)]);
    else if (ev.key === 'ArrowUp' && rows[ri - 1]) focusTooth(rows[ri - 1].teeth[Math.min(i, rows[ri - 1].teeth.length - 1)]);
    else if (ev.key === 'Home') focusTooth(row.teeth[0]);
    else if (ev.key === 'End') focusTooth(row.teeth[row.teeth.length - 1]);
    else if (ev.key === 'Enter' || ev.key === ' ') onpick?.(n, null);
    else if (!readonly && /^[0-9]$/.test(ev.key)) onkey?.(n, ev.key);
    else if (!readonly && (ev.key === 'Delete' || ev.key === 'Backspace')) onkey?.(n, 'clear');
    else return;
    ev.preventDefault();
  }
</script>

<div class="w-full max-w-full overflow-x-auto rounded-2xl border border-app-ink/10 bg-app-panel p-3">
  <svg
    bind:this={svg}
    viewBox="0 0 {width} {height}"
    style="min-width: {Math.min(width, 640)}px; width: 100%; max-width: 760px"
    class="mx-auto block"
    role="group"
    aria-label="Odontograma, notación FDI. Usa las flechas para moverte entre piezas, Enter para seleccionar y los números para aplicar un estado a la pieza."
  >
    {#each rows as row, ri (ri)}
      {@const y0 = ri * (ROW_H + GAP_Y)}
      {#each row.teeth as n, i (n)}
        {@const x = toothX(row, i)}
        {@const t = data.teeth[String(n)]}
        {@const gy = y0 + (row.arch === 'upper' ? 15 : 3)}
        <text x={x + CELL / 2} y={y0 + (row.arch === 'upper' ? 11 : CELL + 15)} font-size="10" text-anchor="middle" class="fill-app-muted" aria-hidden="true">{n}</text>
        <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
        <g
          data-tooth={n}
          role="button"
          tabindex={stop === n ? 0 : -1}
          aria-label={label(n)}
          aria-pressed={selected === n}
          transform="translate({x} {gy})"
          class="cursor-pointer outline-none [&:focus-visible>rect.ring]:stroke-app-primary"
          onfocus={() => (tab = n)}
          onkeydown={(ev) => keydown(ev, n)}
          onclick={() => onpick?.(n, null)}
        >
          <rect class="ring fill-transparent stroke-transparent" x="-3" y="-3" width={CELL + 6} height={CELL + 6} rx="7" stroke-width="2.5" />
          {#if selected === n}<rect x="-4" y="-4" width={CELL + 8} height={CELL + 8} rx="8" class="fill-app-primary/10 stroke-app-primary" stroke-width="2" />{/if}
          {#if highlight.has(n)}<rect x="-4" y="-4" width={CELL + 8} height={CELL + 8} rx="8" fill="none" stroke="#f59e0b" stroke-width="2.5" stroke-dasharray="4 3" />{/if}
          {#each surfaceShapes(n) as sh (sh.surface)}
            {@const st = t?.surfaces?.[sh.surface]}
            <polygon
              points={sh.points}
              class="stroke-app-ink/50 {st ? '' : 'fill-app-surface'}"
              fill={st ? (stateDef(st)?.color ?? '#999') : undefined}
              stroke-width="0.9"
              onclick={(ev) => {
                if (readonly) return;
                ev.stopPropagation();
                onpick?.(n, sh.surface);
              }}
              role="presentation"
            ><title>{SURFACE_NAMES[sh.surface]}</title></polygon>
          {/each}
          {#if t?.state}{@html glyph(t.state)}{/if}
          {#if t?.note}<circle cx={CELL - 3} cy="3" r="3.5" class="fill-app-primary" />{/if}
        </g>
      {/each}
    {/each}
  </svg>
</div>
