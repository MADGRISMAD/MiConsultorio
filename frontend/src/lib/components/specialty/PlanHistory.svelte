<script lang="ts">
  import { dateTime as dt } from '$lib/format';
  import { onMount } from 'svelte';
  import { specialtyApi } from '$lib/api/specialty';
  import type { PlanEvent } from '$lib/types/specialty';

  let { id }: { id: string } = $props();
  let events = $state<PlanEvent[] | null>(null);
  let failed = $state(false);

  const label: Record<string, string> = {
    created: 'Plan creado',
    edited: 'Plan editado',
    proposed: 'Propuesto al paciente',
    accepted: 'Aceptado con firma',
    items_added: 'Conceptos agregados',
    item_done: 'Concepto realizado',
    item_cancel: 'Concepto cancelado',
    cancelled: 'Plan cancelado'
  };

  onMount(async () => {
    try {
      events = (await specialtyApi.plan(id)).events ?? [];
    } catch {
      failed = true;
    }
  });
</script>

{#if failed}
  <p class="mt-2 text-sm text-app-danger">No se pudo cargar el historial.</p>
{:else if events === null}
  <p class="mt-2 text-sm text-app-muted">Cargando…</p>
{:else}
  <ol class="mt-2 space-y-1 text-sm">
    {#each events as ev}
      <li><span class="text-app-muted">{dt(ev.created_at)} · v{ev.version} · {ev.actor_name}:</span> {label[ev.action] ?? ev.action}{ev.detail ? ` (${ev.detail})` : ''}</li>
    {/each}
  </ol>
{/if}
