<script lang="ts">
  import { page } from '$app/state';
  import Directory from '$lib/components/directory/Directory.svelte';
</script>

{#key page.params.area}<Directory areaSlug={page.params.area} />{/key}
