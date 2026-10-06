<script lang="ts">
  const label = 'font-mono text-[10px] uppercase tracking-[0.14em] text-ink-faint';
  const items = [
    ['12 feb', 'Valoración inicial', 'Radiografía panorámica', 'dental'],
    ['04 mar', 'Colocación de brackets', 'Arcada superior e inferior', 'dental'],
    ['21 mar', 'Consulta general', 'Dolor de garganta · tratamiento indicado', 'medica'],
    ['08 abr', 'Primer ajuste', 'Sin molestias reportadas', 'dental']
  ] as const;
</script>

<div>
  <p class={label}>Historial clínico</p>
  <p class="font-display text-3xl leading-none">Diego Hernández</p>
  <ol class="relative mt-6 space-y-3">
    <span class="a-line absolute bottom-3 left-[5px] top-3 w-px bg-ink/15" style="--t: 0.9s"></span>
    {#each items as [d, t, n, tone], i}
      <li class="a-x16 relative pl-7" style="--d: {0.15 + i * 0.1}s">
        <span class="absolute left-0 top-4 h-[11px] w-[11px] rounded-full ring-4 ring-white {tone === 'dental' ? 'bg-mint' : 'bg-signal'}"></span>
        <div class="rounded-xl bg-paper px-4 py-3">
          <div class="flex items-baseline justify-between gap-3">
            <span class="text-[14px] font-medium">{t}</span>
            <span class="font-mono text-[11px] tabular-nums text-ink-faint">{d}</span>
          </div>
          <p class="mt-0.5 text-[13px] text-ink-soft">{n}</p>
        </div>
      </li>
    {/each}
  </ol>
</div>
