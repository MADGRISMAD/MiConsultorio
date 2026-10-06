<script lang="ts">
  interface Props {
    name: 'check' | 'arrow' | 'warn';
    class?: string;
    strokeWidth?: number;
  }
  let { name, class: cls = 'h-5 w-5', strokeWidth = 1.75 }: Props = $props();
</script>

<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width={strokeWidth} stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" class={cls}>
  {#if name === 'check'}
    <path d="M20 6 9 17l-5-5" />
  {:else if name === 'arrow'}
    <path d="M5 12h14M13 6l6 6-6 6" />
  {:else}
    <path d="M12 9v4M12 17h.01M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z" />
  {/if}
</svg>
