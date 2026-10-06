<script lang="ts">
  import type { Snippet } from 'svelte';
  import Icon, { type IconName } from './Icon.svelte';

  interface Props {
    icon: IconName;
    title: string;
    text?: string;
    children?: Snippet;
  }
  let { icon, title, text, children }: Props = $props();
</script>

<div class="flex flex-col items-center px-6 py-14 text-center">
  <span class="grid h-14 w-14 place-items-center rounded-2xl bg-app-primary/10 text-app-primary"><Icon name={icon} size={26} /></span>
  <h3 class="display mt-4 text-2xl">{title}</h3>
  {#if text}<p class="mt-1 max-w-sm text-sm text-app-muted">{text}</p>{/if}
  {#if children}<div class="mt-5">{@render children()}</div>{/if}
</div>
