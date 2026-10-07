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

  type Tone = 'primary' | 'accent' | 'warn' | 'danger' | 'muted';
  interface Item {
    id: string;
    label: string;
    desc: string;
    icon: IconName;
    tone: Tone;
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
        { id: 'negocio', label: 'Datos del consultorio', desc: 'Nombre, giro, especialidades y horario', icon: 'building', tone: 'primary', show: admin },
        { id: 'ticket', label: 'Datos fiscales y ticket', desc: 'RFC, domicilio fiscal y cómo sale tu ticket', icon: 'receipt', tone: 'accent', show: cobros },
        { id: 'ventas', label: 'Ventas y pagos', desc: 'IVA, descuentos y métodos de pago', icon: 'cash', tone: 'warn', show: cobros },
        { id: 'terminal', label: 'Mercado Pago y terminal', desc: 'Conecta tu cuenta y tu terminal Point', icon: 'wallet', tone: 'primary', show: cobros }
      ]
    },
    {
      title: 'Equipo y plan',
      items: [
        { id: 'equipo', to: '/equipo', label: 'Equipo', desc: 'Quién entra, roles y lugares del plan', icon: 'users', tone: 'accent', show: admin },
        { id: 'plan', to: '/suscripcion', label: 'Suscripción y plan', desc: 'Tu plan, pagos y facturación de Caresia', icon: 'sparkles', tone: 'warn', show: () => session.has(PERMISSIONS.adminUsers) }
      ]
    },
    {
      title: 'Esta app',
      items: [
        { id: 'impresora', label: 'Impresora de tickets', desc: 'Térmica USB, Bluetooth o la del navegador', icon: 'receipt', tone: 'muted', show: cobros },
        { id: 'apariencia', label: 'Apariencia', desc: 'Tema claro u oscuro', icon: 'sun', tone: 'muted', show: () => true },
        { id: 'cuenta', label: 'Mi cuenta', desc: 'Tus datos y tu contraseña', icon: 'user', tone: 'primary', show: () => true }
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

  const TONE: Record<Tone, string> = {
    primary: 'bg-app-primary/12 text-app-primary',
    accent: 'bg-app-accent/14 text-app-accent',
    warn: 'bg-app-warning/14 text-app-warning',
    danger: 'bg-app-danger/12 text-app-danger',
    muted: 'bg-app-ink/8 text-app-muted'
  };

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
  <!-- Categories -->
  <nav class="grid min-w-0 gap-5 {section && !wide ? 'hidden' : ''}" aria-label="Secciones de ajustes">
    <header class="px-1">
      <h1 class="display text-[2.25rem] leading-none sm:text-5xl">Ajustes</h1>
      <p class="mt-2 text-[15px] text-app-muted">Tu consultorio, tus cobros y esta app.</p>
    </header>

    {#each groups as g (g.title)}
      <div>
        <h2 class="mb-1.5 px-2 font-mono text-[11px] uppercase tracking-[0.14em] text-app-muted">{g.title}</h2>
        <ul class="card divide-y divide-app-ink/8 overflow-hidden p-0">
          {#each g.items as item (item.id)}
            {@const on = !item.to && section?.id === item.id}
            <li>
              {#if item.to}
                <a href={item.to} class="flex items-center gap-3 px-4 py-3.5 transition hover:bg-app-ink/[0.04]">
                  <span class="grid h-10 w-10 flex-none place-items-center rounded-xl {TONE[item.tone]}"><Icon name={item.icon} size={19} /></span>
                  <span class="min-w-0 flex-1">
                    <strong class="block truncate text-[15px] font-semibold">{item.label}</strong>
                    <small class="block truncate text-xs text-app-muted">{item.desc}</small>
                  </span>
                  <Icon name="chevron-down" size={18} class="-rotate-90 text-app-muted" />
                </a>
              {:else}
                <button
                  type="button"
                  class="flex w-full items-center gap-3 px-4 py-3.5 text-left transition {on ? 'bg-app-primary/8' : 'hover:bg-app-ink/[0.04]'}"
                  aria-current={on ? 'page' : undefined}
                  onclick={() => open(item.id)}
                >
                  <span class="grid h-10 w-10 flex-none place-items-center rounded-xl {TONE[item.tone]}"><Icon name={item.icon} size={19} /></span>
                  <span class="min-w-0 flex-1">
                    <strong class="flex items-center gap-1.5 truncate text-[15px] font-semibold {on ? 'text-app-primary' : ''}">
                      {item.label}{#if dirtyIn(item.id)}<i class="h-2 w-2 flex-none rounded-full bg-app-warning" aria-label="Con cambios sin guardar"></i>{/if}
                    </strong>
                    <small class="block truncate text-xs text-app-muted">{item.desc}</small>
                  </span>
                  <Icon name="chevron-down" size={18} class="-rotate-90 text-app-muted" />
                </button>
              {/if}
            </li>
          {/each}
          {#if g.title === 'Esta app'}
            <li>
              <button type="button" class="flex w-full items-center gap-3 px-4 py-3.5 text-left transition hover:bg-app-danger/[0.06]" onclick={logout}>
                <span class="grid h-10 w-10 flex-none place-items-center rounded-xl {TONE.danger}"><Icon name="logout" size={19} /></span>
                <span class="min-w-0 flex-1">
                  <strong class="block truncate text-[15px] font-semibold">Cerrar sesión</strong>
                  <small class="block truncate text-xs text-app-muted">{session.user?.username ? `Sales como ${session.user.username}` : 'Salir de este dispositivo'}</small>
                </span>
              </button>
            </li>
          {/if}
        </ul>
      </div>
    {/each}
  </nav>

  <!-- Section -->
  {#if section}
    <main class="min-w-0">
      <header class="mb-4 flex items-center gap-3">
        {#if !wide}
          <button type="button" class="icon-btn" aria-label="Volver a Ajustes" onclick={back}><Icon name="arrow-left" size={20} /></button>
        {/if}
        <span class="grid h-11 w-11 flex-none place-items-center rounded-xl {TONE[section.tone]}"><Icon name={section.icon} size={21} /></span>
        <div class="min-w-0">
          <h2 class="display text-3xl leading-none">{section.label}</h2>
          <p class="mt-1 truncate text-sm text-app-muted">{section.desc}</p>
        </div>
      </header>

      {#if section.id === 'negocio'}
        <ClinicSettings />
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
