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

  // One way or the other: "in N days" or an exact day chosen from the calendar
  let mode = $state<'days' | 'date'>('days');
  let days = $state<number | null>(null);
  const addDays = (n: number) => {
    const d = new Date();
    d.setDate(d.getDate() + n);
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
  };
  function setMode(m: 'days' | 'date') {
    if (mode === m) return;
    mode = m;
    date = time = '';
    days = null;
    slots = [];
  }
  function pickDays(n: number | null) {
    days = n && n > 0 ? Math.min(Math.floor(n), 730) : null;
    date = days ? addDays(days) : '';
    time = '';
  }
  const shown = (iso: string) => new Date(`${iso}T12:00:00`).toLocaleDateString('es-MX', { weekday: 'long', day: 'numeric', month: 'long' });

  // free times of the chosen day, for the person who will see the patient
  let slots = $state<{ start: string; end: string }[]>([]);
  let loadingSlots = $state(false);
  let seq = 0;
  $effect(() => {
    const d = date;
    const pro = professionalId;
    if (mode !== 'date' || !d) {
      slots = [];
      return;
    }
    const my = ++seq;
    loadingSlots = true;
    agendaApi
      .freeSlots(d, pro)
      .then((r) => my === seq && (slots = r))
      .catch(() => my === seq && (slots = []))
      .finally(() => my === seq && (loadingSlots = false));
  });
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

  <div class="mb-3 inline-flex gap-1 rounded-full bg-app-ink/5 p-1" role="group" aria-label="Cómo elegir la fecha">
    <button type="button" aria-pressed={mode === 'days'} class="rounded-full px-3.5 py-1.5 text-sm font-medium transition {mode === 'days' ? 'bg-app-panel text-app-ink shadow-sm' : 'text-app-muted hover:text-app-ink'}" onclick={() => setMode('days')}>En tantos días</button>
    <button type="button" aria-pressed={mode === 'date'} class="rounded-full px-3.5 py-1.5 text-sm font-medium transition {mode === 'date' ? 'bg-app-panel text-app-ink shadow-sm' : 'text-app-muted hover:text-app-ink'}" onclick={() => setMode('date')}>Elegir fecha en el calendario</button>
  </div>

  {#if mode === 'days'}
    <div class="flex flex-wrap items-end gap-2">
      {#each [7, 15, 30, 60, 90] as n}
        <button type="button" class="rounded-full px-3 py-1.5 text-sm ring-1 ring-inset ring-app-ink/15 hover:bg-app-elevated {days === n ? 'bg-app-primary/10 ring-app-primary' : ''}" onclick={() => pickDays(days === n ? null : n)}>{n} días</button>
      {/each}
      <div>
        <label class="label" for="{id}-days">Otro número</label>
        <input id="{id}-days" type="number" min="1" max="730" class="field w-28" value={days ?? ''} oninput={(e) => pickDays(e.currentTarget.valueAsNumber)} />
      </div>
    </div>
    {#if date}<p class="mt-2 text-sm">Le tocaría el <strong>{shown(date)}</strong>, en el primer horario libre de ese día.</p>{/if}
  {:else}
    <div class="grid gap-3 sm:grid-cols-2">
      <div>
        <label class="label" for="{id}-date">Fecha</label>
        <input id="{id}-date" type="date" class="field" min={today} bind:value={date} onchange={() => (time = '')} />
      </div>
      <div>
        <p class="label">Hora <span class="font-normal text-app-muted">(si no eliges, se toma el primer horario libre)</span></p>
        {#if !date}
          <p class="text-sm text-app-muted">Elige primero el día.</p>
        {:else if loadingSlots}
          <p class="text-sm text-app-muted">Buscando horarios…</p>
        {:else if slots.length === 0}
          <p class="text-sm text-app-muted">No hay horarios libres ese día. Prueba con otra fecha.</p>
        {:else}
          <div class="flex max-h-28 flex-wrap gap-1.5 overflow-y-auto" role="group" aria-label="Horarios libres">
            {#each slots as s (s.start)}
              <button type="button" aria-pressed={time === s.start} class="rounded-full px-3 py-1 text-sm ring-1 ring-inset ring-app-ink/15 hover:bg-app-elevated {time === s.start ? 'bg-app-primary/10 ring-app-primary' : ''}" onclick={() => (time = time === s.start ? '' : s.start)}>{s.start}</button>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  {/if}
  <p class="hint mt-2">{hint || (referral ? 'Se agenda con esa persona como pendiente de confirmar, indica que tú lo derivaste y le llega un aviso.' : 'Se agrega a la agenda como pendiente de confirmar: recepción la confirma con el paciente.')} Si el paciente ya tiene otra cita pendiente en este giro, se te avisará; si ya lo están atendiendo, sí se puede agendar.</p>
</fieldset>
