<script lang="ts">
  import { insurerByName } from '$lib/specialty/insurers';

  /**
   * An insurer as a logo plus its name. The logo is a file in /seguros/<id>.svg when the project has one; otherwise a
   * badge with the initials, so a missing file never shows a broken image.
   */
  let { name, size = 'md' }: { name: string; size?: 'sm' | 'md' } = $props();

  const known = $derived(insurerByName(name));
  let broken = $state(false);
  $effect(() => {
    name;
    broken = false;
  });
  const box = $derived(size === 'sm' ? 'h-8 w-8 text-[10px]' : 'h-11 w-11 text-xs');
  const initials = $derived((known?.mark ?? name.slice(0, 2)).toUpperCase());
</script>

<span class="inline-flex items-center gap-2.5">
  {#if known && !broken}
    <img src="/seguros/{known.id}.svg" alt="" class="{box} flex-none rounded-lg bg-white object-contain p-1 ring-1 ring-black/10" onerror={() => (broken = true)} />
  {:else}
    <span class="{box} grid flex-none place-items-center rounded-lg font-bold text-white" style="background:{known?.color ?? '#475569'}" aria-hidden="true">{initials}</span>
  {/if}
  <span class="font-medium">{known?.name ?? name}</span>
</span>
