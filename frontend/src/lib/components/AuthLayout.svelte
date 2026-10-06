<script lang="ts">
  import type { Snippet } from 'svelte';
  import { theme } from '$lib/theme.svelte';
  import Ecg from '$lib/landing/Ecg.svelte';
  import Brand from './ui/Brand.svelte';
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
  data-theme={theme.mode}
>
  <!-- Brand panel: the landing's ink section, with its heart-monitor line -->
  <aside
    class="relative flex flex-col justify-between gap-8 overflow-hidden bg-[#0B2540] px-5 pb-16 pt-4 text-[#F4F8FB] md:sticky md:top-0 md:h-dvh md:self-start md:px-8 md:py-9 lg:px-12"
    style="background-image: radial-gradient(ellipse 70% 50% at 10% 0%, rgba(22,115,209,.38), transparent 62%), radial-gradient(ellipse 55% 45% at 100% 100%, rgba(15,158,142,.22), transparent 60%)"
  >
    <Ecg class="pointer-events-none absolute inset-x-[-10%] bottom-[14%] hidden h-28 w-[120%] md:block" base="stroke-white/10" sweep="stroke-[#56A4F0]" beats={4} />

    <a href="/" class="relative inline-flex" aria-label="Caresia, ir al inicio"><Brand onDark size={34} class="text-[1.1rem]" /></a>

    <div class="relative hidden max-w-[28rem] md:block">
      <p class="mb-5 font-mono text-[11px] uppercase tracking-[0.16em] text-white/55">Software clínico</p>
      <h2 class="display text-[clamp(2.4rem,4.2vw,3.6rem)] leading-[0.98]">Tu consultorio,<br /><em class="italic text-[#56A4F0]">en orden.</em></h2>
      <ul class="mt-8 space-y-3.5 text-[15px] leading-snug text-white/80">
        {#each points as p}
          <li class="flex items-start gap-3">
            <span class="mt-px grid h-5 w-5 flex-none place-items-center rounded-full bg-[#0F9E8E]/25 text-[#2DC4B2]"><Icon name="check" size={12} stroke={3} /></span>
            {p}
          </li>
        {/each}
      </ul>
      <p class="mt-9 inline-flex items-center gap-2 rounded-full px-3.5 py-2 text-sm text-white/80 ring-1 ring-inset ring-white/15">
        <Icon name="shield" size={16} class="text-[#2DC4B2]" />Cada consultorio ve únicamente sus propios datos
      </p>
    </div>

    <p class="relative hidden font-mono text-[11px] uppercase tracking-[0.14em] text-white/45 md:block">Dental · Medicina general · Veterinaria</p>
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

    <div class="card page-in w-full px-6 py-7 sm:px-9 sm:py-9 {wide ? 'max-w-[38rem]' : 'max-w-[27rem]'}">
      {@render children()}
    </div>
    <p class="flex gap-2 text-[13px] text-app-muted">
      <a href="/terminos" class="hover:text-app-ink hover:underline">Términos</a><span aria-hidden="true">·</span><a href="/privacidad" class="hover:text-app-ink hover:underline">Aviso de privacidad</a>
    </p>
  </main>
</div>
