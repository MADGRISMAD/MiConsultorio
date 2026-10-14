<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import PlatformBlog from '$lib/components/platform/PlatformBlog.svelte';
</script>

<svelte:head><title>Blog · Caresia</title></svelte:head>

<Guard title="Blog" roles={['platform_admin', 'platform_support']}>
  <PlatformBlog />
</Guard>
