<script lang="ts">
  import { page } from '$app/state';
  import Directory from '$lib/components/directory/Directory.svelte';
</script>

{#key page.params.area + (page.params.city ?? "")}<Directory areaSlug={page.params.area} citySlug={page.params.city} />{/key}
