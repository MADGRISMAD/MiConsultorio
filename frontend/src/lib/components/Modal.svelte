<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    open: boolean;
    title: string;
    onclose: () => void;
    children: Snippet;
    footer?: Snippet;
    wide?: boolean;
  }
  let { open, title, onclose, children, footer, wide = false }: Props = $props();

  let dialog = $state<HTMLDivElement>();

  $effect(() => {
    if (!open) return;
    const previous = document.activeElement as HTMLElement | null;
    document.body.style.overflow = 'hidden';
    dialog?.focus();
    return () => {
      document.body.style.overflow = '';
      previous?.focus?.();
    };
  });

  function onkeydown(e: KeyboardEvent) {
    if (open && e.key === 'Escape') onclose();
  }
</script>

<svelte:window {onkeydown} />

{#if open}
  <div class="fixed inset-0 z-50 overflow-y-auto">
    <div class="flex min-h-full items-center justify-center p-4">
      <button type="button" aria-label="Cerrar" class="fixed inset-0 cursor-default bg-ink/50 backdrop-blur-sm" onclick={onclose}></button>
      <div
        bind:this={dialog}
        role="dialog"
        aria-modal="true"
        aria-labelledby="modal-title"
        tabindex="-1"
        class="relative w-full rounded-2xl bg-white p-6 shadow-2xl outline-none sm:p-8 {wide ? 'max-w-4xl' : 'max-w-xl'}"
      >
        <h2 id="modal-title" class="font-display text-3xl leading-tight text-ink">{title}</h2>
        <div class="py-6">{@render children()}</div>
        {#if footer}
          <div class="flex flex-wrap justify-end gap-3">{@render footer()}</div>
        {/if}
      </div>
    </div>
  </div>
{/if}
