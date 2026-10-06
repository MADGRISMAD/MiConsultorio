<script lang="ts">
  let { class: cls = 'h-7 w-7', light = false }: { class?: string; light?: boolean } = $props();
</script>

<svg viewBox="0 0 32 32" aria-hidden="true" class={cls}>
  <rect width="32" height="32" rx="9" class={light ? 'fill-paper' : 'fill-ink'} />
  <path d="M16 8v16M8 16h16" class={light ? 'stroke-ink' : 'stroke-paper'} stroke-width="3.2" stroke-linecap="round" />
  <circle cx="23.5" cy="8.5" r="2.5" class="fill-signal" />
</svg>
