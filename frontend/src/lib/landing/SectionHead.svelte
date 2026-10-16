<script lang="ts">
  import Split from './Split.svelte';

  /** Encabezado común de cada sección: titular serif con acento en cursiva y una frase de apoyo. */
  let { title, blurb }: { title: string; blurb?: string } = $props();
</script>

<div class="grid gap-5 border-b border-ink/15 pb-8 md:grid-cols-[1.7fr_1fr] md:items-end">
  <Split text={title} class="font-display text-[clamp(2.25rem,4.5vw,4rem)] leading-[0.98] tracking-[-0.03em]" accent="italic text-signal" />
  {#if blurb}<p class="max-w-sm text-base leading-relaxed text-ink-soft sm:text-lg md:justify-self-end">{blurb}</p>{/if}
</div>
