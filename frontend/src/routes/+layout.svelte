<script lang="ts">
  import '../app.css';
  import { onMount } from 'svelte';
  import { session } from '$lib/session.svelte';

  let { children } = $props();

  onMount(() => session.load());
</script>

<svelte:head>
  <title>Caresia · Software para clínicas dentales y consultorios médicos</title>
</svelte:head>

{@render children()}
