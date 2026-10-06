<script lang="ts">
  import { initials } from '$lib/format';

  let { name, size = 40 }: { name: string; size?: number } = $props();
  // a stable tint per person, from the landing palette
  const tone = $derived(['bg-app-primary/12 text-app-primary', 'bg-app-accent/14 text-app-accent', 'bg-app-ink/8 text-app-ink'][[...name].reduce((a, c) => a + c.charCodeAt(0), 0) % 3]);
</script>

<span class="grid flex-none place-items-center rounded-full font-display {tone}" style="width:{size}px;height:{size}px;font-size:{size * 0.42}px" aria-hidden="true">{initials(name)}</span>
