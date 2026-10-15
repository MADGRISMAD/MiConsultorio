<script lang="ts">
  import { session } from '$lib/session.svelte';
  import { onMount } from 'svelte';
  import { fly } from 'svelte/transition';
  import { cubicOut } from 'svelte/easing';
  import { page } from '$app/state';
  import { PUBLIC_PATHS, cookieConsent } from '$lib/cookies.svelte';

  let ready = $state(false);
  const isPublic = $derived(PUBLIC_PATHS.includes(page.url.pathname) || page.url.pathname.startsWith('/blog/'));

  onMount(() => {
    cookieConsent.init();
    ready = true;
  });

  const reduce = () => typeof matchMedia !== 'undefined' && matchMedia('(prefers-reduced-motion: reduce)').matches;
</script>

{#if ready && session.status === 'anonymous' && cookieConsent.open && (isPublic || cookieConsent.manual)}
  <div
    role="dialog"
    aria-labelledby="cookies-title"
    aria-describedby="cookies-text"
    class="fixed inset-x-3 bottom-3 z-[80] sm:inset-x-auto sm:bottom-5 sm:left-5 sm:max-w-[25rem]"
    transition:fly={{ y: reduce() ? 0 : 24, duration: 500, easing: cubicOut }}
  >
    <div class="rounded-[22px] bg-ink p-5 font-body text-paper antialiased shadow-[0_24px_60px_-18px_rgba(11,37,64,0.55)] ring-1 ring-paper/10 sm:p-6">
      <div class="flex items-start gap-3">
        <span aria-hidden="true" class="mt-0.5 grid h-9 w-9 flex-none place-items-center rounded-full bg-paper/10">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" class="h-[18px] w-[18px]">
            <path d="M12 3a9 9 0 1 0 9 9 4 4 0 0 1-5-5 4 4 0 0 1-4-4z" />
            <path d="M8.5 11.5h.01M15.5 15.5h.01M11 16.5h.01M7.5 15h.01" stroke-width="2.4" />
          </svg>
        </span>
        <div>
          <p id="cookies-title" class="font-display text-[22px] leading-tight">Cookies en este sitio</p>
          <p id="cookies-text" class="mt-2 text-[14px] leading-relaxed text-paper/70">
            Con tu permiso usamos cookies de Google Analytics para medir visitas a las páginas públicas. Nunca se cargan dentro de la plataforma ni reciben datos de pacientes.
            <a href="/privacidad#cookies" class="whitespace-nowrap text-paper underline decoration-paper/40 underline-offset-4 hover:decoration-paper">Más información</a>
          </p>
        </div>
      </div>
      <div class="mt-5 grid grid-cols-2 gap-2">
        <button
          type="button"
          onclick={() => cookieConsent.reject()}
          class="h-11 rounded-full text-[14px] font-medium text-paper ring-1 ring-inset ring-paper/25 transition-colors hover:bg-paper/10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-signal active:scale-[0.97]"
        >
          Rechazar
        </button>
        <button
          type="button"
          onclick={() => cookieConsent.accept()}
          class="h-11 rounded-full bg-signal text-[14px] font-medium text-white transition-colors hover:bg-paper hover:text-ink focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-paper active:scale-[0.97]"
        >
          Aceptar
        </button>
      </div>
    </div>
  </div>
{/if}
