<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/state';

  // the old address: /arco/<name> became /<name>/arco
  $effect(() => {
    void goto(`/${page.params.slug}/arco`, { replaceState: true });
  });
</script>
