<script lang="ts">
  import type { BodyFinding, BodyView } from '$lib/types/specialty';
  import { center, intensityColor, kindLabel, VIEW_H, VIEW_W, worstByZone, zonesFor } from './bodyGeometry';

  interface Props {
    view: BodyView;
    findings: BodyFinding[];
    selected?: string | null;
    onpick?: (zone: string) => void;
  }
  let { view, findings, selected = null, onpick }: Props = $props();

  const zones = $derived(zonesFor(view));
  const worst = $derived(worstByZone(findings, view));
  let tab = $state<string | null>(null);
  const stop = $derived(tab && zones.some((z) => z.id === tab) ? tab : zones[0]?.id);
  let svg = $state<SVGSVGElement>();

  function describe(id: string, label: string) {
    const here = findings.filter((f) => f.view === view && f.zone === id);
    if (!here.length) return `${label}, sin hallazgos`;
    return `${label}: ${here.map((f) => `${kindLabel(f.kind).toLowerCase()} ${f.intensity} de 10`).join(', ')}`;
  }
  function move(ev: KeyboardEvent, id: string) {
    const i = zones.findIndex((z) => z.id === id);
    let next = -1;
    if (ev.key === 'ArrowRight' || ev.key === 'ArrowDown') next = (i + 1) % zones.length;
    else if (ev.key === 'ArrowLeft' || ev.key === 'ArrowUp') next = (i - 1 + zones.length) % zones.length;
    else if (ev.key === 'Enter' || ev.key === ' ') {
      onpick?.(id);
      ev.preventDefault();
      return;
    } else return;
    ev.preventDefault();
    tab = zones[next].id;
    svg?.querySelector<SVGGElement>(`[data-zone="${zones[next].id}"]`)?.focus();
  }
</script>

<svg
  bind:this={svg}
  viewBox="0 0 {VIEW_W} {VIEW_H}"
  class="mx-auto block h-auto w-full max-w-[220px]"
  role="group"
  aria-label="Figura humana, vista {view === 'front' ? 'frontal' : 'posterior'}. Usa las flechas para recorrer las zonas y Enter para elegir una."
>
  {#each zones as z (z.id)}
    {@const f = worst.get(z.id)}
    {@const [cx, cy] = center(z)}
    <g
      data-zone={z.id}
      role="button"
      tabindex={stop === z.id ? 0 : -1}
      aria-label={describe(z.id, z.label)}
      aria-pressed={selected === z.id}
      class="cursor-pointer outline-none [&:focus-visible>.shape]:stroke-app-primary [&:focus-visible>.shape]:stroke-[3]"
      onfocus={() => (tab = z.id)}
      onkeydown={(ev) => move(ev, z.id)}
      onclick={() => onpick?.(z.id)}
    >
      <title>{z.label}</title>
      {#if z.kind === 'rect'}
        <rect class="shape {f ? '' : 'fill-app-surface'} {selected === z.id ? 'stroke-app-primary' : 'stroke-app-ink/40'}" fill={f ? intensityColor(f.intensity) : undefined} x={z.x} y={z.y} width={z.w} height={z.h} rx="7" stroke-width={selected === z.id ? 3 : 1} />
      {:else}
        <ellipse class="shape {f ? '' : 'fill-app-surface'} {selected === z.id ? 'stroke-app-primary' : 'stroke-app-ink/40'}" fill={f ? intensityColor(f.intensity) : undefined} cx={z.x} cy={z.y} rx={z.w} ry={z.h} stroke-width={selected === z.id ? 3 : 1} />
      {/if}
      {#if f}<text x={cx} y={cy + 4} text-anchor="middle" font-size="12" font-weight="700" fill="#111" aria-hidden="true">{f.intensity}</text>{/if}
    </g>
  {/each}
</svg>
