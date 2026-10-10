<script lang="ts">
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import AgendaSettings from './AgendaSettings.svelte';
  import PortalSettings from './PortalSettings.svelte';
  import ProfileSettings from './ProfileSettings.svelte';

  type Tab = 'reservas' | 'pagina' | 'directorio' | 'portal' | 'opiniones';
  const TABS: { id: Tab; title: string; what: string }[] = [
    { id: 'reservas', title: 'Reservas', what: 'Tu dirección y que los pacientes agenden solos' },
    { id: 'pagina', title: 'Página', what: 'Tu página con servicios, equipo y fotos' },
    { id: 'directorio', title: 'Directorio', what: 'Aparecer en el buscador de Caresia' },
    { id: 'portal', title: 'Portal', what: 'Citas y recetas de tus pacientes' },
    { id: 'opiniones', title: 'Opiniones', what: 'Encuesta, Google y respuestas' }
  ];

  // Lo que reporta cada parte; el resumen de arriba se arma con esto.
  let slug = $state('');
  let booking = $state<boolean | null>(null);
  let pageOn = $state<boolean | null>(null);
  let listed = $state<boolean | null>(null);
  let portal = $state<boolean | null>(null);
  let survey = $state<boolean | null>(null);

  // «?s=perfil» (enlace viejo) abre directamente la página del consultorio
  const requested = $derived((page.url.searchParams.get('t') ?? (page.url.searchParams.get('s') === 'perfil' ? 'pagina' : null)) as Tab | null);
  let tab = $state<Tab>('reservas');
  $effect(() => {
    if (requested && TABS.some((t) => t.id === requested)) tab = requested;
  });
  function go(t: string) {
    tab = t as Tab;
    const u = new URL(page.url);
    u.searchParams.set('t', t);
    void goto(u, { replaceState: true, noScroll: true, keepFocus: true });
  }

  const status = (id: Tab): { text: string; on: boolean | null } => {
    switch (id) {
      case 'reservas':
        return !slug && booking !== null ? { text: 'Falta tu dirección', on: false } : { text: booking ? 'Activas' : 'Apagadas', on: booking };
      case 'pagina':
        return { text: pageOn ? 'Publicada' : 'Sin publicar', on: pageOn };
      case 'directorio':
        return { text: listed ? 'Apareces' : 'No apareces', on: listed };
      case 'portal':
        return { text: portal ? 'Activo' : 'Apagado', on: portal };
      case 'opiniones':
        return { text: survey ? 'Encuesta activa' : 'Sin encuesta', on: survey };
    }
  };
  const profileTab = $derived((tab === 'pagina' || tab === 'directorio' || tab === 'opiniones' ? tab : 'pagina') as 'pagina' | 'directorio' | 'opiniones');
  const active = $derived(TABS.find((t) => t.id === tab)!);
</script>

<div class="grid gap-6">
  <!-- Los cinco pasos, con su estado: de un vistazo qué está encendido y qué no -->
  <div role="tablist" aria-label="Reservas y página pública" class="-mx-1 flex gap-2 overflow-x-auto px-1 pb-1 [scrollbar-width:none] md:grid md:grid-cols-5">
    {#each TABS as t, i (t.id)}
      {@const st = status(t.id)}
      <button
        type="button"
        role="tab"
        id="on-tab-{t.id}"
        aria-selected={tab === t.id}
        aria-controls="on-panel"
        class="flex min-w-[8.5rem] flex-none flex-col items-start gap-1 rounded-2xl px-3.5 py-3 text-left transition md:min-w-0 {tab === t.id ? 'bg-app-ink text-app-surface' : 'bg-app-panel ring-1 ring-app-ink/12 hover:ring-app-ink/30'}"
        onclick={() => go(t.id)}
      >
        <span class="flex w-full items-baseline gap-2">
          <span class="font-mono text-[11px] {tab === t.id ? 'opacity-70' : 'text-app-muted'}">0{i + 1}</span>
          <span class="text-[15px] font-medium">{t.title}</span>
        </span>
        <span class="flex items-center gap-1.5 text-xs font-medium {tab === t.id ? 'opacity-90' : st.on ? 'text-app-accent' : 'text-app-muted'}">
          <i class="h-2 w-2 flex-none rounded-full {st.on === null ? 'bg-app-ink/20' : st.on ? 'bg-app-accent' : 'bg-app-ink/30'}"></i>{st.on === null ? '…' : st.text}
        </span>
      </button>
    {/each}
  </div>

  <div id="on-panel" role="tabpanel" aria-labelledby="on-tab-{tab}" class="grid gap-5">
    <p class="text-[15px] text-app-muted">{active.what}.</p>

    <!-- Una sola copia de cada parte, siempre montada: no se pierde lo escrito al cambiar de paso -->
    <div class={tab === 'reservas' ? '' : '!hidden'}>
      <AgendaSettings part="online" onstate={(v) => ((slug = v.slug), (booking = v.enabled))} />
    </div>
    <div class={tab === 'portal' ? '' : '!hidden'}>
      <PortalSettings slugOverride={slug} onstate={(v) => (portal = v.enabled)} goto={go} />
    </div>
    <div class={tab === 'pagina' || tab === 'directorio' || tab === 'opiniones' ? '' : '!hidden'}>
      <ProfileSettings tab={profileTab} slugOverride={slug} goto={go} onstate={(v) => ((pageOn = v.enabled), (listed = v.listed), (survey = v.survey))} />
    </div>
  </div>
</div>
