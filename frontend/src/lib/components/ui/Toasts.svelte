<script lang="ts">
  import { toast } from '$lib/toast.svelte';
  import Icon from './Icon.svelte';
</script>

<div class="pointer-events-none fixed inset-x-0 bottom-4 z-[60] flex flex-col items-center gap-2 px-4 sm:items-end sm:px-6" aria-live="polite">
  {#each toast.items as t (t.id)}
    <div
      role={t.kind === 'error' ? 'alert' : 'status'}
      class="page-in pointer-events-auto flex max-w-sm items-center gap-3 rounded-xl border border-app-ink/10 bg-app-panel px-4 py-3 text-sm font-semibold shadow-app"
    >
      <span class="grid h-6 w-6 flex-none place-items-center rounded-full {t.kind === 'success' ? 'bg-app-success/20 text-app-success' : 'bg-app-danger/20 text-app-danger'}">
        <Icon name={t.kind === 'success' ? 'check' : 'alert'} size={14} stroke={2.6} />
      </span>
      {t.message}
      <button type="button" class="icon-btn -mr-2 h-7 w-7" aria-label="Cerrar aviso" onclick={() => toast.dismiss(t.id)}><Icon name="x" size={14} /></button>
    </div>
  {/each}
</div>
