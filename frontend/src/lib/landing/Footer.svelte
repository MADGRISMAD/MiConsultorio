<script lang="ts">
  import Wordmark from './Wordmark.svelte';
  import { contactEmail, loginPath, registerHref } from './data';
  import { cookieConsent } from '$lib/cookies.svelte';

  /** home="/" fuera de la portada: las anclas apuntan a la portada. */
  let { home = '' }: { home?: string } = $props();
  const cols: [string, [string, string][]][] = $derived([
    ['Producto', [[`${home}#funciones`, 'Funciones'], [`${home}#especialidades`, 'Especialidades'], ['/software-dental', 'Software dental'], ['/software-medico', 'Software médico'], ['/software-nutricion', 'Software para nutrición'], ['/software-veterinaria', 'Software veterinario'], [`${home}#incluye`, 'Más funciones'], [`${home}#precios`, 'Precios'], [`${home}#preguntas`, 'Preguntas frecuentes']]],
    ['Pacientes', [['/directorio', 'Buscar especialista'], ['/blog', 'Blog']]],
    ['Cuenta', [[loginPath, 'Iniciar sesión'], [registerHref, 'Solicitar demo'], [`mailto:${contactEmail}`, 'Contacto']]]
  ]);
</script>

<footer class="l-deep relative mt-3 overflow-hidden bg-ink text-paper">
  <div class="mx-auto grid max-w-6xl gap-12 px-5 pb-8 pt-14 sm:px-8 md:grid-cols-[1.6fr_1fr_1fr_1fr]">
    <div>
      <Wordmark light />
      <p class="mt-5 max-w-xs text-[15px] leading-relaxed text-paper/55">Software para consultorios y clínicas.</p>
    </div>
    {#each cols as [title, links]}
      <div>
        <p class="font-mono text-[11px] uppercase tracking-[0.16em] text-paper/60">{title}</p>
        <ul class="mt-5 space-y-3 text-[15px]">
          {#each links as [href, label]}
            <li><a {href} class="text-paper/80 transition-colors hover:text-signal">{label}</a></li>
          {/each}
        </ul>
      </div>
    {/each}
  </div>
  <div class="mx-auto max-w-6xl px-5 sm:px-8">
    <div class="flex flex-wrap justify-between gap-3 border-t border-paper/10 py-6 font-mono text-[11px] uppercase tracking-[0.12em] text-paper/60">
      <span>© {new Date().getFullYear()} Caresia · Todos los derechos reservados</span>
      <span class="flex flex-wrap gap-x-5 gap-y-2"><a href="/privacidad" class="transition-colors hover:text-signal">Aviso de privacidad</a><a href="/terminos" class="transition-colors hover:text-signal">Términos de uso</a><button type="button" onclick={() => cookieConsent.reopen()} class="uppercase tracking-[0.12em] transition-colors hover:text-signal">Preferencias de cookies</button></span>
    </div>
  </div>
</footer>
