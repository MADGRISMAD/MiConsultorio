<script lang="ts">
  import { untrack } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { PosSettings, ProviderStatus } from '$lib/types';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import PageHeader from '$lib/components/ui/PageHeader.svelte';
  import BusinessSection from './BusinessSection.svelte';
  import MercadoPagoSection from './MercadoPagoSection.svelte';
  import MethodsSection from './MethodsSection.svelte';
  import PrinterSection from './PrinterSection.svelte';
  import RulesSection from './RulesSection.svelte';
  import TicketSection from './TicketSection.svelte';

  let s = $state<PosSettings | null>(null);
  let providers = $state<ProviderStatus | null>(null);
  let saved = $state('');
  let error = $state('');
  const saveOp = new Op();

  const plain = (v: PosSettings) => JSON.stringify($state.snapshot(v));
  const dirty = $derived(!!s && plain(s) !== saved);

  async function load() {
    error = '';
    try {
      const r = await api.pos.settings();
      s = r.settings;
      providers = r.providers;
      saved = plain(r.settings);
    } catch (e) {
      error = e instanceof Error ? e.message : 'No se pudieron cargar los ajustes.';
    }
  }
  async function refreshProviders() {
    try {
      providers = (await api.pos.settings()).providers;
    } catch {
      /* keep the last known status */
    }
  }
  void load();

  // Back from Mercado Pago's authorization screen
  $effect(() => {
    const mp = page.url.searchParams.get('mp');
    if (!mp) return;
    const reason = page.url.searchParams.get('reason') ?? '';
    const detail = page.url.searchParams.get('detail') ?? '';
    // Once per visit: the toast store and the reload below must not re-trigger this effect.
    untrack(() => {
      if (mp === 'ok') toast.show('Cuenta de Mercado Pago conectada');
      else {
        const why: Record<string, string> = {
          cancelled: 'Cancelaste la autorización en Mercado Pago.',
          state: 'El enlace de conexión caducó. Inténtalo de nuevo.',
          oauth_failed: `Mercado Pago no aceptó la conexión${detail ? `: ${detail}` : ''}. Revisa que la URL de redireccionamiento de tu aplicación sea exactamente la que usa este servidor.`
        };
        toast.show(why[reason] ?? 'No se pudo conectar Mercado Pago. Inténtalo de nuevo.', 'error', 9000);
      }
      void goto('/pos/ajustes', { replaceState: true, noScroll: true, keepFocus: true });
      void refreshProviders();
    });
  });

  function validate(v: PosSettings): string {
    if (!v.methods.length) return 'Activa al menos un método de pago.';
    if (v.zip_code && !/^\d{5}$/.test(v.zip_code.trim())) return 'El código postal debe tener 5 dígitos.';
    return '';
  }

  async function save() {
    if (!s) return;
    const bad = validate(s);
    if (bad) return saveOp.fail(bad);
    const body = { ...($state.snapshot(s) as PosSettings), rfc: s.rfc.trim().toUpperCase() };
    let out: PosSettings | null = null;
    if (await saveOp.run(async () => (out = await api.pos.saveSettings(body)))) {
      s = out;
      saved = plain(out!);
      toast.show('Ajustes guardados');
    }
  }
  function discard() {
    if (saved) s = JSON.parse(saved);
    saveOp.reset();
  }
</script>

<PageHeader title="Ajustes de cobros" subtitle="Datos fiscales, ticket, reglas de venta, métodos de pago e impresora." />

{#if error}
  <p class="alert" role="alert"><Icon name="alert" size={18} />{error} <button type="button" class="ml-2 underline" onclick={load}>Reintentar</button></p>
{:else if !s || !providers}
  <div class="card"><LoadingRows /></div>
{:else}
  <div class="grid gap-4 {dirty ? 'pb-24' : ''}">
    <BusinessSection bind:s />
    <TicketSection bind:s />
    <RulesSection bind:s />
    <MethodsSection bind:s {providers} />
    <MercadoPagoSection {providers} onchanged={refreshProviders} />
    <PrinterSection bind:s />
  </div>

  {#if dirty || saveOp.phase === 'error'}
    <div class="page-in fixed inset-x-0 bottom-0 z-40 border-t border-app-ink/10 bg-app-panel/95 px-4 py-3 shadow-app backdrop-blur" role="region" aria-label="Cambios sin guardar">
      <div class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3">
        <p class="text-sm {saveOp.phase === 'error' ? 'font-medium text-app-danger' : 'text-app-muted'}" role={saveOp.phase === 'error' ? 'alert' : undefined}>
          {saveOp.phase === 'error' ? saveOp.message : 'Tienes cambios sin guardar.'}
        </p>
        <div class="flex gap-2">
          <button type="button" class="btn-secondary" disabled={saveOp.phase === 'loading'} onclick={discard}>Descartar</button>
          <button type="button" class="btn-primary" disabled={saveOp.phase === 'loading'} onclick={save}>
            {#if saveOp.phase === 'loading'}<span class="spin"></span>{/if}Guardar cambios
          </button>
        </div>
      </div>
    </div>
  {/if}
{/if}
