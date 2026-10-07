<script lang="ts">
  import type { AccessEntry } from '$lib/types';
  import EmptyState from '../../ui/EmptyState.svelte';
  let { access }: { access: AccessEntry[] } = $props();
  const ACTION: Record<string, string> = { view: 'Abrió el expediente', print: 'Imprimió', export: 'Exportó' };
  const dt = (iso: string) => new Date(iso).toLocaleString('es-MX', { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false });
</script>

<section class="card overflow-hidden">
  {#if access.length === 0}
    <EmptyState icon="shield" title="Sin accesos registrados" />
  {:else}
    <p class="px-5 pt-4 text-sm text-app-muted">Cada apertura o impresión del expediente queda registrada.</p>
    <ul class="divide-y divide-app-ink/8">
      {#each access as a}
        <li class="flex flex-wrap items-center justify-between gap-2 px-5 py-3 text-sm">
          <span><strong>{a.user}</strong> <span class="text-app-muted">· {ACTION[a.action] ?? a.action}</span></span>
          <time class="text-app-muted" datetime={a.at}>{dt(a.at)}</time>
        </li>
      {/each}
    </ul>
  {/if}
</section>
