<script lang="ts">
  import type { Snippet } from 'svelte';
  import Icon from './Icon.svelte';

  let { children, class: cls = '' }: { children: Snippet; class?: string } = $props();
</script>

<p class="alert {cls}" role="alert"><Icon name="alert" size={18} />{@render children()}</p>
