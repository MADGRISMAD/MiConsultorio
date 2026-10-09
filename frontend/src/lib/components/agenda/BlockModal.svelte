<script lang="ts">
  import { agendaApi } from '$lib/api/agenda';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { Appt, Professional, TimeBlock } from '$lib/types/agenda';
  import Modal from '../Modal.svelte';
  import Icon from '../ui/Icon.svelte';
  import { fmtShort, fullName } from './util';

  interface Props {
    open: boolean;
    blocks: TimeBlock[];
    pros: Professional[];
    /** a specialist without agenda-admin rights: the only professional they may block (themselves) */
    ownOnly?: string | null;
    /** date to prefill */
    date: string;
    onclose: () => void;
    onchanged: () => void;
  }
  let { open, blocks: allBlocks, pros, ownOnly = null, date, onclose, onchanged }: Props = $props();
  const blocks = $derived(ownOnly ? allBlocks.filter((b) => b.professional_id === ownOnly) : allBlocks);

  let professional = $state('');
  let from = $state('');
  let to = $state('');
  let allDay = $state(true);
  let start = $state('09:00');
  let end = $state('10:00');
  let reason = $state('');
  let affected = $state<Appt[] | null>(null);
  const op = new Op();
  const delOp = new Op();

  $effect(() => {
    if (open) {
      from = to = date;
      professional = ownOnly ?? '';
      allDay = true;
      reason = '';
      affected = null;
      op.reset();
    }
  });

  const proName = (id: string | null) => (id ? (pros.find((p) => p.id === id)?.name ?? 'Profesional') : 'Todo el consultorio');

  async function create(e: SubmitEvent) {
    e.preventDefault();
    const ok = await op.run(async () => {
      const r = await agendaApi.createBlock({
        professional_id: professional || null, date_from: from, date_to: to || from,
        startHour: allDay ? '' : start, endHour: allDay ? '' : end, reason
      });
      affected = r.affected;
      toast.show('Bloqueo creado');
    });
    if (ok) onchanged();
  }
  async function remove(b: TimeBlock) {
    if (await delOp.run(() => agendaApi.deleteBlock(b.id))) {
      toast.show('Bloqueo quitado');
      onchanged();
    }
  }
</script>

<Modal {open} title={ownOnly ? "Marcar no disponible" : "Bloquear horarios"} wide {onclose}>
  <form id="block-form" class="grid gap-3 text-left sm:grid-cols-2" onsubmit={create}>
    {#if ownOnly}
      <p class="rounded-xl bg-app-primary/8 px-3.5 py-2.5 text-sm sm:col-span-2">Los pacientes no podrán agendar contigo en las fechas y horas que marques (por ejemplo, vacaciones). Las citas que ya tengas no se cancelan.</p>
    {:else}
    <label class="block sm:col-span-2">
      <span class="label">A quién aplica</span>
      <select class="field" bind:value={professional}>
        <option value="">Todo el consultorio</option>
        {#each pros as p (p.id)}<option value={p.id}>{p.name}</option>{/each}
      </select>
    </label>
    {/if}
    <label class="block"><span class="label">Desde *</span><input type="date" class="field" bind:value={from} required /></label>
    <label class="block"><span class="label">Hasta *</span><input type="date" class="field" bind:value={to} min={from} required /></label>
    <label class="flex cursor-pointer items-center gap-2 text-sm sm:col-span-2">
      <input type="checkbox" bind:checked={allDay} class="h-4 w-4 accent-[rgb(var(--app-primary))]" /> Todo el día
    </label>
    {#if !allDay}
      <label class="block"><span class="label">De las</span><input type="time" step="300" class="field" bind:value={start} required /></label>
      <label class="block"><span class="label">A las</span><input type="time" step="300" class="field" bind:value={end} required /></label>
    {/if}
    <label class="block sm:col-span-2">
      <span class="label">Motivo</span>
      <input type="text" class="field" bind:value={reason} maxlength="200" placeholder="Vacaciones, congreso, mantenimiento…" />
    </label>
    <div class="flex justify-end sm:col-span-2">
      <button type="submit" class="btn-primary" disabled={op.phase === 'loading'}>{#if op.phase === 'loading'}<span class="spin"></span>{/if}{ownOnly ? 'Marcar no disponible' : 'Crear bloqueo'}</button>
    </div>
  </form>
  {#if op.phase === 'error'}<p class="alert mt-3" role="alert"><Icon name="alert" size={18} />{op.message}</p>{/if}

  {#if affected}
    <div class="mt-4 rounded-xl bg-app-warning/12 px-3.5 py-3 text-sm" role="status">
      {#if affected.length === 0}
        <p class="font-medium text-app-ink">No hay citas dentro de ese bloqueo.</p>
      {:else}
        <p class="font-medium text-app-warning">Ya hay {affected.length} {affected.length === 1 ? 'cita' : 'citas'} en ese horario. No se cancelaron: reprográmalas o cancélalas desde la agenda.</p>
        <ul class="mt-2 grid gap-1 text-app-ink">
          {#each affected as a (a.id)}<li>{fmtShort(a.date)} {a.startHour} · {fullName(a)}</li>{/each}
        </ul>
      {/if}
    </div>
  {/if}

  <h3 class="section-title mb-2 mt-6">Bloqueos próximos</h3>
  {#if blocks.length === 0}
    <p class="text-sm text-app-muted">No hay bloqueos en el periodo que se está viendo.</p>
  {:else}
    <ul class="divide-y divide-app-ink/8 rounded-xl border border-app-ink/10">
      {#each blocks as b (b.id)}
        <li class="flex items-center gap-3 px-3.5 py-2.5 text-sm">
          <div class="min-w-0 flex-1">
            <p class="truncate font-medium">{b.reason || 'Sin motivo'} <span class="font-normal text-app-muted">· {proName(b.professional_id)}</span></p>
            <p class="text-xs text-app-muted">{fmtShort(b.date_from)}{b.date_to !== b.date_from ? ` – ${fmtShort(b.date_to)}` : ''} · {b.startHour ? `${b.startHour}–${b.endHour}` : 'todo el día'}</p>
          </div>
          <button type="button" class="icon-btn danger" aria-label="Quitar el bloqueo {b.reason}" onclick={() => remove(b)}><Icon name="trash" size={18} /></button>
        </li>
      {/each}
    </ul>
    {#if delOp.phase === 'error'}<p class="alert mt-3" role="alert">{delOp.message}</p>{/if}
  {/if}
  {#snippet footer()}<button type="button" class="btn-secondary" onclick={onclose}>Cerrar</button>{/snippet}
</Modal>
