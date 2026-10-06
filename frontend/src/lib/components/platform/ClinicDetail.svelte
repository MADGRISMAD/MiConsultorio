<script lang="ts">
  import { api } from '$lib/api';
  import { ago, dateShort, moneyCents } from '$lib/format';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import { CLINIC_KINDS, ROLES, type ActivityItem, type ClinicKind, type ClinicRow, type Payment, type Person, type Plan, type Seats } from '$lib/types';
  import ConfirmModal from '../ConfirmModal.svelte';
  import Modal from '../Modal.svelte';
  import Avatar from '../ui/Avatar.svelte';
  import Icon from '../ui/Icon.svelte';
  import LoadingRows from '../ui/LoadingRows.svelte';
  import Pill from '../ui/Pill.svelte';
  import RolePill from '../ui/RolePill.svelte';
  import StatePill from '../ui/StatePill.svelte';

  interface Props {
    id: string;
    canEdit: boolean;
    plans: Plan[];
    /** the list on the left should refresh after a change */
    onchanged: (c: ClinicRow) => void;
  }
  let { id, canEdit, plans, onchanged }: Props = $props();

  type Tab = 'resumen' | 'licencia' | 'personas' | 'pagos' | 'actividad';
  const tabs: [Tab, string][] = [
    ['resumen', 'Resumen'],
    ['licencia', 'Licencia'],
    ['personas', 'Personas'],
    ['pagos', 'Pagos'],
    ['actividad', 'Actividad']
  ];
  let tab = $state<Tab>('resumen');

  let clinic = $state<ClinicRow | null>(null);
  let people = $state<Person[]>([]);
  let seats = $state<Seats | null>(null);
  let activity = $state<ActivityItem[]>([]);
  let payments = $state<Payment[]>([]);
  let error = $state('');
  let loadedId = $state('');

  async function load() {
    const target = id;
    try {
      const d = await api.platform.clinic(target);
      if (target !== id) return;
      ({ clinic, people, seats, activity, payments } = d);
      fill();
      error = '';
      loadedId = target;
    } catch (e) {
      error = e instanceof Error ? e.message : 'No se pudo cargar el negocio.';
    }
  }
  $effect(() => {
    id;
    clinic = null;
    tab = 'resumen';
    void load();
  });

  // ----- license form -----
  let draft = $state({ name: '', kind: 'GENERAL_MEDICAL' as ClinicKind, phone_number: '', address: '', plan: 'basico', billing_status: 'active', trial_ends_on: '' });
  function fill() {
    if (!clinic) return;
    draft = {
      name: clinic.name,
      kind: clinic.kind,
      phone_number: clinic.phone_number,
      address: clinic.address,
      plan: clinic.plan,
      billing_status: clinic.billing_status,
      trial_ends_on: clinic.trial_ends_at ? clinic.trial_ends_at.slice(0, 10) : ''
    };
  }
  const dirty = $derived(
    !!clinic &&
      (draft.name !== clinic.name ||
        draft.kind !== clinic.kind ||
        draft.phone_number !== clinic.phone_number ||
        draft.address !== clinic.address ||
        draft.plan !== clinic.plan ||
        draft.billing_status !== clinic.billing_status ||
        draft.trial_ends_on !== (clinic.trial_ends_at ? clinic.trial_ends_at.slice(0, 10) : ''))
  );
  const saveOp = new Op();
  async function save(e: SubmitEvent) {
    e.preventDefault();
    const body: Record<string, unknown> = { ...draft };
    if (draft.trial_ends_on === '') delete body.trial_ends_on;
    if (!(await saveOp.run(async () => (clinic = await api.platform.updateClinic(id, body))))) return;
    toast.show('Cambios guardados');
    onchanged(clinic!);
    await load();
  }

  // ----- suspend / reactivate -----
  let suspendOpen = $state(false);
  let reason = $state('');
  const suspendOp = new Op();
  async function suspend() {
    if (!(await suspendOp.run(async () => (clinic = await api.platform.suspend(id, reason))))) return;
    suspendOpen = false;
    toast.show('Negocio suspendido');
    onchanged(clinic!);
    await load();
  }
  let reactivateOpen = $state(false);
  const reactivateOp = new Op();
  async function reactivate() {
    if (!(await reactivateOp.run(async () => (clinic = await api.platform.reactivate(id))))) return;
    reactivateOpen = false;
    toast.show('Negocio reactivado');
    onchanged(clinic!);
    await load();
  }

  // ----- manual payment -----
  let pay = $state({ amount: 0, months: 1, note: '' });
  const payOp = new Op();
  $effect(() => {
    const p = plans.find((x) => x.id === clinic?.plan);
    if (p && pay.amount === 0) pay.amount = p.price_month;
  });
  async function recordPayment(e: SubmitEvent) {
    e.preventDefault();
    if (!(await payOp.run(() => api.platform.pay(id, { amount: Number(pay.amount), months: Number(pay.months), note: pay.note })))) return;
    toast.show('Pago registrado');
    pay = { amount: plans.find((x) => x.id === clinic?.plan)?.price_month ?? 0, months: 1, note: '' };
    await load();
    if (clinic) onchanged(clinic);
  }

  const kindLabel = $derived(clinic ? CLINIC_KINDS[clinic.kind]?.label : '');
  const seatLine = (max: number | null, used: number) => (max === null ? `${used} (sin límite)` : `${used} de ${max}`);
</script>

{#if error}
  <p class="alert" role="alert"><Icon name="alert" size={18} />{error}</p>
{:else if !clinic || loadedId !== id}
  <div class="card"><LoadingRows /></div>
{:else}
  <section class="card overflow-hidden">
    <header class="flex flex-wrap items-center gap-4 border-b border-app-ink/10 p-5 sm:p-6">
      <Avatar name={clinic.name} size={52} />
      <div class="min-w-0 flex-1 basis-56">
        <h2 class="display truncate text-3xl leading-tight">{clinic.name}</h2>
        <p class="mt-1 flex flex-wrap items-center gap-2 text-sm text-app-muted">
          <StatePill state={clinic.state} /><Pill tone="info">{clinic.plan_name}</Pill><span>{kindLabel}</span>
        </p>
      </div>
      {#if canEdit}
        {#if clinic.state === 'suspended'}
          <button type="button" class="btn-primary" onclick={() => { reactivateOp.reset(); reactivateOpen = true; }}><Icon name="refresh" size={17} />Reactivar</button>
        {:else}
          <button type="button" class="btn-secondary" onclick={() => { suspendOp.reset(); reason = ''; suspendOpen = true; }}><Icon name="ban" size={17} />Suspender</button>
        {/if}
      {/if}
    </header>

    <div class="flex gap-1 overflow-x-auto border-b border-app-ink/10 px-3 py-2" role="tablist" aria-label="Secciones del negocio">
      {#each tabs as [key, label]}
        <button type="button" role="tab" aria-selected={tab === key} class="whitespace-nowrap rounded-full px-3.5 py-1.5 text-sm font-medium transition {tab === key ? 'bg-app-ink text-app-surface' : 'text-app-muted hover:bg-app-ink/5 hover:text-app-ink'}" onclick={() => (tab = key)}>{label}</button>
      {/each}
    </div>

    <div class="p-5 sm:p-6">
      {#if tab === 'resumen'}
        {#if clinic.state === 'suspended' && clinic.suspended_reason}
          <p class="alert mb-5"><Icon name="ban" size={18} />Suspendido: {clinic.suspended_reason}</p>
        {/if}
        <dl class="grid gap-x-8 gap-y-5 text-sm sm:grid-cols-2">
          <div><dt class="section-title">Dueño</dt><dd class="mt-1 font-semibold">{clinic.owner_name || '—'}</dd></div>
          <div><dt class="section-title">Correo</dt><dd class="mt-1 break-all">{#if clinic.owner_email}<a class="text-app-primary hover:underline" href="mailto:{clinic.owner_email}">{clinic.owner_email}</a>{:else}—{/if}</dd></div>
          <div><dt class="section-title">Teléfono</dt><dd class="mt-1">{clinic.phone_number || '—'}</dd></div>
          <div><dt class="section-title">Dirección</dt><dd class="mt-1">{clinic.address || '—'}</dd></div>
          <div><dt class="section-title">Alta</dt><dd class="mt-1">{dateShort(clinic.created_at)}</dd></div>
          <div><dt class="section-title">Último acceso</dt><dd class="mt-1">{clinic.last_seen ? ago(clinic.last_seen) : 'nunca ha entrado'}</dd></div>
          <div>
            <dt class="section-title">{clinic.state === 'trialing' || clinic.state === 'trial_expired' ? 'Fin de la prueba' : 'Periodo pagado hasta'}</dt>
            <dd class="mt-1">
              {#if clinic.state === 'trialing' || clinic.state === 'trial_expired'}
                {dateShort(clinic.trial_ends_at)}{#if clinic.trial_days_left !== null} · {clinic.trial_days_left} día(s){/if}
              {:else}{dateShort(clinic.current_period_end)}{/if}
            </dd>
          </div>
          {#if seats}
            <div>
              <dt class="section-title">Cuentas del plan</dt>
              <dd class="mt-1">{seatLine(seats.max_users, seats.used_users)} · médicos {seatLine(seats.max_doctors, seats.used_doctors)}</dd>
            </div>
          {/if}
        </dl>
      {:else if tab === 'licencia'}
        {#if !canEdit}
          <p class="mb-5 flex items-start gap-2 rounded-xl bg-app-warning/12 px-4 py-3 text-sm text-app-warning"><Icon name="info" size={17} class="mt-0.5 flex-none" />Estás en modo soporte: puedes ver los datos, pero solo un administrador cambia el plan, el estado o los datos del negocio.</p>
        {/if}
        <form class="grid gap-4 sm:grid-cols-2" onsubmit={save}>
          <fieldset class="contents" disabled={!canEdit}>
            <div class="sm:col-span-2">
              <label class="label" for="cl-name">Nombre del negocio</label>
              <input id="cl-name" class="field" bind:value={draft.name} required minlength="2" />
            </div>
            <div>
              <label class="label" for="cl-kind">Giro</label>
              <select id="cl-kind" class="field" bind:value={draft.kind}>
                {#each Object.entries(CLINIC_KINDS) as [k, info]}<option value={k}>{info.label}</option>{/each}
              </select>
            </div>
            <div>
              <label class="label" for="cl-phone">Teléfono</label>
              <input id="cl-phone" class="field" type="tel" bind:value={draft.phone_number} />
            </div>
            <div class="sm:col-span-2">
              <label class="label" for="cl-addr">Dirección</label>
              <input id="cl-addr" class="field" bind:value={draft.address} />
            </div>

            <div class="sm:col-span-2">
              <p class="label">Plan</p>
              <div class="grid gap-2 sm:grid-cols-3" role="radiogroup" aria-label="Plan del negocio">
                {#each plans as p}
                  <button type="button" role="radio" aria-checked={draft.plan === p.id} class="rounded-xl border-2 p-3 text-left transition {draft.plan === p.id ? 'border-app-primary bg-app-primary/8' : 'border-app-ink/10 hover:border-app-ink/25'}" onclick={() => (draft.plan = p.id)}>
                    <strong class="block text-sm {draft.plan === p.id ? 'text-app-primary' : ''}">{p.name}</strong>
                    <small class="block text-xs text-app-muted">{p.price_month ? `$${p.price_month.toLocaleString('es-MX')} / mes` : 'A medida'} · {p.description}</small>
                  </button>
                {/each}
              </div>
            </div>
            <div>
              <label class="label" for="cl-status">Estado</label>
              <select id="cl-status" class="field" bind:value={draft.billing_status}>
                <option value="trialing">Prueba</option>
                <option value="active">Activo</option>
                <option value="past_due">Pago atrasado</option>
                <option value="suspended">Suspendido</option>
              </select>
            </div>
            <div>
              <label class="label" for="cl-trial">Fin de la prueba</label>
              <input id="cl-trial" class="field" type="date" bind:value={draft.trial_ends_on} />
            </div>
          </fieldset>
          {#if saveOp.phase === 'error'}<p class="alert sm:col-span-2" role="alert"><Icon name="alert" size={18} />{saveOp.message}</p>{/if}
          {#if canEdit}
            <div class="flex gap-2 sm:col-span-2">
              <button type="submit" class="btn-primary" disabled={!dirty || saveOp.phase === 'loading'}>{#if saveOp.phase === 'loading'}<span class="spin"></span>{/if}{dirty ? 'Guardar cambios' : 'Sin cambios'}</button>
              {#if dirty}<button type="button" class="btn-ghost" onclick={fill}>Descartar</button>{/if}
            </div>
          {/if}
        </form>

        {#if canEdit}
          <form class="mt-8 grid gap-3 border-t border-app-ink/10 pt-6 sm:grid-cols-[1fr_8rem_2fr_auto] sm:items-end" onsubmit={recordPayment}>
            <h3 class="display text-2xl sm:col-span-4">Registrar un pago</h3>
            <p class="-mt-1 text-sm text-app-muted sm:col-span-4">Para pagos recibidos fuera de la app (transferencia, efectivo). Activa el negocio y extiende su periodo pagado.</p>
            <div>
              <label class="label" for="pay-amount">Monto (MXN)</label>
              <input id="pay-amount" class="field" type="number" min="0" step="0.01" bind:value={pay.amount} required />
            </div>
            <div>
              <label class="label" for="pay-months">Meses</label>
              <input id="pay-months" class="field" type="number" min="1" max="36" bind:value={pay.months} required />
            </div>
            <div>
              <label class="label" for="pay-note">Nota</label>
              <input id="pay-note" class="field" bind:value={pay.note} maxlength="200" placeholder="SPEI, folio…" />
            </div>
            <button type="submit" class="btn-primary" disabled={payOp.phase === 'loading'}>Registrar</button>
            {#if payOp.phase === 'error'}<p class="alert sm:col-span-4" role="alert"><Icon name="alert" size={18} />{payOp.message}</p>{/if}
          </form>
        {/if}
      {:else if tab === 'personas'}
        {#if people.length === 0}
          <p class="text-sm text-app-muted">Este negocio no tiene cuentas.</p>
        {:else}
          <ul class="divide-y divide-app-ink/8">
            {#each people as p (p.id)}
              <li class="flex flex-wrap items-center gap-3 py-3 {p.disabled ? 'opacity-70' : ''}">
                <Avatar name={p.name} size={40} />
                <div class="min-w-0 flex-1 basis-48">
                  <p class="truncate font-semibold">{p.name}</p>
                  <p class="truncate text-sm text-app-muted">{p.email || `@${p.username}`} · {p.last_login_at ? ago(p.last_login_at) : 'nunca ha entrado'}</p>
                </div>
                <RolePill role={p.role} long />
                {#if p.disabled}<Pill>Desactivada</Pill>{/if}
                {#if p.email}<a class="icon-btn" href="mailto:{p.email}" aria-label="Escribir a {p.name}" title="Escribir"><Icon name="mail" size={18} /></a>{/if}
              </li>
            {/each}
          </ul>
          <p class="mt-4 text-xs text-app-muted">{ROLES.admin.label}es administran el equipo del negocio; los cambios de cuentas los hace el propio negocio.</p>
        {/if}
      {:else if tab === 'pagos'}
        {#if payments.length === 0}
          <p class="text-sm text-app-muted">Aún no hay pagos registrados.</p>
        {:else}
          <ul class="divide-y divide-app-ink/8">
            {#each payments as p (p.id)}
              <li class="flex flex-wrap items-center gap-x-4 gap-y-1 py-3">
                <strong class="display text-2xl tabular-nums">{moneyCents(p.amount_cents)}</strong>
                <span class="min-w-0 flex-1 basis-40 text-sm text-app-muted">{p.months} mes(es) · hasta {dateShort(p.period_end)}{p.note ? ` · ${p.note}` : ''}</span>
                <span class="text-sm text-app-muted">{p.created_by ? `${p.created_by} · ` : ''}{ago(p.created_at)}</span>
              </li>
            {/each}
          </ul>
        {/if}
      {:else}
        {#if activity.length === 0}
          <p class="text-sm text-app-muted">Aún no hay actividad.</p>
        {:else}
          <ul class="relative space-y-4 border-l border-app-ink/12 pl-5">
            {#each activity as a (a.id)}
              <li class="relative">
                <span class="absolute -left-[1.62rem] top-1.5 h-2.5 w-2.5 rounded-full bg-app-primary ring-4 ring-app-panel"></span>
                <p class="text-sm">{a.message}</p>
                <p class="text-xs text-app-muted">{a.actor_name || 'Sistema'} · {ago(a.created_at)}</p>
              </li>
            {/each}
          </ul>
        {/if}
      {/if}
    </div>
  </section>

  <Modal open={suspendOpen} title="Suspender negocio" onclose={() => (suspendOpen = false)}>
    <p class="text-sm text-app-muted">Mientras esté suspendido, nadie de <strong class="text-app-ink">{clinic.name}</strong> podrá usar la app. Sus datos no se borran.</p>
    <label class="label mt-4" for="susp-reason">Motivo <span class="font-normal text-app-muted">(lo verán ellos)</span></label>
    <input id="susp-reason" class="field" bind:value={reason} maxlength="200" placeholder="Pago pendiente" />
    {#if suspendOp.phase === 'error'}<p class="alert mt-4" role="alert"><Icon name="alert" size={18} />{suspendOp.message}</p>{/if}
    {#snippet footer()}
      <button type="button" class="btn-secondary" onclick={() => (suspendOpen = false)}>Cancelar</button>
      <button type="button" class="btn-danger" disabled={suspendOp.phase === 'loading'} onclick={suspend}>Suspender</button>
    {/snippet}
  </Modal>

  <ConfirmModal open={reactivateOpen} title="¿Reactivar este negocio?" confirmLabel="Reactivar" op={reactivateOp} onconfirm={reactivate} onclose={() => (reactivateOpen = false)}>
    <p>Volverá a tener acceso como {clinic.trial_ends_at && !clinic.current_period_end && new Date(clinic.trial_ends_at) > new Date() ? 'prueba' : 'plan activo'}.</p>
  </ConfirmModal>
{/if}
