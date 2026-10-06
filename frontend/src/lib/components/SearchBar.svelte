<script lang="ts">
  let { onsearch, onclear }: { onsearch: (text: string) => void; onclear: () => void } = $props();
  let value = $state('');

  function submit(e: SubmitEvent) {
    e.preventDefault();
    const text = value.trim();
    if (text) onsearch(text);
    else onclear();
  }
</script>

<form class="mx-auto flex max-w-md items-center gap-2 rounded-lg bg-white px-4 shadow-sm ring-1 ring-ink/10 focus-within:ring-2 focus-within:ring-signal" role="search" onsubmit={submit}>
  <img src="/search.png" alt="" class="h-5 w-5 opacity-50" />
  <input
    type="search"
    class="h-12 w-full bg-transparent text-sm outline-none"
    placeholder="Escribe una CURP y presiona Enter..."
    aria-label="Buscar por CURP"
    bind:value
  />
</form>
