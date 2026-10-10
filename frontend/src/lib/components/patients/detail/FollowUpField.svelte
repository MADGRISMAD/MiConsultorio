<script lang="ts">
  import { onMount } from 'svelte';
  import { agendaApi } from '$lib/api/agenda';
  import { session } from '$lib/session.svelte';
  import { CLINIC_KINDS } from '$lib/types';
  import type { Professional } from '$lib/types/agenda';

  interface Props {
    date: string;
    time: string;
    /** who sees the patient: empty is me; someone else of the team makes it a referral */
    professionalId?: string;
    /** shown under the field */
    hint?: string;
    id?: string;
  }
  let { date = $bindable(), time = $bindable(), professionalId = $bindable(''), hint = '', id = 'followup' }: Props = $props();
  const today = new Date().toISOString().slice(0, 10);

  let team = $state<Professional[]>([]);
  onMount(async () => {
    try {
      team = (await agendaApi.professionals()).filter((p) => p.consults);
    } catch {
      team = []; // without the list the field still works for the usual follow-up
    }
  });
  const me = $derived(session.user?.userId ?? '');
  const area = (p: Professional) => (p.areas ?? []).map((a) => CLINIC_KINDS[a as keyof typeof CLINIC_KINDS]?.label ?? a).join(', ') || p.specialty;
  const others = $derived(team.filter((p) => p.id !== me));
  const referral = $derived(!!professionalId && professionalId !== me);
</script>

<fieldset class="rounded-2xl border border-app-ink/10 p-4">
  <legend class="section-title px-1">{referral ? 'Derivar al paciente' : 'Cita de seguimiento recomendada'} <span class="font-normal normal-case text-app-muted">(opcional)</span></legend>
  {#if others.length}
    <div class="mb-3">
      <label class="label" for="{id}-pro">Con quién</label>
      <select id="{id}-pro" class="field" bind:value={professionalId}>
        <option value="">Conmigo (seguimiento)</option>
        {#each others as p (p.id)}<option value={p.id}>Derivar a {p.name}{area(p) ? ` · ${area(p)}` : ''}</option>{/each}
      </select>
    </div>
  {/if}
  <div class="grid gap-3 sm:grid-cols-2">
    <div>
      <label class="label" for="{id}-date">Fecha</label>
      <input id="{id}-date" type="date" class="field" min={today} bind:value={date} />
    </div>
    <div>
      <label class="label" for="{id}-time">Hora <span class="font-normal text-app-muted">(si la dejas vacía se toma el primer horario libre)</span></label>
      <input id="{id}-time" type="time" class="field" bind:value={time} disabled={!date} />
    </div>
  </div>
  <p class="hint">{hint || (referral ? 'Se agenda con esa persona como pendiente de confirmar, indica que tú lo derivaste y le llega un aviso. El motivo de la consulta se envía como razón de la derivación.' : 'Se agrega a la agenda como pendiente de confirmar: recepción la confirma con el paciente.')}</p>
</fieldset>
