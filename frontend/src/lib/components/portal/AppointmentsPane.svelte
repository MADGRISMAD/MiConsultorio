<script lang="ts">
  import Alert from '$lib/components/ui/Alert.svelte';
  import AddToCalendar from '$lib/components/ui/AddToCalendar.svelte';
  import { onMount } from 'svelte';
  import { portalApi } from '$lib/api/portal';
  import { Op } from '$lib/op.svelte';
  import type { PortalAppointment } from '$lib/types/portal';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import { fmtDay, t } from '$lib/i18n/index.svelte';

  let { slug, patientId, multi, clinic }: { slug: string; patientId: string; multi: boolean; clinic: { name: string; address: string } } = $props();

  let upcoming = $state<PortalAppointment[]>([]);
  let history = $state<PortalAppointment[]>([]);
  let minHours = $state(0);
  let loaded = $state(false);
  const loadOp = new Op();
  const cancelOp = new Op();
  let target = $state<PortalAppointment | null>(null);
  let reason = $state('');
  let notice = $state('');

  async function load() {
    await loadOp.run(async () => {
      const r = await portalApi.appointments();
      upcoming = r.upcoming;
      history = r.history;
      minHours = r.cancel_min_hours;
      loaded = true;
    });
  }
  onMount(load);

  const mine = (list: PortalAppointment[]) => list.filter((a) => !patientId || a.patient_id === patientId);
  const up = $derived(mine(upcoming));
  const past = $derived(mine(history));

  const STATUS: Record<string, 'info' | 'ok' | 'warn' | 'bad' | 'muted'> = {
    scheduled: 'info',
    confirmed: 'ok',
    arrived: 'ok',
    in_progress: 'ok',
    completed: 'muted',
    no_show: 'warn',
    cancelled: 'bad'
  };
  const tone = (s: string) => `pill pill-${STATUS[s] ?? 'info'}`.replace('pill-muted', '');
  const statusLabel = (s: string) => (s in STATUS ? t(`portal.appts.st.${s}`) : s);


  async function confirmCancel() {
    if (!target) return;
    const id = target.id;
    if (
      await cancelOp.run(async () => {
        await portalApi.cancel(id, reason.trim());
        await load();
      })
    ) {
      notice = t('portal.appts.cancelled');
      target = null;
      reason = '';
    }
  }
</script>

{#snippet card(a: PortalAppointment, withCancel: boolean)}
  <li class="card flex flex-wrap items-start justify-between gap-3 px-4 py-4 sm:px-5">
    <div class="min-w-0">
      <p class="font-medium first-letter:uppercase">{fmtDay(a.date)}</p>
      <p class="mt-0.5 flex items-center gap-1.5 text-sm text-app-muted"><Icon name="clock" size={15} />{t('portal.appts.hours', { start: a.start_hour, end: a.end_hour })}</p>
      {#if multi && a.patient_name}<p class="mt-1 text-sm">{t('portal.appts.patient')} <strong>{a.patient_name}</strong></p>{/if}
      {#if a.service}<p class="text-sm text-app-muted">{a.service}</p>{/if}
      {#if a.professional}<p class="text-sm text-app-muted">{t('portal.appts.with', { name: a.professional })}</p>{/if}
    </div>
    <div class="flex flex-col items-end gap-2">
      <span class={tone(a.status)}>{statusLabel(a.status)}</span>
      {#if withCancel}
        <AddToCalendar size="small" event={{ title: `Cita en ${clinic.name}`, date: a.date, start: a.start_hour, end: a.end_hour, location: clinic.address, details: [a.professional && `Atiende: ${a.professional}`, a.service && `Servicio: ${a.service}`].filter(Boolean).join('\n'), uid: a.id }} />
        {#if a.can_cancel}
          <button class="btn-secondary !min-h-9" onclick={() => { target = a; reason = ''; cancelOp.reset(); }}>{t('portal.appts.cancel')}</button>
        {:else}
          <span class="max-w-[14rem] text-right text-xs text-app-muted">{t('portal.appts.tooLate', { hours: minHours })}</span>
        {/if}
      {/if}
    </div>
  </li>
{/snippet}

{#if !loaded && loadOp.phase !== 'error'}
  <LoadingRows />
{:else if loadOp.phase === 'error' && !loaded}
  <Alert>{loadOp.message}</Alert>
{:else}
  <div class="grid gap-8">
    {#if notice}<p class="rounded-xl bg-app-accent/12 px-3.5 py-3 text-sm font-medium text-app-accent" role="status">{notice}</p>{/if}
    <section aria-labelledby="ap-next">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
        <h2 id="ap-next" class="section-title">{t('portal.appts.next')}</h2>
        <a class="btn-primary !min-h-9" href="/reservar/{encodeURIComponent(slug)}"><Icon name="plus" size={16} />{t('portal.appts.book')}</a>
      </div>
      {#if up.length === 0}
        <div class="card-empty">{t('portal.appts.none')}</div>
      {:else}
        <ul class="grid gap-3">{#each up as a (a.id)}{@render card(a, true)}{/each}</ul>
      {/if}
    </section>

    <section aria-labelledby="ap-hist">
      <h2 id="ap-hist" class="section-title mb-3">{t('portal.appts.history')}</h2>
      {#if past.length === 0}
        <div class="card-empty">{t('portal.appts.noneHistory')}</div>
      {:else}
        <ul class="grid gap-3">{#each past as a (a.id)}{@render card(a, false)}{/each}</ul>
      {/if}
    </section>
  </div>
{/if}

<ConfirmModal open={target !== null} title={t('portal.appts.cancel')} op={cancelOp} onconfirm={confirmCancel} onclose={() => (target = null)} confirmLabel={t('portal.appts.confirmCancel')}>
  {#if target}
    <p>{t('portal.appts.confirmBefore')}<strong>{fmtDay(target.date)}</strong>{t('portal.appts.confirmAfter', { hour: t('common.hourSuffix', { time: target.start_hour }) })}</p>
    <label class="label mt-4" for="ap-reason">{t('portal.appts.reason')}</label>
    <input id="ap-reason" class="field" maxlength="300" bind:value={reason} />
  {/if}
</ConfirmModal>
