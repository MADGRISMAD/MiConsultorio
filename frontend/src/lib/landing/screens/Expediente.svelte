<script lang="ts">
  const label = 'font-mono text-[10px] uppercase tracking-[0.14em] text-ink-faint';
  const rows = [['CURP', 'MEJJ700312HDFDRR04'], ['Edad', '54 años'], ['Peso', '78 kg'], ['Estatura', '172 cm'], ['Ocupación', 'Contador'], ['Actividad física', '3 veces / semana']];
  const ant: [string, boolean][] = [['Diabetes', true], ['Alergias', false], ['Cardiopatías', false], ['Cirugías', true], ['Fracturas', false], ['Transfusiones', false]];
</script>

<div>
  <div class="flex items-center gap-4">
    <span class="grid h-14 w-14 place-items-center rounded-full bg-signal-soft font-display text-2xl text-signal">JM</span>
    <div><p class={label}>Expediente</p><p class="font-display text-3xl leading-none">Jorge Medina</p></div>
  </div>
  <dl class="mt-6 grid grid-cols-2 gap-x-6 gap-y-3 border-t border-ink/10 pt-5">
    {#each rows as [k, v], i}
      <div class="a-up8" style="--d: {0.1 + i * 0.05}s; --t: 0.5s">
        <dt class={label}>{k}</dt>
        <dd class="mt-0.5 truncate text-[14px] font-medium tabular-nums">{v}</dd>
      </div>
    {/each}
  </dl>
  <p class="{label} mt-6">Antecedentes patológicos</p>
  <ul class="mt-2 flex flex-wrap gap-1.5">
    {#each ant as [a, on], i}
      <li class="a-pop rounded-full px-3 py-1 text-[12px] font-medium {on ? 'bg-ink text-paper' : 'bg-paper text-ink-faint'}" style="--d: {0.4 + i * 0.05}s; --t: 0.4s">{a}</li>
    {/each}
  </ul>
</div>
