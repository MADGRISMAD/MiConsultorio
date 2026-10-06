<script lang="ts">
  let { size = 40 }: { size?: number } = $props();
</script>

<svg width={size} height={size} viewBox="0 0 40 40" aria-hidden="true">
  <defs>
    <linearGradient id="bm-g" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0" stop-color="#3b82d6" />
      <stop offset="1" stop-color="#1e5aa8" />
    </linearGradient>
  </defs>
  <rect width="40" height="40" rx="11" fill="url(#bm-g)" />
  <path d="M20 11v18M11 20h18" stroke="#fff" stroke-width="4" stroke-linecap="round" />
  <circle cx="30" cy="10" r="3.2" fill="#f0a94a" />
</svg>
