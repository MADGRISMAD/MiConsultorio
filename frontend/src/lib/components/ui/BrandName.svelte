<script lang="ts">
  let { dark = false }: { dark?: boolean } = $props();
</script>

<span class="font-extrabold tracking-tight"
  ><span class={dark ? 'text-[#f0a94a]' : 'text-app-accent'}>Care</span><span class={dark ? 'text-[#5b9ae8]' : 'text-app-primary'}>sia</span></span
>
