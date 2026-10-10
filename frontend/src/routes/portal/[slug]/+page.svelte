<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/state';

  // the old address: /portal/<name> became /<name>/portal
  $effect(() => {
    void goto(`/${page.params.slug}/portal`, { replaceState: true });
  });
</script>
