<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/state';

  // the old address: /reservar/<name> became /<name>/reservar
  $effect(() => {
    void goto(`/${page.params.slug}/reservar`, { replaceState: true });
  });
</script>
