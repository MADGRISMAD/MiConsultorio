<script lang="ts">
  import { rxApi } from '$lib/api/rx';
  import { Op } from '$lib/op.svelte';
  import { dateTime } from '$lib/format';
  import type { ConsultSummaryResult } from '$lib/types/rx';
  import { session } from '$lib/session.svelte';
  import OpError from '../../ui/OpError.svelte';
  import Icon from '../../ui/Icon.svelte';

  let { patientId, hasNotes }: { patientId: string; hasNotes: boolean } = $props();

  let result = $state<ConsultSummaryResult | null>(null);
  const op = new Op();
  const run = async () => {
    await op.run(async () => {
      result = await rxApi.summary(patientId);
    });
  };
  const sections = $derived(
    result
      ? [
          { title: 'Datos clave', items: result.summary.key_facts },
          { title: 'Pendientes', items: result.summary.pending },
          { title: 'A vigilar', items: result.summary.watch_for }
        ].filter((s) => s.items.length)
      : []
  );
</script>

{#if hasNotes && session.magic}
  <section class="card p-5 sm:p-6" aria-labelledby="ai-sum-h">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 id="ai-sum-h" class="display text-2xl">Resumen para la consulta</h2>
        <p class="mt-1 max-w-xl text-sm text-app-muted">
          La IA lee las últimas consultas, las alergias y la medicación, y te prepara un resumen. No ve el nombre del paciente ni tus notas privadas. Cada resumen usa 1 uso de magia.
        </p>
      </div>
      <button type="button" class="btn-secondary" disabled={op.phase === 'loading'} onclick={run}>
        {#if op.phase === 'loading'}<span class="spin"></span>Leyendo el expediente…{:else}<Icon name="sparkles" size={18} />{result ? 'Actualizar resumen' : 'Resumir expediente'}{/if}
      </button>
    </div>
    <OpError {op} class="mt-3" />
    {#if result}
      <div class="mt-4 space-y-4">
        <p class="text-[15px] leading-relaxed">{result.summary.overview}</p>
        {#each sections as s (s.title)}
          <div>
            <p class="section-title mb-1">{s.title}</p>
            <ul class="list-disc space-y-0.5 pl-5 text-sm">{#each s.items as it}<li>{it}</li>{/each}</ul>
          </div>
        {/each}
        <p class="text-xs text-app-muted">Generado {dateTime(result.generated_at)} con las últimas {result.based_on} consulta{result.based_on === 1 ? '' : 's'}. Es una ayuda de lectura: confirma siempre con el expediente.</p>
      </div>
    {/if}
  </section>
{/if}
