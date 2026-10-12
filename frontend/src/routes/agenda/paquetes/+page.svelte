<script lang="ts">
  import SectionTabs from '$lib/components/nav/SectionTabs.svelte';
  import { AGENDA_TABS } from '$lib/components/nav/tabs';
  import Alert from '$lib/components/ui/Alert.svelte';
  import OpError from '$lib/components/ui/OpError.svelte';
  import { onMount } from 'svelte';
  import { agendaLoadApi } from '$lib/api/waitlist';
  import { packagesApi, type SessionPackage } from '$lib/api/packages';
  import { moneyCents } from '$lib/format';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { ServiceDuration } from '$lib/types/waitlist';
  import Guard from '$lib/components/Guard.svelte';
  import Modal from '$lib/components/Modal.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import PageHeader from '$lib/components/ui/PageHeader.svelte';

  const uid = $props.id();
  let list = $state<SessionPackage[]>([]);
  let enabled = $state(true);
  let services = $state<ServiceDuration[]>([]);
  const load = new Op();
  const op = new Op();

  const canWrite = $derived(session.user?.role === 'admin' || session.user?.role === 'doctor');

  let open = $state(false);
  let editing = $state<SessionPackage | null>(null);
  let name = $state('');
  let price = $state('');
  let sessions = $state('5');
  let serviceId = $state('');
  let days = $state('');
  let isPublic = $state(false);
  let active = $state(true);

  onMount(() =>
    void load.run(async () => {
      const [r, s] = await Promise.all([packagesApi.list(), agendaLoadApi.services()]);
      list = r.packages;
      enabled = r.enabled;
      services = s;
    })
  );

  function openForm(p: SessionPackage | null) {
    editing = p;
    name = p?.name ?? '';
    price = p ? String(p.price_cents / 100) : '';
    sessions = String(p?.sessions ?? 5);
    serviceId = p?.service_id ?? '';
    days = p?.valid_days ? String(p.valid_days) : '';
    isPublic = p?.public ?? false;
    active = p?.active ?? true;
    op.reset();
    open = true;
  }

  async function save(e: SubmitEvent) {
    e.preventDefault();
    const cents = Math.round(Number(price.replace(/[$,\s]/g, '')) * 100);
    const n = Number(sessions);
    const d = days.trim() === '' ? null : Number(days);
    if (!name.trim()) return op.fail('Escribe el nombre del paquete.');
    if (!Number.isFinite(cents) || cents <= 0) return op.fail('Escribe el precio total del paquete.');
    if (!Number.isInteger(n) || n < 2 || n > 100) return op.fail('Un paquete tiene entre 2 y 100 sesiones.');
    if (d !== null && !(Number.isInteger(d) && d >= 1 && d <= 1825)) return op.fail('La vigencia debe ser un número de días entre 1 y 1825.');
    const input = { name: name.trim(), price_cents: cents, sessions: n, service_id: serviceId || null, valid_days: d, public: isPublic, active };
    let saved: SessionPackage | undefined;
    if (await op.run(async () => (saved = await packagesApi.save(editing?.id ?? null, input)))) {
      const s = saved!;
      list = editing ? list.map((x) => (x.id === s.id ? s : x)) : [s, ...list];
      open = false;
      toast.show(editing ? 'Paquete actualizado' : 'Paquete creado');
    }
  }

  const per = (p: SessionPackage) => moneyCents(Math.round(p.price_cents / p.sessions));
</script>

<Guard title="Paquetes de sesiones" permissions={['navAppointments', 'adminAppointments']} cobros>
  <SectionTabs tabs={AGENDA_TABS} label="Agenda" />
  <PageHeader title="Paquetes de sesiones" subtitle="Un precio por varias sesiones: el paciente lo paga en caja y cada cita descuenta una. Útil en psicología, nutrición, fisioterapia, quiropráctica y dermatología.">
    {#snippet actions()}
      {#if canWrite && enabled}<button type="button" class="btn-primary" onclick={() => openForm(null)}><Icon name="plus" size={18} />Nuevo paquete</button>{/if}
    {/snippet}
  </PageHeader>

  {#if load.phase === 'loading' && list.length === 0}
    <div class="card"><LoadingRows /></div>
  {:else if load.phase === 'error'}
    <Alert>{load.message}</Alert>
  {:else if !enabled}
    <div class="card"><EmptyState icon="info" title="Tu consultorio no usa paquetes" text="Los paquetes de sesiones están disponibles cuando atiendes psicología, nutrición, fisioterapia, quiropráctica o dermatología." /></div>
  {:else if list.length === 0}
    <div class="card"><EmptyState icon="plus" title="Aún no hay paquetes" text="Crea, por ejemplo, «5 sesiones de terapia» con un precio menor al de las sesiones sueltas." /></div>
  {:else}
    <ul class="grid gap-3 sm:grid-cols-2">
      {#each list as p (p.id)}
        <li class="card flex flex-col gap-2 px-5 py-4 {p.active ? '' : 'opacity-60'}">
          <div class="flex items-start justify-between gap-3">
            <p class="min-w-0 font-semibold [overflow-wrap:anywhere]">{p.name}</p>
            <p class="whitespace-nowrap font-mono text-lg font-semibold">{moneyCents(p.price_cents)}</p>
          </div>
          <p class="text-sm text-app-muted">{p.sessions} sesiones · {per(p)} cada una{p.service_name ? ` · ${p.service_name}` : ' · cualquier servicio'}</p>
          <p class="text-xs text-app-muted">{p.valid_days ? `Vigencia de ${p.valid_days} días desde la compra` : 'No vence'}{p.public ? ' · se muestra en tu página pública' : ''}{p.active ? '' : ' · inactivo'}</p>
          {#if canWrite}<button type="button" class="btn-secondary mt-1 self-start" onclick={() => openForm(p)}>Editar</button>{/if}
        </li>
      {/each}
    </ul>
  {/if}
</Guard>

<Modal open={open} title={editing ? 'Editar paquete' : 'Nuevo paquete'} onclose={() => (open = false)}>
  <form class="grid gap-4" onsubmit={save}>
    <OpError {op} />
    <label class="block"><span class="label">Nombre</span><input class="field" bind:value={name} maxlength="120" placeholder="Por ejemplo: 5 sesiones de terapia" required /></label>
    <div class="grid gap-4 sm:grid-cols-2">
      <label class="block"><span class="label">Precio total (MXN)</span><input class="field" inputmode="decimal" bind:value={price} placeholder="3500" required /></label>
      <label class="block"><span class="label">Número de sesiones</span><input class="field" inputmode="numeric" bind:value={sessions} maxlength="3" required /></label>
    </div>
    <label class="block">
      <span class="label">Para el servicio</span>
      <select class="field" bind:value={serviceId}>
        <option value="">Cualquier servicio</option>
        {#each services as s (s.id)}<option value={s.id}>{s.name}</option>{/each}
      </select>
      <span class="hint">Si eliges uno, solo las citas de ese servicio pueden usar sus sesiones.</span>
    </label>
    <label class="block"><span class="label">Vigencia (días, opcional)</span><input class="field" inputmode="numeric" bind:value={days} maxlength="4" placeholder="90" /><span class="hint">Vacío: no vence.</span></label>
    <label class="flex cursor-pointer items-start gap-3 text-sm"><input type="checkbox" class="mt-0.5 h-4 w-4" bind:checked={isPublic} /><span>Mostrar en mi página pública con su precio</span></label>
    {#if editing}<label class="flex cursor-pointer items-start gap-3 text-sm"><input type="checkbox" class="mt-0.5 h-4 w-4" bind:checked={active} /><span>Disponible para vender. Quitarlo no afecta a los paquetes ya vendidos.</span></label>{/if}
    <div class="flex justify-end gap-2">
      <button type="button" class="btn-secondary" onclick={() => (open = false)}>Cancelar</button>
      <button type="submit" class="btn-primary" disabled={op.phase === 'loading'}>Guardar</button>
    </div>
  </form>
</Modal>
