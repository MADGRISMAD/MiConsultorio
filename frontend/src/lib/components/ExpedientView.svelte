<script lang="ts">
  import type { Expedient } from '$lib/types';

  let { expedient }: { expedient: Expedient } = $props();

  const history: [string, keyof Expedient][] = [
    ['Diabetes', 'diabetes'],
    ['Enf. reumáticas', 'rheumatic_diseases'],
    ['Fracturas', 'fractures'],
    ['Alergias', 'allergies'],
    ['Encames', 'layed'],
    ['Contracturas musculares', 'contractures'],
    ['Cáncer', 'cancer'],
    ['Accidentes', 'accidents'],
    ['Transfusiones', 'transfusions'],
    ['Cardiopatías', 'cardiopathies'],
    ['Cirugías', 'surgeries']
  ];
  const habits: [string, keyof Expedient][] = [
    ['Tabaquismo', 'tabaquism'],
    ['Alcoholismo', 'alcoholism'],
    ['Se automédica', 'automedication'],
    ['Usa drogas', 'drug_use']
  ];
  const alerts = $derived(history.filter(([, k]) => expedient[k]).map(([l]) => l));
</script>

{#snippet item(label: string, value: string | number)}
  <div>
    <dt class="text-xs font-semibold uppercase tracking-wide text-app-muted">{label}</dt>
    <dd class="mt-0.5 break-words text-sm font-semibold">{value || '—'}</dd>
  </div>
{/snippet}

{#snippet flag(label: string, on: boolean)}
  <div class="flex items-center justify-between gap-3 rounded-lg px-3 py-2 text-sm {on ? 'bg-app-warning/12' : 'bg-app-ink/5'}">
    <span class="font-semibold {on ? '' : 'text-app-muted'}">{label}</span>
    <span class="text-xs font-semibold uppercase {on ? 'text-app-warning' : 'text-app-muted'}">{on ? 'Sí' : 'No'}</span>
  </div>
{/snippet}

<div class="space-y-6 text-left">
  {#if alerts.length}
    <p class="flex flex-wrap items-center gap-2 rounded-xl bg-app-warning/12 px-4 py-3 text-sm font-semibold text-app-warning">
      <strong class="mr-1 uppercase">Antecedentes:</strong>{alerts.join(' · ')}
    </p>
  {/if}

  <section>
    <h3 class="section-title mb-3">Datos del paciente</h3>
    <dl class="grid grid-cols-2 gap-x-6 gap-y-4 sm:grid-cols-4">
      <div class="col-span-2">{@render item('Nombre completo', `${expedient.names} ${expedient.last_names}`)}</div>
      <div class="col-span-2">{@render item('CURP', expedient.CURP)}</div>
      {@render item('Sexo', expedient.sex)}
      {@render item('Edad', expedient.age)}
      {@render item('Escolaridad', expedient.education)}
      {@render item('Ocupación', expedient.occupation)}
      {@render item('Peso', expedient.weight)}
      {@render item('Talla', expedient.clothes_size)}
      {@render item('Estatura (cm)', expedient.height)}
      {@render item('Etnia', expedient.ethnicity)}
    </dl>
  </section>

  <section>
    <h3 class="section-title mb-3">Antecedentes patológicos y heredofamiliares</h3>
    <div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
      {#each history as [label, key]}{@render flag(label, expedient[key] as boolean)}{/each}
    </div>
  </section>

  <div class="grid gap-6 sm:grid-cols-2">
    <section>
      <h3 class="section-title mb-3">Hábitos de salud</h3>
      <div class="grid gap-2">
        {#each habits as [label, key]}{@render flag(label, expedient[key] as boolean)}{/each}
      </div>
      <dl class="mt-4 grid grid-cols-2 gap-4">
        {@render item('Actividad física', expedient.physical_activity)}
        {@render item('Pasatiempos', expedient.hobbies)}
      </dl>
    </section>
    {#if expedient.sex === 'Mujer'}
      <section>
        <h3 class="section-title mb-3">En mujeres</h3>
        <div class="grid gap-2">{@render flag('Está embarazada', expedient.pregnant)}</div>
        <dl class="mt-4">{@render item('¿Cuántos hijos tiene?', expedient.child || 'Ninguno')}</dl>
      </section>
    {/if}
  </div>
</div>
