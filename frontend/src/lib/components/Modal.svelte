<script lang="ts">
  import type { Snippet } from 'svelte';
  import Icon from './ui/Icon.svelte';

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

  /** Moves the overlay to the `.app` root so no ancestor's stacking context can trap it. */
  function portal(node: HTMLElement) {
    const root = node.closest('.app') ?? document.body;
    root.appendChild(node);
    return { destroy: () => node.remove() };
  }
  const uid = $props.id();

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
  <div class="fixed inset-0 z-50 overflow-y-auto" use:portal>
    <div class="flex min-h-full items-end justify-center p-0 sm:items-center sm:p-4">
      <button type="button" aria-label="Cerrar" class="fixed inset-0 cursor-default bg-black/55 backdrop-blur-sm" onclick={onclose}></button>
      <div
        bind:this={dialog}
        role="dialog"
        aria-modal="true"
        aria-labelledby="{uid}-title"
        tabindex="-1"
        class="page-in relative flex max-h-[92dvh] w-full flex-col rounded-t-[28px] border border-app-ink/10 bg-app-panel text-app-ink shadow-app outline-none sm:rounded-[28px] {wide ? 'max-w-4xl' : 'max-w-xl'}"
      >
        <div class="flex items-start justify-between gap-4 px-6 pb-2 pt-6">
          <h2 id="{uid}-title" class="display text-[1.75rem] leading-tight">{title}</h2>
          <button type="button" class="icon-btn -mr-2 -mt-1" aria-label="Cerrar" onclick={onclose}><Icon name="x" size={20} /></button>
        </div>
        <div class="min-h-0 flex-1 overflow-y-auto px-6 py-4">{@render children()}</div>
        {#if footer}
          <div class="flex flex-wrap justify-end gap-2 border-t border-app-ink/10 px-6 py-4">{@render footer()}</div>
        {/if}
      </div>
    </div>
  </div>
{/if}
