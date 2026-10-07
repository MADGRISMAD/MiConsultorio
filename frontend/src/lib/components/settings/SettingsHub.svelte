<script lang="ts">
  import { untrack } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import { PERMISSIONS, type PosSettings, type ProviderStatus } from '$lib/types';
  import AccountPanel from '$lib/components/AccountPanel.svelte';
  import BusinessSection from '$lib/components/pos/settings/BusinessSection.svelte';
  import MercadoPagoSection from '$lib/components/pos/settings/MercadoPagoSection.svelte';
  import MethodsSection from '$lib/components/pos/settings/MethodsSection.svelte';
  import PrinterSection from '$lib/components/pos/settings/PrinterSection.svelte';
  import RulesSection from '$lib/components/pos/settings/RulesSection.svelte';
  import TicketSection from '$lib/components/pos/settings/TicketSection.svelte';
  import Icon, { type IconName } from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import Appearance from './Appearance.svelte';
  import ClinicSettings from './ClinicSettings.svelte';
  import ComplianceSection from './ComplianceSection.svelte';

  interface Item {
    id: string;
    label: string;
    desc: string;
    icon: IconName;
    /** opens another page instead of a section here */
    to?: string;
    show: () => boolean;
  }
  interface Group {
    title: string;
    items: Item[];
  }

  const unlocked = () => !session.locked;
  const admin = () => unlocked() && session.has(PERMISSIONS.adminUsers);
  const cobros = () => unlocked() && session.cobros && session.has(PERMISSIONS.posManage);

  const GROUPS: Group[] = [
    {
      title: 'Tu consultorio',
      items: [
        { id: 'negocio', label: 'Datos del consultorio', desc: 'Nombre, giro, especialidades y horario', icon: 'building', show: admin },
        { id: 'cumplimiento', label: 'Cumplimiento (México)', desc: 'Datos legales, aviso de privacidad y pendientes', icon: 'shield', show: admin },
        { id: 'ticket', label: 'Datos fiscales y ticket', desc: 'RFC, domicilio fiscal y cómo sale tu ticket', icon: 'receipt', show: cobros },
        { id: 'ventas', label: 'Ventas y pagos', desc: 'IVA, descuentos y métodos de pago', icon: 'cash', show: cobros },
        { id: 'terminal', label: 'Mercado Pago y terminal', desc: 'Conecta tu cuenta y tu terminal Point', icon: 'wallet', show: cobros }
      ]
    },
    {
      title: 'Equipo y plan',
      items: [
        { id: 'equipo', to: '/equipo', label: 'Equipo', desc: 'Quién entra, roles y lugares del plan', icon: 'users', show: admin },
        { id: 'plan', to: '/suscripcion', label: 'Suscripción y plan', desc: 'Tu plan, pagos y facturación de Caresia', icon: 'sparkles', show: () => session.has(PERMISSIONS.adminUsers) }
      ]
    },
    {
      title: 'Esta app',
      items: [
        { id: 'impresora', label: 'Impresora de tickets', desc: 'Térmica USB, Bluetooth o la del navegador', icon: 'receipt', show: cobros },
        { id: 'apariencia', label: 'Apariencia', desc: 'Tema claro u oscuro', icon: 'sun', show: () => true },
        { id: 'cuenta', label: 'Mi cuenta', desc: 'Tus datos y tu contraseña', icon: 'user', show: () => true }
      ]
    }
  ];

  const groups = $derived(GROUPS.map((g) => ({ ...g, items: g.items.filter((i) => i.show()) })).filter((g) => g.items.length));
  const sections = $derived(groups.flatMap((g) => g.items).filter((i) => !i.to));
  const requested = $derived(page.url.searchParams.get('s') ?? '');

  // Wide screens always show one section; on a phone the list comes first.
  let wide = $state(true);
  $effect(() => {
    const mq = window.matchMedia('(min-width: 768px)');
    wide = mq.matches;
    const on = (e: MediaQueryListEvent) => (wide = e.matches);
    mq.addEventListener('change', on);
    return () => mq.removeEventListener('change', on);
  });

  const section = $derived.by(() => {
    const hit = sections.find((i) => i.id === requested);
    if (hit) return hit;
    return wide ? (sections[0] ?? null) : null;
  });
  const open = (id: string) => goto(`/ajustes?s=${id}`, { noScroll: false, keepFocus: true });
  const back = () => goto('/ajustes', { keepFocus: true });

  // ---- cobros settings: loaded once, saved with one bar for every cobros section ----
  const POS_SECTIONS = ['ticket', 'ventas', 'terminal', 'impresora'];
  let s = $state<PosSettings | null>(null);
  let providers = $state<ProviderStatus | null>(null);
  let saved = $state('');
  let loadError = $state('');
  let started = false;
  const saveOp = new Op();

  const plain = (v: PosSettings) => JSON.stringify($state.snapshot(v));
  const dirty = $derived(!!s && plain(s) !== saved);

  async function loadPos() {
    loadError = '';
    try {
      const r = await api.pos.settings();
      s = r.settings;
      providers = r.providers;
      saved = plain(r.settings);
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'No se pudieron cargar los ajustes.';
    }
  }
  async function refreshProviders() {
    try {
      providers = (await api.pos.settings()).providers;
    } catch {
      /* keep the last known status */
    }
  }
  $effect(() => {
    if (section && POS_SECTIONS.includes(section.id) && !started) {
      started = true;
      untrack(() => void loadPos());
    }
  });

  // Which sections hold unsaved changes (the dot in the list)
  const KEYS: Record<string, (keyof PosSettings)[]> = {
    ticket: ['business_name', 'legal_name', 'rfc', 'tax_regime', 'tax_address', 'zip_code', 'phone', 'ticket_header', 'ticket_footer', 'show_tax_line'],
    ventas: ['currency', 'default_tax_rate', 'allow_negative_stock', 'require_open_cash', 'allow_discounts', 'max_discount_pct', 'methods'],
    impresora: ['printer']
  };
  function dirtyIn(id: string): boolean {
    if (!s || !saved || !KEYS[id]) return false;
    const base = JSON.parse(saved) as PosSettings;
    return KEYS[id].some((k) => JSON.stringify(s![k]) !== JSON.stringify(base[k]));
  }

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

  // Back from Mercado Pago's authorization screen
  $effect(() => {
    const mp = page.url.searchParams.get('mp');
    if (!mp) return;
    const reason = page.url.searchParams.get('reason') ?? '';
    const detail = page.url.searchParams.get('detail') ?? '';
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
      void goto('/ajustes?s=terminal', { replaceState: true, noScroll: true, keepFocus: true });
      if (started) void refreshProviders();
    });
  });

  async function logout() {
    await session.logout();
    await goto('/', { replaceState: true });
  }
</script>

<div class="grid gap-6 md:grid-cols-[minmax(17rem,20rem)_minmax(0,1fr)] md:items-start {dirty ? 'pb-24' : ''}">
  <!-- Categories: same language as the sidebar -->
  <nav class="grid min-w-0 grid-cols-[minmax(0,1fr)] gap-5 {section && !wide ? 'hidden' : ''}" aria-label="Secciones de ajustes">
    <header>
      <h1 class="display text-[2.25rem] leading-[1] sm:text-5xl">Ajustes</h1>
      <p class="mt-2 text-[15px] text-app-muted">Tu consultorio, tus cobros y esta app.</p>
    </header>

    <div class="card grid gap-5 p-3">
      {#each groups as g (g.title)}
        <div>
          <p class="section-title px-3 pb-2">{g.title}</p>
          <ul class="space-y-0.5">
            {#each g.items as item (item.id)}
              {@const on = !item.to && section?.id === item.id}
              <li>
                {#if item.to}
                  <a href={item.to} class="flex items-center gap-3 rounded-xl px-3 py-2.5 text-app-muted transition hover:bg-app-ink/5 hover:text-app-ink">
                    <Icon name={item.icon} size={20} class="flex-none" />
                    <span class="min-w-0 flex-1">
                      <span class="block truncate text-[15px] font-medium">{item.label}</span>
                      <small class="block truncate text-xs font-normal">{item.desc}</small>
                    </span>
                    <Icon name="chevron-down" size={16} class="-rotate-90 flex-none opacity-60" />
                  </a>
                {:else}
                  <button
                    type="button"
                    class="flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-left transition {on ? 'bg-app-primary/10 text-app-primary' : 'text-app-muted hover:bg-app-ink/5 hover:text-app-ink'}"
                    aria-current={on ? 'page' : undefined}
                    onclick={() => open(item.id)}
                  >
                    <Icon name={item.icon} size={20} class="flex-none" />
                    <span class="min-w-0 flex-1">
                      <span class="flex items-center gap-1.5 truncate text-[15px] font-medium">
                        {item.label}{#if dirtyIn(item.id)}<i class="h-2 w-2 flex-none rounded-full bg-app-warning" aria-label="Con cambios sin guardar"></i>{/if}
                      </span>
                      <small class="block truncate text-xs font-normal {on ? 'text-app-primary/80' : ''}">{item.desc}</small>
                    </span>
                    <Icon name="chevron-down" size={16} class="-rotate-90 flex-none opacity-60" />
                  </button>
                {/if}
              </li>
            {/each}
            {#if g.title === 'Esta app'}
              <li>
                <button type="button" class="flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-left text-app-muted transition hover:bg-app-danger/10 hover:text-app-danger" onclick={logout}>
                  <Icon name="logout" size={20} class="flex-none" />
                  <span class="min-w-0 flex-1">
                    <span class="block truncate text-[15px] font-medium">Cerrar sesión</span>
                    <small class="block truncate text-xs font-normal">{session.user?.username ? `Sales como ${session.user.username}` : 'Salir de este dispositivo'}</small>
                  </span>
                </button>
              </li>
            {/if}
          </ul>
        </div>
      {/each}
    </div>
  </nav>

  <!-- Section -->
  {#if section}
    <main class="min-w-0">
      <header class="mb-5 flex items-start gap-3">
        {#if !wide}
          <button type="button" class="icon-btn mt-1" aria-label="Volver a Ajustes" onclick={back}><Icon name="arrow-left" size={20} /></button>
        {/if}
        <div class="min-w-0">
          <h2 class="display text-[2rem] leading-[1.05] sm:text-4xl">{section.label}</h2>
          <p class="mt-1.5 text-[15px] text-app-muted">{section.desc}</p>
        </div>
      </header>

      {#if section.id === 'negocio'}
        <ClinicSettings />
      {:else if section.id === 'cumplimiento'}
        <ComplianceSection />
      {:else if section.id === 'apariencia'}
        <Appearance />
      {:else if section.id === 'cuenta'}
        <AccountPanel embedded />
      {:else if loadError}
        <p class="alert" role="alert"><Icon name="alert" size={18} />{loadError} <button type="button" class="ml-2 underline" onclick={loadPos}>Reintentar</button></p>
      {:else if !s || !providers}
        <div class="card"><LoadingRows /></div>
      {:else}
        <div class="grid gap-4">
          {#if section.id === 'ticket'}
            <BusinessSection bind:s />
            <TicketSection bind:s />
          {:else if section.id === 'ventas'}
            <RulesSection bind:s />
            <MethodsSection bind:s {providers} />
          {:else if section.id === 'terminal'}
            <MercadoPagoSection {providers} onchanged={refreshProviders} />
          {:else if section.id === 'impresora'}
            <PrinterSection bind:s />
          {/if}
        </div>
      {/if}
    </main>
  {:else if !wide}
    <!-- nothing: the list above is the page -->
  {:else}
    <div class="card"><p class="p-6 text-sm text-app-muted">No tienes ajustes disponibles con tu rol.</p></div>
  {/if}
</div>

{#if dirty || saveOp.phase === 'error'}
  <div class="page-in fixed inset-x-0 bottom-0 z-40 border-t border-app-ink/10 bg-app-panel/95 px-4 py-3 shadow-app backdrop-blur" role="region" aria-label="Cambios sin guardar">
    <div class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 lg:pl-72">
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
