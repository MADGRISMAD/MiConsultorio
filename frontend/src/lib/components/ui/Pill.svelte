<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    tone?: 'info' | 'ok' | 'warn' | 'bad' | 'muted';
    children: Snippet;
  }
  let { tone = 'muted', children }: Props = $props();
</script>

<span class="pill {tone === 'muted' ? '' : `pill-${tone}`}">{@render children()}</span>
