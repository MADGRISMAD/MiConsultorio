<script lang="ts">
  import Icon from './ui/Icon.svelte';

  let { onsearch, onclear }: { onsearch: (text: string) => void; onclear: () => void } = $props();
  let value = $state('');

  function submit(e: SubmitEvent) {
    e.preventDefault();
    const text = value.trim();
    if (text) onsearch(text);
    else onclear();
  }
</script>

<form class="relative w-full max-w-md" role="search" onsubmit={submit}>
  <Icon name="search" size={18} class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-app-muted" />
  <input type="search" class="field pl-10" placeholder="Buscar por CURP y presionar Enter…" aria-label="Buscar por CURP" bind:value />
</form>
