<script lang="ts">
  import { onMount } from 'svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { theme } from '$lib/theme.svelte';
  import ButtonLabel from './ButtonLabel.svelte';
  import Wordmark from './Wordmark.svelte';
  import { demoHref, loginPath } from './data';
  import { btn, EASE, onFrame, pageProgress, reducedMotion, Spring } from './motion';

  const links = [
    ['#funciones', 'Funciones'],
    ['#especialidades', 'Especialidades'],
    ['#precios', 'Precios'],
    ['#preguntas', 'Preguntas']
  ];
  const mobileLinks = [...links, [loginPath, 'Iniciar sesión']];

  let hidden = $state(false);
  let solid = $state(false);
  let open = $state(false);
  let bar = $state<HTMLDivElement>();

  onMount(() => {
    const reduce = reducedMotion();
    const spring = new Spring(pageProgress(), { stiffness: 140, damping: 30, mass: 0.3 });
    let prevY = scrollY;
    const stop = onFrame((dt) => {
      const y = scrollY;
      solid = y > 40;
      hidden = y > 400 && y > prevY && !open;
      prevY = y;
      const p = reduce ? pageProgress() : spring.step(pageProgress(), dt);
      if (bar) bar.style.transform = `scaleX(${p})`;
    });
    return stop;
  });

  $effect(() => {
    document.documentElement.style.overflow = open ? 'hidden' : '';
    return () => {
      document.documentElement.style.overflow = '';
    };
  });
</script>

<div bind:this={bar} aria-hidden="true" class="fixed inset-x-0 top-0 z-[70] h-[2px] origin-left scale-x-0 bg-signal"></div>

<header class="fixed inset-x-0 top-0 z-50 px-3 pt-3 transition-transform duration-500 sm:px-5" style="transform: translateY({hidden ? -100 : 0}px); transition-timing-function: {EASE}">
  <nav
    class="mx-auto flex h-14 max-w-6xl items-center justify-between rounded-full pl-4 pr-2 transition-[background-color,box-shadow,backdrop-filter] duration-500 {solid
      ? 'bg-paper/75 shadow-[0_1px_0_rgba(11,37,64,0.06),0_12px_32px_-12px_rgba(11,37,64,0.18)] ring-1 ring-ink/5 backdrop-blur-xl'
      : ''}"
  >
    <a href="#inicio" aria-label="Caresia, inicio" class="rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-signal">
      <Wordmark />
    </a>
    <div class="hidden items-center md:flex">
      {#each links as [href, label]}
        <a {href} class="group relative px-3.5 py-2 text-[15px] text-ink-soft transition-colors hover:text-ink">
          {label}
          <span class="absolute inset-x-3.5 bottom-1 h-px origin-right scale-x-0 bg-ink transition-transform duration-500 ease-out-strong group-hover:origin-left group-hover:scale-x-100"></span>
        </a>
      {/each}
    </div>
    <div class="flex items-center gap-1.5">
      <button
        type="button"
        class="grid h-10 w-10 place-items-center rounded-full text-ink-soft transition-colors hover:bg-ink/5 hover:text-ink focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-signal"
        title={theme.mode === 'dark' ? 'Tema claro' : 'Tema oscuro'}
        aria-label={theme.mode === 'dark' ? 'Cambiar a tema claro' : 'Cambiar a tema oscuro'}
        onclick={() => theme.toggle()}
      >
        <Icon name={theme.mode === 'dark' ? 'sun' : 'moon'} size={20} />
      </button>
      <a href={loginPath} class="hidden px-3 text-[15px] font-medium text-ink-soft hover:text-ink sm:block">Entrar</a>
      <a href={demoHref} class="{btn} h-10 bg-ink px-5 text-sm text-paper hover:bg-signal">
        <ButtonLabel>Solicitar demo</ButtonLabel>
      </a>
      <button
        type="button"
        aria-label={open ? 'Cerrar menú' : 'Abrir menú'}
        aria-expanded={open}
        aria-controls="mobile-menu"
        onclick={() => (open = !open)}
        class="relative grid h-10 w-10 place-items-center rounded-full md:hidden"
      >
        <span class="absolute h-[1.5px] w-5 bg-ink transition-transform duration-300 {open ? 'rotate-45' : '-translate-y-1'}"></span>
        <span class="absolute h-[1.5px] w-5 bg-ink transition-transform duration-300 {open ? '-rotate-45' : 'translate-y-1'}"></span>
      </button>
    </div>
  </nav>
</header>

<!-- Mobile menu: wipes down from the top -->
<div
  id="mobile-menu"
  inert={!open}
  class="fixed inset-0 z-40 flex flex-col justify-end bg-paper px-6 pb-12 pt-24 md:hidden"
  style="clip-path: inset(0 0 {open ? 0 : 100}% 0); transition: clip-path 0.7s {EASE}; visibility: {open ? 'visible' : 'hidden'}; transition-property: clip-path, visibility; transition-duration: 0.7s, 0s; transition-delay: 0s, {open ? 0 : 0.7}s"
>
  <ul class="space-y-1">
    {#each mobileLinks as [href, label], i}
      <li class="overflow-hidden">
        <a
          {href}
          onclick={() => (open = false)}
          class="block font-display text-6xl leading-[1.1] text-ink"
          style="transform: translateY({open ? 0 : 100}%); transition: transform 0.8s {EASE} {open ? 0.15 + i * 0.06 : 0}s">{label}</a
        >
      </li>
    {/each}
  </ul>
  <p class="mt-10 font-mono text-xs uppercase tracking-[0.14em] text-ink-faint">Dental · Medicina general</p>
</div>
