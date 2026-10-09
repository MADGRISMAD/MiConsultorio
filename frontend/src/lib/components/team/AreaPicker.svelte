<script lang="ts">
  import { session } from '$lib/session.svelte';
  import { CLINIC_KINDS, type ClinicKind } from '$lib/types';

  interface Props {
    value: string[];
  }
  let { value = $bindable() }: Props = $props();

  /** the giros of this clinic: nobody is offered an area the clinic does not work in */
  const kinds = $derived(session.clinic ? ([session.clinic.kind, ...session.clinic.specialties.filter((k) => k !== session.clinic!.kind)] as ClinicKind[]) : []);

  function flip(k: string, on: boolean) {
    value = on ? [...value, k] : value.filter((x) => x !== k);
  }
</script>

{#if kinds.length > 1}
  <fieldset>
    <legend class="label">Áreas en las que atiende <span class="font-normal text-app-muted">(si no eliges ninguna, atiende en todas)</span></legend>
    <div class="grid gap-2 sm:grid-cols-2">
      {#each kinds as k}
        <label class="flex cursor-pointer items-center gap-2 rounded-xl border px-3 py-2 text-sm {value.includes(k) ? 'border-app-primary bg-app-primary/8' : 'border-app-ink/10'}">
          <input type="checkbox" class="accent-[rgb(var(--app-primary))]" checked={value.includes(k)} onchange={(e) => flip(k, e.currentTarget.checked)} />
          {CLINIC_KINDS[k].label}
        </label>
      {/each}
    </div>
    <p class="hint">Define a quién puede elegir el paciente al agendar en línea y qué registros de especialidad ve y edita.</p>
  </fieldset>
{/if}
