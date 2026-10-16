<script lang="ts">
  /** Maquetas de producto por especialidad: HTML/SVG decorativo, con datos de ejemplo. */
  let { kind }: { kind: string } = $props();

  const teeth = [
    ['ok', 'ok', 'done', 'ok', 'ok', 'caries', 'ok', 'ok'],
    ['ok', 'done', 'ok', 'ok', 'missing', 'ok', 'ok', 'done']
  ];
  const tooth: Record<string, string> = {
    ok: 'bg-paper/85',
    done: 'bg-mint',
    caries: 'bg-amber-400',
    missing: 'bg-transparent ring-1 ring-paper/25'
  };
  const bars = [62, 78, 70, 88, 74, 58, 66];
  const mood = [4, 5, 5, 6, 6, 7];
  const moodPath = mood.map((m, i) => `${i === 0 ? 'M' : 'L'}${10 + i * 36} ${70 - m * 8}`).join(' ');
</script>

<div class="text-paper">
  {#if kind === 'dental'}
    <p class="font-mono text-[10px] uppercase tracking-[0.14em] text-paper/55">Odontograma · Mariana López</p>
    <div class="mt-4 space-y-3">
      {#each teeth as row}
        <div class="flex justify-between">
          {#each row as t}<span class="block h-10 w-7 rounded-[10px_10px_15px_15px] sm:w-8 {tooth[t]}"></span>{/each}
        </div>
      {/each}
    </div>
    <div class="mt-4 flex flex-wrap gap-x-4 gap-y-1 text-[11px] text-paper/60">
      <span class="flex items-center gap-1.5"><i class="h-2 w-2 rounded-full bg-mint"></i>Tratado</span>
      <span class="flex items-center gap-1.5"><i class="h-2 w-2 rounded-full bg-amber-400"></i>Caries</span>
      <span class="flex items-center gap-1.5"><i class="h-2 w-2 rounded-full ring-1 ring-paper/40"></i>Ausente</span>
    </div>
    <div class="mt-5 grid grid-cols-3 gap-2 text-[12px]">
      {#each ['Fase 1 · Limpieza', 'Fase 2 · Resina', 'Fase 3 · Control'] as f, i}
        <span class="rounded-xl px-3 py-2 {i === 0 ? 'bg-mint text-white' : 'bg-paper/8 text-paper/75'}">{f}</span>
      {/each}
    </div>
  {:else if kind === 'medico'}
    <p class="font-mono text-[10px] uppercase tracking-[0.14em] text-paper/55">Signos vitales · Consulta de hoy</p>
    <div class="mt-4 grid grid-cols-2 gap-2.5">
      {#each [['Presión', '120/80', 'mmHg'], ['Pulso', '72', 'lpm'], ['Temperatura', '36.6', '°C'], ['Saturación', '98', '%']] as [l, v, u]}
        <div class="rounded-2xl bg-paper/8 p-3.5"><p class="text-[11px] text-paper/55">{l}</p><p class="mt-1 font-display text-3xl leading-none">{v}<span class="ml-1 text-sm text-paper/55">{u}</span></p></div>
      {/each}
    </div>
    <div class="mt-3 flex items-center justify-between rounded-2xl bg-paper/8 px-4 py-3 text-[13px]">
      <span>Faringitis aguda</span><span class="rounded-full bg-signal px-2.5 py-0.5 font-mono text-[10px] text-white">CIE-10 · J02.9</span>
    </div>
    <svg viewBox="0 0 220 44" class="mt-3 h-11 w-full" fill="none" aria-hidden="true"><path d="M2 34 C30 30 40 14 70 20 S110 38 140 22 S190 6 218 12" stroke="rgb(var(--l-mint))" stroke-width="2.5" stroke-linecap="round" /></svg>
  {:else if kind === 'nutricion'}
    <p class="font-mono text-[10px] uppercase tracking-[0.14em] text-paper/55">Plan de 7 días · 1,850 kcal al día</p>
    <div class="mt-5 flex h-28 items-end justify-between gap-2">
      {#each bars as h, i}
        <div class="flex h-full flex-1 flex-col items-center justify-end gap-2">
          <span class="w-full rounded-t-lg {i === 3 ? 'bg-mint' : 'bg-paper/20'}" style="height: {h}%"></span>
          <span class="text-[11px] text-paper/55">{'LMXJVSD'[i]}</span>
        </div>
      {/each}
    </div>
    <div class="mt-5 flex h-3 overflow-hidden rounded-full"><span class="w-1/4 bg-mint"></span><span class="w-1/2 bg-signal"></span><span class="w-1/4 bg-amber-400"></span></div>
    <div class="mt-2 flex justify-between text-[11px] text-paper/60"><span>Proteínas 25%</span><span>Carbohidratos 50%</span><span>Grasas 25%</span></div>
    <p class="mt-4 rounded-xl bg-paper/8 px-3 py-2 text-[12px] text-paper/75">Borrador con IA · tú lo revisas y lo ajustas</p>
  {:else if kind === 'veterinaria'}
    <p class="font-mono text-[10px] uppercase tracking-[0.14em] text-paper/55">Propietario · Jorge Medina</p>
    <div class="mt-4 space-y-3">
      {#each [['Luna', 'Labrador · 3 años', ['Rabia ✓', 'Triple ✓', 'Desparasitar']], ['Miel', 'Gato · 5 años', ['Rabia ✓', 'Triple ✓']]] as [n, d, v]}
        <div class="rounded-2xl bg-paper/8 p-4">
          <div class="flex items-center gap-3">
            <span class="grid h-10 w-10 place-items-center rounded-full bg-mint font-display text-lg text-white">{String(n)[0]}</span>
            <span><span class="block text-[15px] font-medium">{n}</span><span class="block text-[12px] text-paper/60">{d}</span></span>
          </div>
          <div class="mt-3 flex flex-wrap gap-1.5">
            {#each v as t}<span class="rounded-full px-2.5 py-1 text-[11px] {String(t).includes('✓') ? 'bg-paper/12 text-paper/80' : 'bg-amber-400 text-ink'}">{t}</span>{/each}
          </div>
        </div>
      {/each}
    </div>
  {:else if kind === 'pediatria'}
    <p class="font-mono text-[10px] uppercase tracking-[0.14em] text-paper/55">Curva de crecimiento · Peso</p>
    <svg viewBox="0 0 240 110" class="mt-4 w-full" fill="none" aria-hidden="true">
      <path d="M8 98 C60 82 120 60 232 24" stroke="rgb(var(--l-paper) / 0.18)" stroke-width="2" />
      <path d="M8 88 C60 68 120 44 232 10" stroke="rgb(var(--l-paper) / 0.35)" stroke-width="2" stroke-dasharray="4 4" />
      <path d="M8 104 C60 94 120 78 232 44" stroke="rgb(var(--l-paper) / 0.18)" stroke-width="2" />
      <path d="M8 96 C50 82 96 60 150 44" stroke="rgb(var(--l-mint))" stroke-width="3" stroke-linecap="round" />
      <circle cx="150" cy="44" r="6" fill="rgb(var(--l-mint))" /><circle cx="150" cy="44" r="12" fill="rgb(var(--l-mint) / 0.25)" />
    </svg>
    <ul class="mt-4 space-y-2 text-[13px]">
      {#each [['Hexavalente · 2.ª dosis', true], ['Rotavirus', true], ['Neumococo · refuerzo', false]] as [t, ok]}
        <li class="flex items-center justify-between rounded-xl bg-paper/8 px-3 py-2.5"><span>{t}</span><span class="text-[11px] {ok ? 'text-mint' : 'text-amber-400'}">{ok ? 'Aplicada' : 'Pendiente'}</span></li>
      {/each}
    </ul>
  {:else if kind === 'psicologia'}
    <p class="font-mono text-[10px] uppercase tracking-[0.14em] text-paper/55">Seguimiento por sesión · Ánimo</p>
    <svg viewBox="0 0 200 80" class="mt-4 w-full" fill="none" aria-hidden="true">
      <path d={moodPath} stroke="rgb(var(--l-mint))" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" />
      {#each mood as m, i}<circle cx={10 + i * 36} cy={70 - m * 8} r="4" fill="rgb(var(--l-paper))" />{/each}
    </svg>
    <div class="mt-1 flex justify-between px-1 text-[10px] text-paper/50">{#each [1, 2, 3, 4, 5, 6] as s}<span>S{s}</span>{/each}</div>
    <div class="mt-4 rounded-2xl bg-paper/8 p-4">
      <p class="flex items-center gap-2 text-[12px] text-paper/60"><svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><rect x="5" y="11" width="14" height="9" rx="2" /><path d="M8 11V8a4 4 0 0 1 8 0v3" /></svg>Nota privada · solo tú puedes leerla</p>
      <p class="mt-2 h-2 w-4/5 rounded-full bg-paper/15"></p><p class="mt-2 h-2 w-3/5 rounded-full bg-paper/15"></p>
    </div>
    <p class="mt-3 rounded-xl bg-paper/8 px-3 py-2 text-[12px] text-paper/75">Consentimiento firmado en pantalla ✓</p>
  {:else}
    <p class="font-mono text-[10px] uppercase tracking-[0.14em] text-paper/55">Esquema corporal · Zonas de dolor</p>
    <div class="mt-4 grid grid-cols-[auto_1fr] items-center gap-6">
      <svg viewBox="0 0 90 170" class="h-44" fill="none" stroke="rgb(var(--l-paper) / 0.45)" stroke-width="2" stroke-linecap="round" aria-hidden="true">
        <circle cx="45" cy="16" r="11" /><path d="M32 34h26l6 44H26z" /><path d="M26 78l-6 66M64 78l6 66M38 78l-2 84M52 78l2 84M32 34l-16 36M58 34l16 36" />
        <circle class="pain" cx="45" cy="50" r="5" fill="rgb(var(--l-mint))" stroke="none" /><circle class="pain" cx="36" cy="92" r="5" fill="rgb(var(--l-mint))" stroke="none" />
      </svg>
      <div class="space-y-4 text-[12px]">
        {#each [['Visita anterior', 60, 'bg-paper/30'], ['Hoy', 30, 'bg-mint']] as [l, w, c]}
          <div><div class="flex justify-between text-paper/65"><span>{l}</span><span>{Number(w) / 10}/10</span></div><div class="mt-1.5 h-2 rounded-full bg-paper/10"><span class="block h-2 rounded-full {c}" style="width: {w}%"></span></div></div>
        {/each}
        <p class="rounded-xl bg-paper/8 px-3 py-2 text-paper/75">Plan de 8 sesiones · 3 completadas</p>
      </div>
    </div>
  {/if}
</div>

<style>
  @media (prefers-reduced-motion: no-preference) {
    .pain {
      animation: ping 1.8s ease-out infinite;
      transform-origin: center;
      transform-box: fill-box;
    }
  }
  @keyframes ping {
    50% {
      transform: scale(1.5);
      opacity: 0.6;
    }
  }
</style>
