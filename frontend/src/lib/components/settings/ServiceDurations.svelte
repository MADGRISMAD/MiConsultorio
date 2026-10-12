<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { agendaLoadApi } from '$lib/api/waitlist';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import { VIDEO_AREAS } from '$lib/types';
  import type { ServiceDuration } from '$lib/types/waitlist';
  import Icon from '../ui/Icon.svelte';
  import LoadingRows from '../ui/LoadingRows.svelte';

  const uid = $props.id();
  type Row = ServiceDuration & { text: string; mode: ServiceDuration['modality']; dep: string };
  let rows = $state<Row[]>([]);
  // a video call is offered only in the giros where it is prudent
  const videoOk = $derived([session.clinic?.kind, ...(session.clinic?.specialties ?? [])].some((k) => !!k && VIDEO_AREAS.includes(k)));
  $effect(() => {
    if (session.status === 'authenticated') void session.loadClinic();
  });
  let saving = $state('');
  const load = new Op();
  const op = new Op();

  onMount(() =>
    void load.run(async () => {
      rows = (await agendaLoadApi.services()).map((s) => ({ ...s, text: s.duration_minutes ? String(s.duration_minutes) : '', mode: s.modality ?? 'presencial', dep: s.deposit_percent == null ? '' : String(s.deposit_percent) }));
    })
  );

  const dirty = (r: Row) => r.text.trim() !== (r.duration_minutes ? String(r.duration_minutes) : '') || r.mode !== (r.modality ?? 'presencial') || r.dep.trim() !== (r.deposit_percent == null ? '' : String(r.deposit_percent));
  const percent = (r: Row) => (r.dep.trim() === '' ? null : Number(r.dep));

  async function save(r: Row) {
    const t = r.text.trim();
    const n = t === '' ? null : Number(t);
    if (n !== null && !(Number.isInteger(n) && n >= 5 && n <= 480)) return op.fail('La duración debe ser un número de minutos entre 5 y 480.');
    const dp = percent(r);
    if (dp !== null && !(Number.isInteger(dp) && dp >= 0 && dp <= 100)) return op.fail('El anticipo debe ser un porcentaje entre 0 y 100.');
    saving = r.id;
    const ok = await op.run(async () => {
      if (r.text.trim() !== (r.duration_minutes ? String(r.duration_minutes) : '')) await agendaLoadApi.setDuration(r.id, n);
      if (r.mode !== (r.modality ?? 'presencial') || dp !== (r.deposit_percent ?? null)) await agendaLoadApi.setBooking(r.id, r.mode, dp);
    });
    saving = '';
    if (ok) {
      r.duration_minutes = n;
      r.modality = r.mode;
      r.deposit_percent = dp;
      toast.show(`«${r.name}» guardado`);
    }
  }
</script>

<section class="card px-5 py-5 sm:px-6" aria-labelledby="{uid}-t">
  <h2 id="{uid}-t" class="font-semibold">Servicios en la agenda y en la reserva en línea</h2>
  <p class="mt-1 text-sm text-app-muted">Cuánto tiempo reserva cada servicio (si lo dejas vacío se usa el intervalo del profesional){#if videoOk} y si se da en persona, por videollamada o de las dos formas. La videollamada solo la dan los profesionales de psicología, nutrición, medicina general o interna, dermatología y pediatría, y nunca a mascotas{/if}.</p>
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
          <div class="flex flex-wrap items-center gap-2">
            {#if videoOk}
              <select class="field w-auto" bind:value={r.mode} aria-label="Modalidad de {r.name}">
                <option value="presencial">En persona</option>
                <option value="videollamada">Videollamada</option>
                <option value="ambas">En persona o video</option>
              </select>
            {/if}
            {#if session.cobros && (r.price_cents ?? 0) > 0}
              <input class="field w-20" inputmode="numeric" maxlength="3" placeholder="Anticipo" bind:value={r.dep} aria-label="Anticipo en % de {r.name}; vacío usa el de la agenda" />
              <span class="text-sm text-app-muted">%</span>
            {/if}
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
