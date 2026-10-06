<script lang="ts">
  const label = 'font-mono text-[10px] uppercase tracking-[0.14em] text-ink-faint';
  const days = ['Lun', 'Mar', 'Mié', 'Jue', 'Vie'];
  const blocks = [
    [0, 0, 2, 'dental'], [0, 3, 1, 'medica'], [1, 1, 2, 'medica'], [1, 4, 2, 'dental'], [2, 0, 1, 'dental'],
    [2, 2, 2, 'dental'], [3, 1, 1, 'medica'], [3, 3, 2, 'medica'], [4, 0, 2, 'dental'], [4, 4, 1, 'medica']
  ] as const;
</script>

<div>
  <div class="flex items-end justify-between">
    <div><p class={label}>Semana 42</p><p class="font-display text-3xl leading-none">Agenda del equipo</p></div>
    <span class="flex gap-3 text-[11px] text-ink-soft">
      <span class="flex items-center gap-1.5"><span class="h-2 w-2 rounded-full bg-mint"></span>Dental</span>
      <span class="flex items-center gap-1.5"><span class="h-2 w-2 rounded-full bg-signal"></span>Médica</span>
    </span>
  </div>
  <div class="mt-6 grid grid-cols-5 gap-2">
    {#each days as d, di}
      <div>
        <p class="pb-2 text-center text-[12px] font-medium text-ink-soft">{d}</p>
        <div class="relative h-56 rounded-xl bg-paper sm:h-64">
          {#each blocks.filter((b) => b[0] === di) as b, i}
            <span
              class="a-scaley absolute inset-x-1 rounded-md border-l-2 {b[3] === 'dental' ? 'border-mint bg-mint-soft' : 'border-signal bg-signal-soft'}"
              style="top: {b[1] * 16 + 3}%; height: {b[2] * 16 - 2}%; --d: {0.15 + (di + i) * 0.05}s"
            ></span>
          {/each}
        </div>
      </div>
    {/each}
  </div>
</div>
