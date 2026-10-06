<script lang="ts">
  import type { Snippet } from 'svelte';
  import { theme } from '$lib/theme.svelte';
  import BrandMark from './ui/BrandMark.svelte';
  import BrandName from './ui/BrandName.svelte';
  import Icon from './ui/Icon.svelte';

  let { wide = false, children }: { wide?: boolean; children: Snippet } = $props();

  $effect(() => theme.init());

  const points = [
    'Agenda de citas sin empalmes para todo tu equipo',
    'Expedientes e historiales clínicos siempre a la mano',
    'Permisos por rol: cada quien ve solo lo que le toca'
  ];
</script>

<div
  class="app fixed inset-0 overflow-y-auto md:grid md:grid-cols-[minmax(16rem,0.75fr)_minmax(0,1.25fr)] {wide
    ? 'lg:grid-cols-[minmax(18rem,0.75fr)_minmax(0,1.25fr)]'
    : 'lg:grid-cols-[minmax(20rem,0.9fr)_minmax(0,1.1fr)]'}"
  data-theme={theme.mode}>
  <aside
    class="relative flex flex-col justify-between gap-8 overflow-hidden px-5 pb-16 pt-4 text-white md:sticky md:top-0 md:h-dvh md:self-start md:px-8 md:py-9 lg:px-10"
    style="background: radial-gradient(ellipse 70% 45% at 10% 10%, rgba(224,138,30,.28), transparent 60%), radial-gradient(ellipse 60% 50% at 95% 95%, rgba(91,154,232,.35), transparent 60%), linear-gradient(160deg, #0a1a30 0%, #123056 45%, #1e5aa8 100%)"
  >
    <a href="/" class="inline-flex items-center gap-3 text-[1.35rem]" aria-label="Caresia, ir al inicio">
      <BrandMark size={42} />
      <BrandName dark />
    </a>

    <div class="hidden max-w-[26rem] md:block">
      <h2 class="text-[clamp(1.6rem,2.6vw,2.15rem)] font-extrabold leading-[1.15] tracking-tight">El software para tu consultorio, clínica dental o veterinaria.</h2>
      <ul class="mt-6 space-y-3 text-[15px] leading-snug text-white/90">
        {#each points as p}
          <li class="flex items-start gap-3">
            <span class="mt-px grid h-6 w-6 flex-none place-items-center rounded-full bg-[#f0a94a]/20 text-[#f0a94a]"><Icon name="check" size={14} stroke={3} /></span>
            {p}
          </li>
        {/each}
      </ul>
      <p class="mt-7 inline-flex items-center gap-2 rounded-full bg-white/12 px-3.5 py-2 text-sm font-bold">
        <Icon name="shield" size={16} />Cada consultorio ve únicamente sus propios datos
      </p>
    </div>

    <p class="hidden text-[13px] text-white/60 md:block">En la nube · computadora, tableta y celular</p>
  </aside>

  <main class="relative flex flex-col items-center justify-center gap-4 px-3.5 pb-8 pt-0 max-md:-mt-10 md:px-6 md:py-10 {wide ? 'lg:py-5' : ''}">
    <button
      type="button"
      class="icon-btn absolute right-3 top-3 max-md:hidden"
      title={theme.mode === 'dark' ? 'Tema claro' : 'Tema oscuro'}
      aria-label={theme.mode === 'dark' ? 'Cambiar a tema claro' : 'Cambiar a tema oscuro'}
      onclick={() => theme.toggle()}
    >
      <Icon name={theme.mode === 'dark' ? 'sun' : 'moon'} size={20} />
    </button>

    <div class="card page-in w-full px-5 py-6 sm:px-8 sm:py-8 {wide ? 'max-w-[38rem]' : 'max-w-[27rem]'}">
      {@render children()}
    </div>
    <p class="flex gap-2 text-[13px] text-app-muted">
      <a href="/terminos" class="hover:underline">Términos</a><span aria-hidden="true">·</span><a href="/privacidad" class="hover:underline">Aviso de privacidad</a>
    </p>
  </main>
</div>
