<script lang="ts">
  import { clinicAiApi, type AtRiskPatient, type PeriodRange } from '$lib/api/clinicAi';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { dateShort } from '$lib/format';
  import type { AiResult } from '$lib/types/specialty';
  import AiResultView from '../patients/detail/AiResultView.svelte';
  import Icon from '../ui/Icon.svelte';
  import OpError from '../ui/OpError.svelte';

  let range = $state<PeriodRange>('semana');
  let summary = $state<AiResult | null>(null);
  let risk = $state<AiResult | null>(null);
  let riskList = $state<AtRiskPatient[]>([]);
  const sumOp = new Op();
  const riskOp = new Op();

  const runSummary = () =>
    sumOp.run(async () => {
      summary = await clinicAiApi.periodSummary(range);
    });
  const runRisk = () =>
    riskOp.run(async () => {
      const r = await clinicAiApi.atRisk();
      risk = r;
      riskList = r.result.extra ?? [];
    });
</script>

{#if session.magic}
  <section class="card mt-6 p-5 sm:p-6" aria-labelledby="ind-ai-h">
    <h2 id="ind-ai-h" class="display text-2xl">Resumen con IA</h2>
    <p class="mt-1 max-w-xl text-sm text-app-muted">La IA lee las cifras de tu consultorio (no ve nombres de pacientes) y te dice qué va bien y qué revisar. Es una sugerencia: tú decides. Cada resumen usa 1 uso de magia.</p>
    <div class="mt-3 flex flex-wrap items-end gap-3">
      <label class="block"><span class="label">Periodo</span>
        <select class="field" bind:value={range}>
          <option value="hoy">Hoy</option>
          <option value="semana">Últimos 7 días</option>
          <option value="mes">Últimos 30 días</option>
        </select>
      </label>
      <button type="button" class="btn-secondary" disabled={sumOp.phase === 'loading'} onclick={runSummary}>
        {#if sumOp.phase === 'loading'}<span class="spin"></span>Analizando…{:else}<Icon name="sparkles" size={18} />{summary ? 'Actualizar resumen' : 'Resumir el periodo'}{/if}
      </button>
    </div>
    <OpError op={sumOp} class="mt-3" />
    {#if summary}<div class="mt-4"><AiResultView result={summary} /></div>{/if}
  </section>

  <section class="card mt-6 p-5 sm:p-6" aria-labelledby="ind-risk-h">
    <h2 id="ind-risk-h" class="display text-2xl">Pacientes que dejaron de venir</h2>
    <p class="mt-1 max-w-xl text-sm text-app-muted">Quienes llevan más de 3 meses sin consulta y no tienen cita próxima. La IA propone mensajes para retomar el contacto, sin datos clínicos. Usa 1 uso de magia.</p>
    <button type="button" class="btn-secondary mt-3" disabled={riskOp.phase === 'loading'} onclick={runRisk}>
      {#if riskOp.phase === 'loading'}<span class="spin"></span>Buscando…{:else}<Icon name="sparkles" size={18} />{risk ? 'Actualizar' : 'Buscar y proponer mensajes'}{/if}
    </button>
    <OpError op={riskOp} class="mt-3" />
    {#if risk}
      <div class="mt-4 space-y-4">
        <AiResultView result={risk} />
        <div>
          <p class="section-title mb-1">Pacientes ({riskList.length})</p>
          <ul class="divide-y divide-app-ink/8 rounded-xl border border-app-ink/12 text-sm">
            {#each riskList as x (x.id)}
              <li class="flex flex-wrap items-center justify-between gap-2 px-3 py-2">
                <a class="font-medium underline-offset-2 hover:underline" href="/pacientes/{x.id}">{x.name}</a>
                <span class="text-app-muted">Última consulta {dateShort(x.last_visit)} · hace {x.days} días{x.reminders_ok ? '' : ' · sin permiso de recordatorios'}</span>
              </li>
            {/each}
          </ul>
        </div>
      </div>
    {/if}
  </section>
{/if}
