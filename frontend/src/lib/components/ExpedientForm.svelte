<script lang="ts">
  import type { CheckboxField, ExpedientInput } from '$lib/types';

  let { data = $bindable() }: { data: ExpedientInput } = $props();

  const today = new Date().toISOString().slice(0, 10);

  const history: [string, CheckboxField][] = [
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
  const habits: [string, CheckboxField][] = [
    ['Tabaquismo', 'tabaquism'],
    ['Alcoholismo', 'alcoholism'],
    ['Se automédica', 'automedication'],
    ['Usa drogas', 'drug_use']
  ];
</script>

{#snippet text(label: string, key: 'names' | 'last_names' | 'CURP' | 'education' | 'occupation' | 'weight' | 'clothes_size' | 'height' | 'ethnicity' | 'physical_activity' | 'hobbies' | 'child', required = false)}
  <label class="block text-left">
    <span class="mb-1 block text-xs font-semibold text-ink-soft">{label}{required ? ' *' : ''}</span>
    <input type="text" class="field" bind:value={data[key]} {required} autocomplete="off" />
  </label>
{/snippet}

{#snippet check(label: string, key: CheckboxField)}
  <label class="flex items-center gap-2 text-sm">
    <input type="checkbox" class="h-4 w-4 rounded border-ink/30 text-signal focus:ring-signal" bind:checked={data[key]} />
    {label}
  </label>
{/snippet}

<div class="space-y-6 text-left">
  <fieldset>
    <legend class="mb-3 w-full rounded-lg bg-paper px-4 py-2 text-center text-sm font-semibold">Datos del paciente</legend>
    <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
      {@render text('Nombre(s)', 'names', true)}
      {@render text('Apellido(s)', 'last_names', true)}
      <label class="block text-left">
        <span class="mb-1 block text-xs font-semibold text-ink-soft">CURP *</span>
        <input type="text" class="field uppercase" bind:value={data.CURP} required maxlength="18" minlength="18" autocomplete="off" />
      </label>
      <label class="block text-left">
        <span class="mb-1 block text-xs font-semibold text-ink-soft">Sexo *</span>
        <select class="field" bind:value={data.sex}>
          <option value="Hombre">Masculino</option>
          <option value="Mujer">Femenino</option>
        </select>
      </label>
      <label class="block text-left">
        <span class="mb-1 block text-xs font-semibold text-ink-soft">Fecha de nacimiento *</span>
        <input type="date" class="field" min="1900-01-01" max={today} bind:value={data.date_of_birth} required />
      </label>
      {@render text('Escolaridad', 'education')}
      {@render text('Ocupación', 'occupation')}
      {@render text('Peso', 'weight')}
      {@render text('Talla', 'clothes_size')}
      {@render text('Estatura (cm)', 'height')}
      {@render text('Etnia', 'ethnicity')}
    </div>
  </fieldset>

  <fieldset>
    <legend class="mb-3 w-full rounded-lg bg-paper px-4 py-2 text-center text-sm font-semibold">Antecedentes patológicos y heredofamiliares</legend>
    <div class="grid grid-cols-2 gap-3 sm:grid-cols-3">
      {#each history as [label, key]}{@render check(label, key)}{/each}
    </div>
  </fieldset>

  <div class="grid gap-6 sm:grid-cols-2">
    <fieldset>
      <legend class="mb-3 w-full rounded-lg bg-paper px-4 py-2 text-center text-sm font-semibold">Hábitos de salud</legend>
      <div class="grid grid-cols-2 gap-3">
        {#each habits as [label, key]}{@render check(label, key)}{/each}
      </div>
      <div class="mt-3 grid gap-3">
        {@render text('Actividad física', 'physical_activity')}
        {@render text('Pasatiempos', 'hobbies')}
      </div>
    </fieldset>
    <fieldset>
      <legend class="mb-3 w-full rounded-lg bg-paper px-4 py-2 text-center text-sm font-semibold">En mujeres</legend>
      <div class="grid gap-3">
        {@render check('Está embarazada', 'pregnant')}
        {@render text('¿Cuántos hijos tiene?', 'child')}
      </div>
    </fieldset>
  </div>
</div>
