<script lang="ts">
  import { contactEmail, faqs } from './data';
  import SectionHead from './SectionHead.svelte';
  import { EASE } from './motion';

  let open = $state<number | null>(null);
</script>

<section id="preguntas" class="mx-auto max-w-6xl scroll-mt-24 px-5 pb-16 sm:px-8 lg:pb-24">
  <SectionHead title={'Antes de\n*decidir.*'} />
  <div class="mt-4 grid gap-8 lg:grid-cols-[1fr_1.6fr] lg:gap-16">
    <p class="max-w-sm pt-4 text-base leading-relaxed text-ink-soft sm:text-lg">
      ¿Otra duda? Escríbenos a <a href="mailto:{contactEmail}" class="text-ink underline decoration-ink/30 underline-offset-4 hover:decoration-signal">{contactEmail}</a> y te respondemos.
    </p>
    <ul>
      {#each faqs as f, i}
        {@const isOpen = open === i}
        <li class="border-b border-ink/15">
          <h3>
            <button
              type="button"
              aria-expanded={isOpen}
              aria-controls="faq-{i}"
              onclick={() => (open = isOpen ? null : i)}
              class="group flex min-h-14 w-full items-center justify-between gap-6 py-4 text-left"
            >
              <span class="text-lg font-medium transition-colors group-hover:text-signal">{f.q}</span>
              <span class="relative grid h-9 w-9 flex-none place-items-center rounded-full transition-colors duration-300 {isOpen ? 'bg-ink text-paper' : 'ring-1 ring-ink/20'}">
                <span class="absolute h-[1.5px] w-3.5 bg-current"></span>
                <span class="absolute h-3.5 w-[1.5px] bg-current" style="transform: rotate({isOpen ? 90 : 0}deg); transition: transform 0.4s {EASE}"></span>
              </span>
            </button>
          </h3>
          <!-- grid rows 0fr -> 1fr animates height without measuring -->
          <div id="faq-{i}" role="region" class="grid transition-[grid-template-rows,opacity] duration-300" style="grid-template-rows: {isOpen ? '1fr' : '0fr'}; opacity: {isOpen ? 1 : 0}; transition-timing-function: {EASE}">
            <div class="overflow-hidden">
              <p class="max-w-xl pb-5 pr-12 text-[16px] leading-relaxed text-ink-soft">{f.a}</p>
            </div>
          </div>
        </li>
      {/each}
    </ul>
  </div>
</section>
