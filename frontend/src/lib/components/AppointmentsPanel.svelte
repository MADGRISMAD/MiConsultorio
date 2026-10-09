<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { ApiError } from '$lib/api';
  import { agendaApi } from '$lib/api/agenda';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import { PERMISSIONS } from '$lib/types';
  import {
    STATUS_META, emptyApptInput, inputFromAppt, type AgendaSettings, type Appt, type ApptInput, type Professional, type ServiceOption, type TimeBlock
  } from '$lib/types/agenda';
  import AppointmentDetail from './agenda/AppointmentDetail.svelte';
  import BlockModal from './agenda/BlockModal.svelte';
  import MonthGrid from './agenda/MonthGrid.svelte';
  import { agendaLoadApi } from '$lib/api/waitlist';
  import type { MonthLoadDay } from '$lib/types/waitlist';
  import TimeGrid from './agenda/TimeGrid.svelte';
  import { addDays, addMonths, dayKey, fmtShort, fullName, monthDays, proColor, rangeTitle, todayStr, toMin, visibleRange, weekStart } from './agenda/util';
  import AppointmentForm from './AppointmentForm.svelte';
  import ConfirmModal from './ConfirmModal.svelte';
  import Modal from './Modal.svelte';
  import EmptyState from './ui/EmptyState.svelte';
  import Icon from './ui/Icon.svelte';
  import LoadingRows from './ui/LoadingRows.svelte';
  import PageHeader from './ui/PageHeader.svelte';
  import Pill from './ui/Pill.svelte';

  let { admin = false }: { admin?: boolean } = $props();

  type View = 'day' | 'week' | 'month' | 'list';
  const VIEWS: [View, string][] = [['day', 'Día'], ['week', 'Semana'], ['month', 'Mes'], ['list', 'Lista']];

  /** may create, move and delete: the admin page, and only for people with the permission */
  const canEdit = $derived(admin && session.has(PERMISSIONS.adminAppointments));

  const saved = (k: string, d: string) => {
    try {
      return localStorage.getItem('agenda.' + k) ?? d;
    } catch {
      return d;
    }
  };
  const remember = (k: string, v: string) => {
    try {
      localStorage.setItem('agenda.' + k, v);
    } catch {
      /* private mode */
    }
  };

  let view = $state<View>('week');
  let cursor = $state(todayStr());
  let monthLoad = $state<Map<string, MonthLoadDay>>(new Map());
  let proFilter = $state('');
  let roomFilter = $state('');
  let statusFilter = $state('');
  let narrow = $state(false);

  let appts = $state<Appt[]>([]);
  let blocks = $state<TimeBlock[]>([]);
  let pros = $state<Professional[]>([]);
  /** a specialist without agenda-admin rights can still mark their own time as unavailable */
  const ownProId = $derived(!session.has(PERMISSIONS.adminAppointments) ? (pros.find((p) => p.id === session.user?.userId)?.id ?? null) : null);
  let services = $state<ServiceOption[]>([]);
  let cfg = $state<AgendaSettings | null>(null);
  let loading = $state(true);
  let loadError = $state('');

  onMount(() => {
    const v = saved('view', '') as View;
    const mq = window.matchMedia('(max-width: 639px)');
    narrow = mq.matches;
    view = VIEWS.some(([k]) => k === v) ? v : mq.matches ? 'day' : 'week';
    const on = (e: MediaQueryListEvent) => (narrow = e.matches);
    mq.addEventListener('change', on);
    Promise.all([agendaApi.professionals(), agendaApi.settings(), agendaApi.services().catch(() => [] as ServiceOption[])])
      .then(([p, s, sv]) => {
        pros = p;
        cfg = s;
        services = sv;
      })
      .catch(() => {});
    session.loadClinic();
    return () => mq.removeEventListener('change', on);
  });

  const days = $derived.by(() => {
    if (view === 'day') return [cursor];
    if (view === 'week') return Array.from({ length: 7 }, (_, i) => addDays(weekStart(cursor), i));
    return [];
  });
  const range = $derived.by((): [string, string] => {
    if (view === 'day') return [cursor, cursor];
    if (view === 'week') return [days[0], days[6]];
    if (view === 'month') {
      const m = monthDays(cursor);
      return [m[0], m[41]];
    }
    return [cursor.slice(0, 8) + '01', addDays(addMonths(cursor, 1), -1)];
  });

  let seq = 0;
  async function load(silent = false) {
    const mine = ++seq;
    if (!silent) loading = true;
    try {
      const [from, to] = range;
      const [a, b] = await Promise.all([
        agendaApi.list({ from, to, professional: proFilter || undefined, room: roomFilter || undefined, status: statusFilter || undefined }),
        agendaApi.blocks(from, to)
      ]);
      if (mine !== seq) return;
      appts = a;
      if (view === 'month') {
        agendaLoadApi
          .monthLoad(cursor.slice(0, 7), proFilter)
          .then((days) => { if (mine === seq) monthLoad = new Map(days.map((d) => [d.date, d])); })
          .catch(() => { monthLoad = new Map(); });
      }
      blocks = proFilter ? b.filter((x) => !x.professional_id || x.professional_id === proFilter) : b;
      loadError = '';
    } catch (e) {
      if (mine === seq) loadError = e instanceof Error ? e.message : 'No se pudieron cargar las citas.';
    } finally {
      if (mine === seq) loading = false;
    }
  }
  $effect(() => {
    range;
    proFilter;
    roomFilter;
    statusFilter;
    untrack(() => load());
  });

  function setView(v: View) {
    view = v;
    remember('view', v);
  }
  function step(dir: -1 | 1) {
    cursor = view === 'day' ? addDays(cursor, dir) : view === 'week' ? addDays(cursor, 7 * dir) : addMonths(cursor, dir);
  }

  const rooms = $derived.by(() => {
    const set = new Set(cfg?.rooms ?? []);
    for (const a of appts) if (a.room) set.add(a.room);
    return [...set];
  });
  const slot = $derived(Math.min(Math.max(cfg?.slot_minutes ?? session.clinic?.settings?.appointment_minutes ?? 30, 5), 60));
  const gridRange = $derived(visibleRange(session.clinic?.settings, days, appts));
  const isClosed = (d: string) => {
    const h = session.clinic?.settings?.hours?.[dayKey(d)];
    return !!h && !h.open;
  };

  // ----- details / create / edit -----
  let viewing = $state<Appt | null>(null);
  let formOpen = $state(false);
  let editingId = $state<string | null>(null);
  let form = $state<ApptInput>(emptyApptInput());
  let conflict = $state<{ code: string; message: string } | null>(null);
  const formOp = new Op();

  function openCreate(date = cursor < todayStr() ? todayStr() : cursor, start = '') {
    editingId = null;
    form = { ...emptyApptInput(), date, startHour: start, professional_id: proFilter || null, room: roomFilter };
    if (start) {
      const end = toMin(start) + slot;
      form.endHour = `${String(Math.floor(end / 60) % 24).padStart(2, '0')}:${String(end % 60).padStart(2, '0')}`;
    }
    conflict = null;
    formOp.reset();
    viewing = null;
    formOpen = true;
  }
  function openEdit(a: Appt) {
    editingId = a.id;
    form = inputFromAppt(a);
    conflict = null;
    formOp.reset();
    viewing = null;
    formOpen = true;
  }
  async function submitForm(e: SubmitEvent) {
    e.preventDefault();
    const id = editingId;
    conflict = null;
    const input = $state.snapshot(form);
    try {
      formOp.phase = 'loading';
      const saved = id ? await agendaApi.update(id, input) : await agendaApi.create(input);
      formOp.reset();
      formOpen = false;
      toast.show(id ? 'Cita actualizada' : 'Cita creada');
      cursor = saved.date;
      await load(true);
    } catch (err) {
      if (err instanceof ApiError && (err.code === 'SLOT_TAKEN' || err.code === 'SLOT_BLOCKED')) {
        conflict = { code: err.code, message: err.message };
        formOp.reset();
      } else formOp.fail(err instanceof Error ? err.message : 'Ocurrió un error inesperado.');
    }
  }

  // ----- drag and drop -----
  let overbookAsk = $state<{ appt: Appt; date: string; start: string; message: string } | null>(null);
  const moveOp = new Op();
  async function moveTo(a: Appt, date: string, start: string, overbook = false) {
    const dur = toMin(a.endHour) - toMin(a.startHour);
    const e = toMin(start) + dur;
    const input = { ...inputFromAppt(a), date, startHour: start, endHour: `${String(Math.floor(e / 60)).padStart(2, '0')}:${String(e % 60).padStart(2, '0')}`, overbook };
    try {
      await agendaApi.update(a.id, input);
      overbookAsk = null;
      toast.show(`Cita movida al ${fmtShort(date)} a las ${start}`);
    } catch (err) {
      if (err instanceof ApiError && err.code === 'SLOT_TAKEN') {
        overbookAsk = { appt: a, date, start, message: err.message };
        return;
      }
      overbookAsk = null;
      toast.show(err instanceof Error ? err.message : 'No se pudo mover la cita.');
    }
    await load(true);
  }

  // ----- delete -----
  let deleting = $state<Appt | null>(null);
  const deleteOp = new Op();
  async function confirmDelete() {
    const target = deleting;
    if (!target) return;
    if (await deleteOp.run(() => agendaApi.remove(target.id))) {
      deleting = null;
      viewing = null;
      toast.show('Cita eliminada');
      await load(true);
    }
  }

  let blockOpen = $state(false);
  function onChanged(a: Appt) {
    appts = appts.map((x) => (x.id === a.id ? a : x));
    if (viewing?.id === a.id) viewing = a;
    load(true);
  }
  function goDay(d: string) {
    cursor = d;
    setView('day');
  }
  const today = todayStr();
  const listSorted = $derived([...appts].sort((a, b) => (a.date + a.startHour).localeCompare(b.date + b.startHour)));
</script>

<PageHeader title={admin ? 'Administrar citas' : 'Citas'} subtitle={admin ? 'Crea, reprograma y cancela citas. Arrastra una cita para moverla.' : 'Consulta la agenda del consultorio.'}>
  {#snippet actions()}
    {#if canEdit || ownProId}
      <button type="button" class="btn-secondary" onclick={() => (blockOpen = true)}><Icon name="ban" size={18} />{ownProId ? 'Marcar no disponible' : 'Bloquear horario'}</button>
    {/if}
    {#if canEdit}
      <button type="button" class="btn-primary" onclick={() => openCreate()}><Icon name="plus" size={18} stroke={2.2} />Nueva cita</button>
    {/if}
  {/snippet}
</PageHeader>

<div class="mb-3 flex flex-wrap items-center gap-2">
  <div class="flex items-center gap-1">
    <button type="button" class="btn-secondary !min-h-9 !px-3.5" onclick={() => (cursor = todayStr())}>Hoy</button>
    <button type="button" class="icon-btn" aria-label="Anterior" onclick={() => step(-1)}><Icon name="arrow-left" size={19} /></button>
    <button type="button" class="icon-btn" aria-label="Siguiente" onclick={() => step(1)}><Icon name="arrow-right" size={19} /></button>
  </div>
  <h2 class="min-w-0 flex-1 leading-snug text-base font-semibold sm:text-lg" aria-live="polite">{rangeTitle(view === 'list' ? 'month' : view, cursor)}</h2>
  <div class="inline-flex rounded-full bg-app-ink/6 p-0.5" role="group" aria-label="Vista de la agenda">
    {#each VIEWS as [k, label]}
      <button type="button" aria-pressed={view === k} class="rounded-full px-3.5 py-1.5 text-sm font-medium transition {view === k ? 'bg-app-ink text-app-surface' : 'text-app-muted hover:text-app-ink'}" onclick={() => setView(k)}>{label}</button>
    {/each}
  </div>
</div>

<div class="mb-4 flex flex-wrap items-center gap-2">
  <label class="flex items-center gap-2 text-sm">
    <span class="sr-only">Profesional</span>
    <select class="field !min-h-9 !w-auto !py-1" bind:value={proFilter} aria-label="Filtrar por profesional">
      <option value="">Todos los profesionales</option>
      {#each pros as p (p.id)}<option value={p.id}>{p.name}</option>{/each}
    </select>
  </label>
  {#if rooms.length}
    <select class="field !min-h-9 !w-auto !py-1" bind:value={roomFilter} aria-label="Filtrar por sala">
      <option value="">Todas las salas</option>
      {#each rooms as r}<option value={r}>{r}</option>{/each}
    </select>
  {/if}
  {#if view === 'list'}
    <select class="field !min-h-9 !w-auto !py-1" bind:value={statusFilter} aria-label="Filtrar por estado">
      <option value="">Todos los estados</option>
      {#each Object.entries(STATUS_META) as [k, m]}<option value={k}>{m.label}</option>{/each}
    </select>
  {/if}
  <ul class="ml-auto flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-app-muted" aria-label="Profesionales">
    {#each pros as p (p.id)}
      <li class="flex items-center gap-1.5"><span class="h-2.5 w-2.5 rounded-full" style="background:{proColor(pros, p.id)}"></span>{p.name}</li>
    {/each}
  </ul>
</div>

<div class="card overflow-hidden">
  {#if loading && appts.length === 0}
    <LoadingRows />
  {:else if loadError}
    <p class="alert m-5" role="alert"><Icon name="alert" size={18} />{loadError}</p>
  {:else if view === 'day' || view === 'week'}
    <TimeGrid
      {days} {appts} {blocks} {pros} {slot} range={gridRange} {isClosed} canEdit={canEdit}
      oncreate={(d, t) => openCreate(d, t)} onopen={(a) => (viewing = a)} onmove={(a, d, t) => moveTo(a, d, t)}
      onblock={() => (blockOpen = true)} onday={view === 'week' ? goDay : undefined}
    />
  {:else if view === 'month'}
    <MonthGrid {cursor} {appts} {blocks} {pros} load={monthLoad} onday={goDay} onopen={(a) => (viewing = a)} />
  {:else if listSorted.length === 0}
    <EmptyState icon="calendar" title="No hay citas en este mes" text={canEdit ? 'Agenda una cita o cambia de mes.' : 'Cuando se agenden citas aparecerán aquí.'}>
      {#if canEdit}<button type="button" class="btn-primary" onclick={() => openCreate()}><Icon name="plus" size={18} stroke={2.2} />Nueva cita</button>{/if}
    </EmptyState>
  {:else}
    <div class="overflow-x-auto">
      <table class="w-full min-w-[40rem]">
        <thead class="border-b border-app-ink/10 bg-app-elevated">
          <tr>
            <th class="th">Paciente</th>
            <th class="th">Fecha</th>
            <th class="th">Horario</th>
            <th class="th">Profesional</th>
            <th class="th">Estado</th>
            <th class="th"><span class="sr-only">Acciones</span></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-app-ink/8">
          {#each listSorted as a (a.id)}
            <tr class="transition hover:bg-app-ink/[0.03]">
              <td class="td">
                <span class="block font-semibold">{fullName(a)}</span>
                {#if a.service_name}<span class="block text-xs text-app-muted">{a.service_name}</span>{/if}
              </td>
              <td class="td whitespace-nowrap">
                {fmtShort(a.date)}
                {#if a.date === today}<span class="badge ml-2 normal-case">Hoy</span>{/if}
              </td>
              <td class="td whitespace-nowrap font-semibold tabular-nums">{a.startHour} – {a.endHour}</td>
              <td class="td whitespace-nowrap"><span class="mr-1.5 inline-block h-2.5 w-2.5 rounded-full" style="background:{proColor(pros, a.professional_id)}"></span>{a.professional_name || '—'}</td>
              <td class="td"><Pill tone={STATUS_META[a.status].tone}>{STATUS_META[a.status].label}</Pill></td>
              <td class="td">
                <div class="flex justify-end gap-1">
                  <button type="button" class="icon-btn" title="Ver detalles" aria-label="Ver detalles de la cita de {fullName(a)}" onclick={() => (viewing = a)}><Icon name="eye" size={19} /></button>
                  {#if canEdit && ['scheduled', 'confirmed', 'arrived'].includes(a.status)}
                    <button type="button" class="icon-btn" title="Editar" aria-label="Editar la cita de {fullName(a)}" onclick={() => openEdit(a)}><Icon name="edit" size={19} /></button>
                  {/if}
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<ul class="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-app-muted" aria-label="Colores por estado">
  {#each Object.entries(STATUS_META).filter(([k]) => k !== 'no_show') as [k, m]}
    <li class="flex items-center gap-1.5"><span class="h-2.5 w-2.5 rounded-sm border {m.card.split(' ').filter((c) => c.startsWith('bg-') || c.startsWith('border-')).join(' ')}"></span>{m.label}</li>
  {/each}
  <li class="flex items-center gap-1.5"><span class="h-2.5 w-2.5 rounded-sm bg-app-ink/20"></span>Bloqueado</li>
</ul>

<AppointmentDetail appt={viewing} {canEdit} onclose={() => (viewing = null)} onchanged={onChanged} onedit={openEdit} ondelete={(a) => { deleteOp.reset(); deleting = a; }} />

<Modal open={formOpen} title={editingId ? 'Editar cita' : 'Nueva cita'} onclose={() => (formOpen = false)}>
  <form id="appointment-form" onsubmit={submitForm}>
    <AppointmentForm bind:data={form} professionals={pros} {rooms} {services} {conflict} slotMinutes={slot} />
  </form>
  {#if formOp.phase === 'error'}
    <p class="alert mt-4" role="alert"><Icon name="alert" size={18} />{formOp.message}</p>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (formOpen = false)}>Cancelar</button>
    <button type="submit" form="appointment-form" class="btn-primary" disabled={formOp.phase === 'loading'}>
      {#if formOp.phase === 'loading'}<span class="spin"></span>{/if}{conflict?.code === 'SLOT_TAKEN' && form.overbook ? 'Guardar como sobreturno' : 'Guardar'}
    </button>
  {/snippet}
</Modal>

<ConfirmModal open={overbookAsk !== null} title="Ese horario está ocupado" confirmLabel="Agendar como sobreturno" op={moveOp}
  onconfirm={() => overbookAsk && moveTo(overbookAsk.appt, overbookAsk.date, overbookAsk.start, true)} onclose={() => { overbookAsk = null; load(true); }}>
  <p>{overbookAsk?.message} ¿Quieres moverla de todos modos como sobreturno?</p>
</ConfirmModal>

<ConfirmModal open={deleting !== null} title="¿Eliminar esta cita?" confirmLabel="Eliminar" op={deleteOp} onconfirm={confirmDelete} onclose={() => (deleting = null)}>
  <p>Se eliminará la cita de <strong class="text-app-ink">{deleting ? fullName(deleting) : ''}</strong> del {deleting ? fmtShort(deleting.date) : ''}. Si solo no vendrá, mejor cancélala para conservar el historial.</p>
</ConfirmModal>

<BlockModal open={blockOpen} {blocks} {pros} ownOnly={ownProId} date={cursor} onclose={() => (blockOpen = false)} onchanged={() => load(true)} />
