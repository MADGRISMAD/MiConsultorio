<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { agendaLoadApi } from '$lib/api/waitlist';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { ServiceDuration } from '$lib/types/waitlist';
  import Icon from '../ui/Icon.svelte';
  import LoadingRows from '../ui/LoadingRows.svelte';

  const uid = $props.id();
  let rows = $state<(ServiceDuration & { text: string })[]>([]);
  let saving = $state('');
  const load = new Op();
  const op = new Op();

  onMount(() =>
    void load.run(async () => {
      rows = (await agendaLoadApi.services()).map((s) => ({ ...s, text: s.duration_minutes ? String(s.duration_minutes) : '' }));
    })
  );

  const dirty = (r: ServiceDuration & { text: string }) => r.text.trim() !== (r.duration_minutes ? String(r.duration_minutes) : '');

  async function save(r: ServiceDuration & { text: string }) {
    const t = r.text.trim();
    const n = t === '' ? null : Number(t);
    if (n !== null && !(Number.isInteger(n) && n >= 5 && n <= 480)) return op.fail('La duración debe ser un número de minutos entre 5 y 480.');
    saving = r.id;
    const ok = await op.run(() => agendaLoadApi.setDuration(r.id, n));
    saving = '';
    if (ok) {
      r.duration_minutes = n;
      toast.show(`Duración de «${r.name}» guardada`);
    }
  }
</script>

<section class="card px-5 py-5 sm:px-6" aria-labelledby="{uid}-t">
  <h2 id="{uid}-t" class="font-semibold">Duración de los servicios</h2>
  <p class="mt-1 text-sm text-app-muted">Cuánto tiempo reserva cada servicio en la agenda y en la reserva en línea. Si lo dejas vacío se usa el intervalo del profesional.</p>
  <OpError op={op} class="mt-4" />
  {#if load.phase === 'loading'}
    <LoadingRows />
  {:else if load.phase === 'error'}
    <Alert class="mt-4">{load.message}</Alert>
  {:else if rows.length === 0}
    <p class="mt-4 text-sm text-app-muted">Aún no hay servicios en el catálogo.</p>
  {:else}
    <ul class="mt-4 divide-y divide-app-ink/8">
      {#each rows as r (r.id)}
        <li class="flex flex-wrap items-center justify-between gap-3 py-3">
          <label class="min-w-0 flex-1 text-sm font-medium [overflow-wrap:anywhere]" for="{uid}-{r.id}">{r.name}</label>
          <div class="flex items-center gap-2">
            <input id="{uid}-{r.id}" class="field w-24" inputmode="numeric" maxlength="3" placeholder="—" bind:value={r.text} aria-describedby="{uid}-u" />
            <span id="{uid}-u" class="text-sm text-app-muted">min</span>
            <button type="button" class="btn-secondary" disabled={!dirty(r) || saving === r.id} onclick={() => save(r)} aria-label="Guardar la duración de {r.name}">
              {#if saving === r.id}<span class="spin"></span>{/if}Guardar
            </button>
          </div>
        </li>
      {/each}
    </ul>
  {/if}
</section>
