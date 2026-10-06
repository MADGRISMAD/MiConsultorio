<script lang="ts">
  interface Props {
    /** force paper-on-ink colours (for the always-dark brand panel) */
    onDark?: boolean;
    size?: number;
    class?: string;
  }
  let { onDark = false, size = 30, class: cls = '' }: Props = $props();
</script>

<!-- Mark + serif wordmark, as in the landing -->
<span class="inline-flex items-center gap-2.5 {onDark ? 'text-[#F4F8FB]' : 'text-app-ink'} {cls}">
  <svg width={size} height={size} viewBox="0 0 32 32" aria-hidden="true">
    <rect width="32" height="32" rx="9" fill="currentColor" />
    <path d="M16 8v16M8 16h16" stroke={onDark ? '#0B2540' : 'rgb(var(--app-surface))'} stroke-width="3.2" stroke-linecap="round" />
    <circle cx="23.5" cy="8.5" r="2.5" fill="#1673D1" />
  </svg>
  <span class="font-display text-[1.65em] leading-none tracking-[-0.01em]">Caresia</span>
</span>
