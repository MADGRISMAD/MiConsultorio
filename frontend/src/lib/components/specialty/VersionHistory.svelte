<script lang="ts">
  import { dateTime } from '$lib/format';
  import type { PatientChart } from '$lib/types/specialty';
  import EmptyState from '../ui/EmptyState.svelte';
  import Icon, { type IconName } from '../ui/Icon.svelte';

  /** The saved versions of a chart (odontogram, body map, nutrition plan…): see one, compare it with the previous one, or start from it. */
  interface Props {
    id: string;
    title: string;
    history: PatientChart[];
    viewing: string | null;
    canWrite: boolean;
    icon: IconName;
    emptyText: string;
    /** the line under the date: what that version holds */
    summary: (h: PatientChart) => string;
    onview: (h: PatientChart, compare?: boolean) => void;
    onbase: (h: PatientChart) => void;
    /** offer «Comparar con la anterior» */
    compare?: boolean;
    /** when set, every version but the newest carries this label (a plan is replaced by the next one: «Vencido») */
    olderLabel?: string;
  }
  let { id, title, history, viewing, canWrite, icon, emptyText, summary, onview, onbase, compare = false, olderLabel = '' }: Props = $props();
</script>

<h3 {id} class="display mb-2 text-xl">{title}</h3>
{#if history.length === 0}
  <div class="card"><EmptyState {icon} title="Sin versiones" text={emptyText} /></div>
{:else}
  <ul class="space-y-2">
    {#each history as h, i (h.id)}
      <li class="card flex flex-wrap items-center justify-between gap-2 p-3 sm:px-5 {viewing === h.id ? 'ring-2 ring-app-primary' : ''}">
        <div class="min-w-0">
          <p class="text-sm font-medium">{dateTime(h.created_at)}{#if i === 0}<span class="badge ml-2">Vigente</span>{:else if olderLabel}<span class="badge ml-2 !bg-app-warning/15 !text-app-warning">{olderLabel}</span>{/if}</p>
          <p class="truncate text-xs text-app-muted">{h.created_by_name}{summary(h) ? ` · ${summary(h)}` : ''}{h.note ? ` · ${h.note}` : ''}</p>
        </div>
        <div class="flex flex-wrap gap-1">
          <button type="button" class="btn-ghost" onclick={() => onview(h)}><Icon name="eye" size={16} />Ver</button>
          {#if compare}<button type="button" class="btn-ghost" onclick={() => onview(h, true)}><Icon name="rotate" size={16} />Comparar con la anterior</button>{/if}
          {#if canWrite && i > 0}<button type="button" class="btn-ghost" onclick={() => onbase(h)}>Usar como base</button>{/if}
        </div>
      </li>
    {/each}
  </ul>
{/if}
