<script lang="ts">
  import { goto } from '$app/navigation';

  // Settings live in one place now.
  $effect(() => {
    void goto('/ajustes?s=negocio', { replaceState: true });
  });
</script>
