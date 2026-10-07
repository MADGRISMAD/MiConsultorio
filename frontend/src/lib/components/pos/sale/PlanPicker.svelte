<script lang="ts">
  import Modal from '$lib/components/Modal.svelte';
  import { moneyCents } from '$lib/format';
  import type { PlanPrefill, PlanPrefillItem } from '$lib/types/pos2';

  interface Props {
    plan: PlanPrefill | null;
    onclose: () => void;
    onadd: (plan: PlanPrefill, items: PlanPrefillItem[]) => void;
  }
  let { plan, onclose, onadd }: Props = $props();

  const uid = $props.id();
  // what can still be charged: not cancelled and not linked to a sale yet
  const billable = $derived((plan?.items ?? []).filter((i) => i.status !== 'cancelled' && !i.sale_id));
  let picked = $state<Record<string, boolean>>({});

  $effect(() => {
    // pending items start selected; done ones only appear so they can be charged late
    picked = Object.fromEntries(billable.map((i) => [i.id, i.status === 'pending']));
  });

  const chosen = $derived(billable.filter((i) => picked[i.id]));
  const total = $derived(chosen.reduce((a, i) => a + Math.round(i.unit_price_cents * i.qty), 0));
  const phases = $derived([...new Set(billable.map((i) => i.phase))].sort((a, b) => a - b));
</script>

<Modal open={!!plan} title={plan ? `Plan: ${plan.title}` : ''} {onclose}>
  {#if plan}
    <p class="mb-3 text-sm text-app-muted">{plan.patient_name}. Elige los conceptos que se cobran ahora; el resto queda pendiente en el plan.</p>
    {#if !billable.length}
      <p class="rounded-xl bg-app-elevated px-4 py-6 text-center text-sm text-app-muted">Este plan no tiene conceptos por cobrar.</p>
    {:else}
      <div class="space-y-4">
        {#each phases as ph (ph)}
          <fieldset class="min-w-0">
            <legend class="section-title mb-1">Fase {ph}</legend>
            <ul class="divide-y divide-app-ink/10 rounded-xl ring-1 ring-inset ring-app-ink/10">
              {#each billable.filter((i) => i.phase === ph) as i (i.id)}
                <li>
                  <label class="flex min-h-12 cursor-pointer items-center gap-3 px-3 py-2">
                    <input type="checkbox" class="h-4 w-4 shrink-0" bind:checked={picked[i.id]} id="{uid}-{i.id}" />
                    <span class="min-w-0 flex-1 text-sm">
                      <span class="block truncate font-medium">{i.description}{i.tooth ? ` · diente ${i.tooth}` : ''}</span>
                      <span class="block text-xs text-app-muted">{i.qty !== 1 ? `${i.qty} × ` : ''}{moneyCents(i.unit_price_cents)}{i.status === 'done' ? ' · realizado' : ''}</span>
                    </span>
                    <span class="shrink-0 text-sm font-medium tabular-nums">{moneyCents(Math.round(i.unit_price_cents * i.qty))}</span>
                  </label>
                </li>
              {/each}
            </ul>
          </fieldset>
        {/each}
      </div>
    {/if}
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    <button type="button" class="btn-primary" disabled={!chosen.length} onclick={() => plan && onadd(plan, chosen)}>
      Agregar a la cuenta{chosen.length ? ` · ${moneyCents(total)}` : ''}
    </button>
  {/snippet}
</Modal>
