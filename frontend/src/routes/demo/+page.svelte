<script lang="ts">
  import { goto } from '$app/navigation';

  // the demo form no longer exists: the old links lead to the sign-up
  $effect(() => {
    void goto('/register', { replaceState: true });
  });
</script>

<svelte:head><title>Probar Caresia gratis</title><meta name="robots" content="noindex" /></svelte:head>
