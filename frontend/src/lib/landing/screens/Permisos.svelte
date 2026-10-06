<script lang="ts">
  const label = 'font-mono text-[10px] uppercase tracking-[0.14em] text-ink-faint';
  const users: [string, string, boolean[]][] = [
    ['Dra. Ana Ruiz', 'Odontóloga', [true, true, true]],
    ['Dr. Luis Paredes', 'Médico general', [true, true, true]],
    ['Karla Núñez', 'Recepción', [true, false, false]],
    ['Tú', 'Administrador', [true, true, true]]
  ];
</script>

<div>
  <p class={label}>Usuarios</p>
  <p class="font-display text-3xl leading-none">Permisos por rol</p>
  <div class="mt-6 grid grid-cols-[1fr_repeat(3,3.5rem)] items-center gap-y-1 text-[12px] sm:grid-cols-[1fr_repeat(3,4.5rem)]">
    <span></span>
    {#each ['Agenda', 'Expedientes', 'Historial'] as h}<span class="{label} truncate text-center">{h}</span>{/each}
    {#each users as [name, role, perms], ui}
      <span class="min-w-0 border-t border-ink/5 py-3">
        <span class="block truncate text-[14px] font-medium">{name}</span>
        <span class="block text-ink-soft">{role}</span>
      </span>
      {#each perms as on, pi}
        <span class="flex justify-center border-t border-ink/5 py-3">
          <span class="relative h-5 w-9 rounded-full transition-colors {on ? 'bg-mint' : 'bg-ink/10'}">
            <span class="a-thumb absolute top-0.5 h-4 w-4 rounded-full bg-white shadow" style="left: {on ? 18 : 2}px; --d: {0.25 + (ui * 3 + pi) * 0.04}s; --t: 0.5s"></span>
          </span>
        </span>
      {/each}
    {/each}
  </div>
</div>
