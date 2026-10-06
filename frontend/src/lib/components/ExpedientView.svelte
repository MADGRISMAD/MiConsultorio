<script lang="ts">
  import type { Expedient } from '$lib/types';

  let { expedient }: { expedient: Expedient } = $props();

  const yn = (v: boolean) => (v ? 'Sí' : 'No');
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
</script>

{#snippet item(label: string, value: string | number)}
  <div>
    <dt class="text-xs font-semibold uppercase tracking-wide text-ink-faint">{label}</dt>
    <dd class="mt-0.5 break-words text-sm text-ink">{value || '—'}</dd>
  </div>
{/snippet}

<div class="space-y-6 text-left">
  <section>
    <h3 class="rounded-lg bg-paper px-4 py-2 text-center text-sm font-semibold">Datos del paciente</h3>
    <dl class="mt-4 grid grid-cols-2 gap-x-6 gap-y-4 sm:grid-cols-4">
      <div class="col-span-2">{@render item('Nombre completo', `${expedient.names} ${expedient.last_names}`)}</div>
      <div class="col-span-2">{@render item('CURP', expedient.CURP)}</div>
      {@render item('Sexo', expedient.sex)}
      {@render item('Escolaridad', expedient.education)}
      {@render item('Edad', expedient.age)}
      {@render item('Ocupación', expedient.occupation)}
      {@render item('Peso', expedient.weight)}
      {@render item('Talla', expedient.clothes_size)}
      {@render item('Estatura (cm)', expedient.height)}
      {@render item('Etnia', expedient.ethnicity)}
    </dl>
  </section>

  <section>
    <h3 class="rounded-lg bg-paper px-4 py-2 text-center text-sm font-semibold">Antecedentes patológicos y heredofamiliares</h3>
    <dl class="mt-4 grid grid-cols-2 gap-x-6 gap-y-4 sm:grid-cols-3">
      {#each history as [label, key]}
        {@render item(label, yn(expedient[key] as boolean))}
      {/each}
    </dl>
  </section>

  <div class="grid gap-6 sm:grid-cols-2">
    <section>
      <h3 class="rounded-lg bg-paper px-4 py-2 text-center text-sm font-semibold">Hábitos de salud</h3>
      <dl class="mt-4 grid grid-cols-2 gap-x-6 gap-y-4">
        {#each habits as [label, key]}
          {@render item(label, yn(expedient[key] as boolean))}
        {/each}
        {@render item('Actividad física', expedient.physical_activity)}
        {@render item('Pasatiempos', expedient.hobbies)}
      </dl>
    </section>
    {#if expedient.sex === 'Mujer'}
      <section>
        <h3 class="rounded-lg bg-paper px-4 py-2 text-center text-sm font-semibold">En mujeres</h3>
        <dl class="mt-4 grid grid-cols-2 gap-x-6 gap-y-4">
          {@render item('Está embarazada', yn(expedient.pregnant))}
          {@render item('¿Cuántos hijos tiene?', expedient.child || 'Ninguno')}
        </dl>
      </section>
    {/if}
  </div>
</div>
