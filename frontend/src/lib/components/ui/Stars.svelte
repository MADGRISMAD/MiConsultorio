<script lang="ts">
  /** Five stars: read-only, or a radio group when `onpick` is given. */
  let { value = 0, onpick, size = 20, label = 'Calificación' }: { value?: number; onpick?: (n: number) => void; size?: number; label?: string } = $props();
  const STAR = 'M12 2.5l2.9 6.1 6.6.8-4.9 4.6 1.3 6.6L12 17.3 6.1 20.6l1.3-6.6L2.5 9.4l6.6-.8z';
</script>

{#if onpick}
  <div class="inline-flex gap-1" role="radiogroup" aria-label={label}>
    {#each [1, 2, 3, 4, 5] as n}
      <button type="button" role="radio" aria-checked={value === n} aria-label="{n} de 5" class="rounded p-0.5 transition hover:scale-110" onclick={() => onpick(n)}>
        <svg width={size} height={size} viewBox="0 0 24 24" class={n <= value ? 'text-amber-400' : 'text-app-ink/20'} fill="currentColor" aria-hidden="true"><path d={STAR} /></svg>
      </button>
    {/each}
  </div>
{:else}
  <span class="inline-flex gap-0.5" role="img" aria-label="{value} de 5">
    {#each [1, 2, 3, 4, 5] as n}
      <svg width={size} height={size} viewBox="0 0 24 24" class={n <= Math.round(value) ? 'text-amber-400' : 'text-app-ink/20'} fill="currentColor" aria-hidden="true"><path d={STAR} /></svg>
    {/each}
  </span>
{/if}
