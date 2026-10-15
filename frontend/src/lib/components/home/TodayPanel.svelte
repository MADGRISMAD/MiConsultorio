<script lang="ts">
  import { addDays, ymd } from '$lib/dates';
  import { goto } from '$app/navigation';
  import { onDestroy, onMount } from 'svelte';
  import { agendaApi } from '$lib/api/agenda';
  import { beginConsult, consultUrl } from '$lib/consultFlow';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import { PERMISSIONS } from '$lib/types';
  import { isActive, STATUS_META, type Appt } from '$lib/types/agenda';
  import AttentionStrip from './AttentionStrip.svelte';
  import Icon from '../ui/Icon.svelte';
  import Pill from '../ui/Pill.svelte';

  /**
   * «Hoy»: lo que sigue en el consultorio y el siguiente paso en un clic.
   * Una cita en consulta se retoma, una que ya llegó pasa a consulta, y la siguiente se inicia (marca llegada, la pasa
   * a consulta y abre el expediente con la nota lista, ligada a la cita). Se actualiza sola.
   */
  let list = $state<Appt[]>([]);
  let tomorrow = $state<Appt[]>([]);
  let loaded = $state(false);
  let failed = $state(false);
  let now = $state(Date.now());
  let busy = $state('');
  const op = new Op();
  let timer: ReturnType<typeof setInterval> | undefined;
  let tick: ReturnType<typeof setInterval> | undefined;


  async function load() {
    const today = ymd(new Date());
    try {
      const appointments = await agendaApi.list({ from: today, to: addDays(today, 1) });
      list = appointments.filter((a) => a.date === today);
      tomorrow = appointments.filter((a) => a.date !== today && isActive(a) && a.status !== 'completed' && (session.user?.role !== 'doctor' || a.professional_id === session.user.userId)).sort((a, b) => a.startHour.localeCompare(b.startHour));
      failed = false;
    } catch {
      failed = list.length === 0;
    } finally {
      loaded = true;
    }
  }
  onMount(() => {
    load();
    timer = setInterval(load, 30000);
    tick = setInterval(() => (now = Date.now()), 15000);
  });
  onDestroy(() => {
    clearInterval(timer);
    clearInterval(tick);
  });

  const canConsult = $derived(session.has(PERMISSIONS.adminHistorials));
  // un médico ve su día; recepción y administración ven el de todo el consultorio
  const mine = $derived(
    (session.user?.role === 'doctor' ? list.filter((a) => a.professional_id === session.user?.userId) : list).slice().sort((a, b) => a.startHour.localeCompare(b.startHour))
  );
  const open = $derived(mine.filter((a) => isActive(a) && a.status !== 'completed'));
  const inRoom = $derived(open.filter((a) => a.status === 'in_progress'));
  const waiting = $derived(open.filter((a) => a.status === 'arrived').sort((a, b) => (a.arrived_at ?? '').localeCompare(b.arrived_at ?? '')));
  const coming = $derived(open.filter((a) => a.status === 'scheduled' || a.status === 'confirmed'));
  const tooEarly = (a: Appt) => !!a.opens_at && Date.parse(a.opens_at) > now;
  const focus = $derived(inRoom[0] ?? waiting[0] ?? coming.find((a) => !tooEarly(a)) ?? coming[0]);

  const name = (a: Appt) => `${a.names} ${a.last_names}`.trim();
  const what = (a: Appt) => a.service_name || a.details || 'Consulta';
  const hhmm = (iso: string | null) => (iso ? new Date(iso).toLocaleTimeString('es-MX', { hour: '2-digit', minute: '2-digit', hour12: false }) : '');
  function since(iso: string | null): string {
    if (!iso) return '';
    const m = Math.max(0, Math.floor((now - Date.parse(iso)) / 60000));
    if (m < 1) return 'hace un momento';
    if (m < 60) return `hace ${m} min`;
    return `hace ${Math.floor(m / 60)} h ${m % 60} min`;
  }

  /** Lo que se puede hacer ahora con una cita, según su estado, el permiso de quien mira y la hora. */
  type Step = { label: string; icon: 'stethoscope' | 'check' | 'user-plus' | 'file'; run: () => void; disabled?: boolean; hint?: string };
  function stepFor(a: Appt): Step | null {
    if (a.status === 'completed') return a.patient_id ? { label: 'Expediente', icon: 'file', run: () => goto(`/pacientes/${a.patient_id}`) } : null;
    if (!isActive(a)) return null;
    if (!a.patient_id) {
      return canConsult ? { label: 'Registrar paciente', icon: 'user-plus', run: () => goto(`/pacientes/nuevo?cita=${encodeURIComponent(a.id)}`) } : null;
    }
    if (a.status === 'in_progress') return canConsult ? { label: 'Continuar consulta', icon: 'stethoscope', run: () => goto(consultUrl(a)) } : null;
    const early = tooEarly(a);
    const hint = early ? `Disponible a las ${hhmm(a.opens_at)}` : undefined;
    if (!canConsult) {
      // recepción: solo anuncia la llegada; el médico toma al paciente desde su inicio
      return a.status === 'arrived' ? null : { label: 'Marcar llegada', icon: 'check', run: () => arrive(a), disabled: early, hint };
    }
    return { label: a.status === 'arrived' ? 'Pasar a consulta' : 'Iniciar consulta', icon: 'stethoscope', run: () => start(a), disabled: early, hint };
  }

  async function arrive(a: Appt) {
    busy = a.id;
    const ok = await op.run(async () => void (await agendaApi.setStatus(a.id, 'arrived')));
    busy = '';
    if (ok) {
      toast.show(`${name(a)} marcado como llegado`);
      await load();
    } else toast.show(op.message, 'error');
  }

  async function start(a: Appt) {
    busy = a.id;
    const ok = await op.run(async () => void (await beginConsult(a)));
    if (ok) await goto(consultUrl(a));
    else {
      toast.show(op.message, 'error');
      await load();
    }
    busy = '';
  }

  const closed = $derived(mine.filter((a) => !isActive(a) || a.status === 'completed'));
  const remaining = $derived(open.filter((a) => a.id !== focus?.id));
  const canReadPatient = $derived(session.has(PERMISSIONS.navHistorials) || canConsult);
  const href = (a: Appt) => a.patient_id && canReadPatient ? `/pacientes/${encodeURIComponent(a.patient_id)}` : `/admin/navegar-citas?cita=${encodeURIComponent(a.id)}`;
  const done = $derived(mine.filter((a) => a.status === 'completed').length);
  const summary = $derived(
    [
      `${open.length + done} ${open.length + done === 1 ? 'cita' : 'citas'} hoy`,
      waiting.length ? `${waiting.length} en espera` : '',
      done ? `${done} ${done === 1 ? 'atendida' : 'atendidas'}` : ''
    ]
      .filter(Boolean)
      .join(' · ')
  );
  const focusStep = $derived(focus ? stepFor(focus) : null);
</script>

<section aria-label="Hoy" class="space-y-4">
  {#if !loaded}
    <div class="card h-52 motion-safe:animate-pulse bg-app-ink/5" aria-busy="true"></div>
    <div class="card h-64 motion-safe:animate-pulse bg-app-ink/5" aria-busy="true"></div>
  {:else if failed}
    <div class="card p-6 text-sm text-app-muted" role="alert">
      No pudimos cargar tus citas de hoy.
      <button type="button" class="btn-secondary min-h-11" onclick={load}>Reintentar</button>
    </div>
  {:else}
    <section aria-label="Ahora" class="card border-app-primary/25 p-4 sm:p-6">
      <p class="section-title">Ahora{focus ? focus.status === 'in_progress' ? ' · En consulta' : focus.status === 'arrived' ? ' · En espera' : ' · Siguiente cita' : ''}</p>
      {#if focus}
        <div class="mt-2 flex flex-wrap items-center justify-between gap-4">
          <div class="min-w-0 flex-1 basis-48">
            <h2 class="display break-words text-3xl leading-tight sm:text-4xl">{name(focus)}</h2>
            <p class="mt-1 text-sm text-app-muted">
              {#if focus.status === 'in_progress'}Empezó {since(focus.started_at)}
              {:else if focus.status === 'arrived'}Llegó {since(focus.arrived_at)}
              {:else}<span class="font-semibold tabular-nums text-app-ink">{focus.startHour}</span>{/if}
              · {what(focus)}{focus.professional_name && session.user?.role !== 'doctor' ? ` · ${focus.professional_name}` : ''}
            </p>
            {#if !focus.patient_id}<p class="mt-1 text-sm text-app-warning">Aún no está registrado como paciente.</p>{/if}
          </div>
          <div class="flex w-full flex-wrap items-center gap-2 sm:w-auto">
            {#if focusStep?.disabled}
              <p class="flex min-h-12 items-center gap-2 rounded-xl bg-app-primary/10 px-4 text-sm font-medium text-app-primary"><Icon name="clock" size={18} />{focusStep.hint}</p>
            {:else if focusStep}
              <button type="button" class="btn-primary min-h-12 w-full px-5 sm:w-auto" disabled={busy === focus.id} onclick={focusStep.run}><Icon name={focusStep.icon} size={20} />{focusStep.label}</button>
            {/if}
            {#if focus.patient_id && canReadPatient && focus.status !== 'in_progress'}<a class="btn-secondary min-h-11" href="/pacientes/{focus.patient_id}">Expediente</a>{/if}
          </div>
        </div>
      {:else}
        <h2 class="display mt-2 text-3xl">{mine.length ? 'No quedan citas pendientes.' : 'No hay citas hoy.'}</h2>
        <p class="mt-2 text-sm text-app-muted">Tu agenda está libre por ahora.</p>
      {/if}
    </section>

    <AttentionStrip today={mine} {tomorrow} {now} />

    <section aria-label="Agenda de hoy" class="card p-4 sm:p-6">
      <div class="mb-3 flex flex-wrap items-baseline justify-between gap-2">
        <h2 class="display text-2xl">Agenda de hoy</h2>
        <span class="text-sm text-app-muted" aria-live="polite">{summary}</span>
      </div>
      {#if focus}
        <a href={href(focus)} class="mb-2 flex min-h-11 items-center justify-between gap-3 rounded-lg bg-app-primary/8 px-3 text-sm text-app-primary">
          <span class="font-semibold tabular-nums">{focus.startHour}</span><span>{focus.status === 'scheduled' || focus.status === 'confirmed' ? 'Siguiente' : 'Ahora'}</span>
        </a>
      {/if}
      <ul class="divide-y divide-app-ink/8 border-l border-app-ink/15 pl-3">
        {#each remaining as a (a.id)}
          {@const step = stepFor(a)}
          <li class="flex flex-wrap items-center gap-2 py-1">
            <a href={href(a)} class="flex min-h-14 min-w-0 flex-1 items-center gap-3 rounded-lg py-2 hover:bg-app-ink/5">
              <span class="w-12 flex-none text-sm font-semibold tabular-nums">{a.startHour}</span>
              <span class="min-w-0 flex-1"><span class="block truncate font-semibold">{name(a)}</span><span class="block truncate text-sm text-app-muted">{what(a)}{a.professional_name && session.user?.role !== 'doctor' ? ` · ${a.professional_name}` : ''}</span><Pill tone={STATUS_META[a.status].tone}>{STATUS_META[a.status].label}</Pill></span>
            </a>
            {#if step && !step.disabled && (a.status === 'in_progress' || a.status === 'arrived')}
              <button type="button" class="btn-secondary min-h-11" disabled={busy === a.id} onclick={step.run}>{step.label}</button>
            {/if}
          </li>
        {/each}
      </ul>
      {#if !mine.length}<p class="py-4 text-sm text-app-muted">No hay citas programadas.</p>{/if}
      {#if closed.length}
        <details class="mt-3 border-t border-app-ink/10 pt-2">
          <summary class="min-h-11 cursor-pointer py-3 text-sm font-semibold text-app-muted">Atendidas ({closed.length})</summary>
          <ul class="divide-y divide-app-ink/8">
            {#each closed as a (a.id)}
              <li><a href={href(a)} class="flex min-h-14 items-center gap-3 rounded-lg py-2 text-sm hover:bg-app-ink/5"><span class="tabular-nums">{a.startHour}</span><span class="min-w-0 flex-1 truncate">{name(a)}</span><Pill tone={STATUS_META[a.status].tone}>{STATUS_META[a.status].label}</Pill></a></li>
            {/each}
          </ul>
        </details>
      {/if}
    </section>
    {#if tomorrow.length}
      <section aria-label="Mañana" class="px-1 py-2">
        <div class="flex items-center justify-between"><h2 class="text-sm font-semibold text-app-muted">Mañana · {tomorrow.length} citas</h2><a class="flex min-h-11 items-center text-sm font-semibold text-app-primary" href="/admin/navegar-citas">Ver agenda</a></div>
        <ul>{#each tomorrow.slice(0, 3) as a (a.id)}<li><a href={href(a)} class="flex min-h-11 items-center gap-3 rounded-lg text-sm hover:bg-app-ink/5"><span class="tabular-nums text-app-muted">{a.startHour}</span><span class="truncate">{name(a)}</span></a></li>{/each}</ul>
      </section>
    {/if}
  {/if}
</section>
