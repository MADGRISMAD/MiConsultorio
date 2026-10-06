<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { ago, dateShort, money, moneyCents } from '$lib/format';
  import { session } from '$lib/session.svelte';
  import { CLINIC_KINDS, type Overview } from '$lib/types';
  import Avatar from '../ui/Avatar.svelte';
  import Icon from '../ui/Icon.svelte';
  import LoadingRows from '../ui/LoadingRows.svelte';
  import PageHeader from '../ui/PageHeader.svelte';
  import StatePill from '../ui/StatePill.svelte';

  let data = $state<Overview | null>(null);
  let error = $state('');

  onMount(async () => {
    try {
      data = await api.platform.overview();
    } catch (e) {
      error = e instanceof Error ? e.message : 'No se pudo cargar el resumen.';
    }
  });

  const today = new Date().toLocaleDateString('es-MX', { weekday: 'long', day: 'numeric', month: 'long' });
  const hour = new Date().getHours();
  const greeting = hour < 12 ? 'Buenos días' : hour < 19 ? 'Buenas tardes' : 'Buenas noches';
  const tone = { past_due: 'bg-app-warning/14 text-app-warning', trial_expired: 'bg-app-warning/14 text-app-warning', trial_ending: 'bg-app-primary/12 text-app-primary' };
</script>

<PageHeader title="Resumen" subtitle="{greeting}, {session.user?.name?.split(' ')[0]}. {today}." />

{#if error}
  <p class="alert" role="alert"><Icon name="alert" size={18} />{error}</p>
{:else if !data}
  <div class="card"><LoadingRows /></div>
{:else}
  {@const c = data.counts}
  <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
    {#each [
      { label: 'Negocios activos', value: c.active ?? 0, hint: `${c.total} en total`, state: 'active' },
      { label: 'En prueba', value: c.trialing ?? 0, hint: `${data.signups_30d} ${data.signups_30d === 1 ? 'alta' : 'altas'} en 30 días`, state: 'trialing' },
      { label: 'Requieren pago', value: (c.past_due ?? 0) + (c.trial_expired ?? 0), hint: `${c.trial_expired ?? 0} con prueba vencida`, state: 'past_due' },
      { label: 'Suspendidos', value: c.suspended ?? 0, hint: 'sin acceso', state: 'suspended' }
    ] as k}
      <a href="/plataforma/negocios?state={k.state}" class="card p-5 transition hover:-translate-y-0.5 hover:ring-1 hover:ring-app-primary/30">
        <p class="section-title">{k.label}</p>
        <p class="display mt-3 text-6xl leading-none tabular-nums">{k.value}</p>
        <p class="mt-2 text-sm text-app-muted">{k.hint}</p>
      </a>
    {/each}
  </div>

  {#if data.mrr !== undefined}
    <div class="mt-4 grid gap-4 sm:grid-cols-2">
      <div class="card p-5">
        <p class="section-title">Ingreso mensual estimado</p>
        <p class="display mt-3 text-5xl leading-none tabular-nums">{money(data.mrr)}</p>
        <p class="mt-2 text-sm text-app-muted">Suma de los planes activos de pago, al precio de lista.</p>
      </div>
      <div class="card p-5">
        <p class="section-title">Cobrado este mes</p>
        <p class="display mt-3 text-5xl leading-none tabular-nums">{moneyCents(data.paid_this_month_cents ?? 0)}</p>
        <p class="mt-2 text-sm text-app-muted">Pagos registrados a mano en la plataforma.</p>
      </div>
    </div>
  {/if}

  <div class="mt-4 grid gap-4 lg:grid-cols-[1.3fr_1fr]">
    <section class="card p-5 sm:p-6" aria-labelledby="att">
      <h2 id="att" class="display text-3xl">Necesita tu atención</h2>
      <p class="mt-1 text-sm text-app-muted">Lo más urgente primero.</p>
      {#if data.attention.length === 0}
        <p class="mt-5 flex items-center gap-2 rounded-xl bg-app-accent/10 px-4 py-6 text-sm text-app-accent"><Icon name="check" size={18} stroke={2.4} />Todo al corriente. Nada pendiente.</p>
      {:else}
        <ul class="mt-4 divide-y divide-app-ink/8">
          {#each data.attention as a}
            <li class="flex flex-wrap items-center gap-3 py-3">
              <Avatar name={a.clinic_name} size={38} />
              <div class="min-w-0 flex-1 basis-40">
                <p class="truncate font-semibold">{a.clinic_name}</p>
                <p class="truncate text-sm text-app-muted">{a.detail}{a.since ? ` · ${ago(a.since)}` : ''}</p>
              </div>
              <span class="pill {tone[a.kind]}">{a.title}</span>
              <a class="btn-secondary !min-h-9" href="/plataforma/negocios/{a.clinic_id}">Abrir</a>
            </li>
          {/each}
        </ul>
      {/if}
    </section>

    <section class="card p-5 sm:p-6" aria-labelledby="rec">
      <h2 id="rec" class="display text-3xl">Altas recientes</h2>
      {#if data.recent.length === 0}
        <p class="mt-5 rounded-xl bg-app-ink/5 px-4 py-6 text-center text-sm text-app-muted">Aún no hay negocios.</p>
      {:else}
        <ul class="mt-4 divide-y divide-app-ink/8">
          {#each data.recent as r}
            <li>
              <a href="/plataforma/negocios/{r.id}" class="flex items-center gap-3 rounded-lg py-3 transition hover:bg-app-ink/[0.03]">
                <Avatar name={r.name} size={38} />
                <span class="min-w-0 flex-1">
                  <span class="block truncate font-semibold">{r.name}</span>
                  <span class="block truncate text-sm text-app-muted">{CLINIC_KINDS[r.kind]?.label} · {dateShort(r.created_at)}</span>
                </span>
                <StatePill state={r.state} />
              </a>
            </li>
          {/each}
        </ul>
      {/if}
    </section>
  </div>
{/if}
