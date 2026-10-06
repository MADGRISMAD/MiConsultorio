<script lang="ts">
  import { CLINIC_KIND_KEYS, CLINIC_KINDS, type ClinicKind } from '$lib/types';
  import Icon from '../ui/Icon.svelte';

  interface Props {
    kind: ClinicKind;
    specialties: ClinicKind[];
  }
  let { kind = $bindable(), specialties = $bindable() }: Props = $props();

  function pick(k: ClinicKind) {
    kind = k;
    specialties = specialties.filter((s) => s !== k); // the main kind is not repeated
  }
  function toggle(k: ClinicKind) {
    specialties = specialties.includes(k) ? specialties.filter((s) => s !== k) : [...specialties, k];
  }
</script>

<fieldset>
  <legend class="label">¿Qué tipo de consultorio es?</legend>
  <p class="hint mb-3 !mt-0">Elige el principal. Lo usamos para ajustar la experiencia a tu especialidad.</p>
  <div class="grid grid-cols-2 gap-2.5 sm:grid-cols-3" role="radiogroup" aria-label="Tipo de consultorio">
    {#each CLINIC_KIND_KEYS as k}
      {@const on = kind === k}
      <button
        type="button"
        role="radio"
        aria-checked={on}
        class="flex items-center gap-3 rounded-xl border-2 p-3 text-left transition {on ? 'border-app-primary bg-app-primary/8' : 'border-app-ink/10 hover:border-app-ink/25'}"
        onclick={() => pick(k)}
      >
        <span class="grid h-9 w-9 flex-none place-items-center rounded-lg {on ? 'bg-app-primary text-app-on-primary' : 'bg-app-ink/8 text-app-muted'}"><Icon name={CLINIC_KINDS[k].icon} size={19} /></span>
        <span class="min-w-0">
          <strong class="block truncate text-sm font-semibold {on ? 'text-app-primary' : ''}">{CLINIC_KINDS[k].label}</strong>
          <small class="block truncate text-xs text-app-muted">{CLINIC_KINDS[k].hint}</small>
        </span>
      </button>
    {/each}
  </div>
</fieldset>

<fieldset class="mt-6">
  <legend class="label">¿Atienden otras especialidades? <span class="font-normal text-app-muted">(opcional)</span></legend>
  <div class="flex flex-wrap gap-2">
    {#each CLINIC_KIND_KEYS.filter((k) => k !== kind) as k}
      {@const on = specialties.includes(k)}
      <button
        type="button"
        aria-pressed={on}
        class="inline-flex items-center gap-1.5 rounded-full px-3.5 py-1.5 text-sm font-medium transition {on ? 'bg-app-ink text-app-surface' : 'bg-app-ink/6 text-app-muted hover:bg-app-ink/10 hover:text-app-ink'}"
        onclick={() => toggle(k)}
      >
        {#if on}<Icon name="check" size={14} stroke={2.6} />{/if}{CLINIC_KINDS[k].label}
      </button>
    {/each}
  </div>
</fieldset>
