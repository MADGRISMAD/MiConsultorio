<script lang="ts">
  import { page } from '$app/state';
  import { session } from '$lib/session.svelte';

  export interface Tab {
    label: string;
    href: string;
    /** any one of these permissions shows the tab; omitted: everyone who got here */
    perms?: string[];
    /** other paths that mark the tab as current */
    also?: string[];
  }
  let { tabs, label = 'Secciones' }: { tabs: Tab[]; label?: string } = $props();

  const shown = $derived(tabs.filter((t) => !t.perms || t.perms.some((p) => session.has(p))));
  const current = (t: Tab) => [t.href, ...(t.also ?? [])].some((h) => page.url.pathname === h || page.url.pathname.startsWith(h + '/'));
</script>

{#if shown.length > 1}
  <nav class="mb-6 flex gap-1 overflow-x-auto rounded-full bg-app-ink/5 p-1 sm:inline-flex" aria-label={label}>
    {#each shown as t (t.href)}
      <a href={t.href} aria-current={current(t) ? 'page' : undefined}
        class="whitespace-nowrap rounded-full px-4 py-1.5 text-sm font-medium transition {current(t) ? 'bg-app-panel text-app-ink shadow-sm' : 'text-app-muted hover:text-app-ink'}">{t.label}</a>
    {/each}
  </nav>
{/if}
