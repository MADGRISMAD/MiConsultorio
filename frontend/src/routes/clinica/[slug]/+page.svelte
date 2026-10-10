<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/state';

  // the old address: /clinica/<name> became /<name>
  $effect(() => {
    void goto(`/${page.params.slug}`, { replaceState: true });
  });
</script>
