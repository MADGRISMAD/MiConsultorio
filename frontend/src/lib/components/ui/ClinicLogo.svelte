<script lang="ts">
  /** The clinic's own logo or photo; without one, the Caresia logo. */
  let { src = '', size = 48, name = '' }: { src?: string; size?: number; name?: string } = $props();
</script>

{#if src}
  <img {src} alt={name ? `Logo de ${name}` : 'Logo del consultorio'} width={size} height={size} class="flex-none rounded-xl object-contain" style="width:{size}px;height:{size}px" />
{:else}
  <svg width={size} height={size} viewBox="0 0 32 32" role="img" aria-label="Logo de Caresia" class="flex-none">
    <rect width="32" height="32" rx="9" fill="#0B2540" />
    <path d="M16 8v16M8 16h16" stroke="#fff" stroke-width="3.2" stroke-linecap="round" />
    <circle cx="23.5" cy="8.5" r="2.5" fill="#1673D1" />
  </svg>
{/if}
