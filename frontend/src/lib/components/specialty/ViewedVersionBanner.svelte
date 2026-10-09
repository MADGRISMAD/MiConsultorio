<script lang="ts">
  import { dateTime } from '$lib/format';
  import type { PatientChart } from '$lib/types/specialty';

  /** Shown while an older saved version is open (read only), with the way back to the current one. */
  let { viewed, back, backLabel = 'Volver al borrador actual' }: { viewed: PatientChart; back: () => void; backLabel?: string } = $props();
</script>

<div class="mb-3 flex flex-wrap items-center justify-between gap-2 rounded-2xl bg-app-warning/12 px-4 py-3 text-sm" role="status">
  <span>Versión del {dateTime(viewed.created_at)} por {viewed.created_by_name}{viewed.note ? `: ${viewed.note}` : ''}. Solo lectura.</span>
  <button type="button" class="btn-secondary" onclick={back}>{backLabel}</button>
</div>
