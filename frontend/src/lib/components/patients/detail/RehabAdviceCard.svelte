<script lang="ts">
  import { specialtyApi } from '$lib/api/specialty';
  import { Op } from '$lib/op.svelte';
  import { dateTime } from '$lib/format';
  import type { RehabAdviceResult } from '$lib/types/specialty';
  import { session } from '$lib/session.svelte';
  import OpError from '../../ui/OpError.svelte';
  import Icon from '../../ui/Icon.svelte';

  let { patientId, sessions }: { patientId: string; sessions: number } = $props();

  let result = $state<RehabAdviceResult | null>(null);
  const op = new Op();
  const run = () =>
    op.run(async () => {
      result = await specialtyApi.rehabAdviceAI(patientId);
    });
  const STANCE = {
    continuar: 'Podría considerarse continuar con el plan actual',
    ajustar: 'Podría considerarse revisar o ajustar el plan',
    valorar_alta: 'Podría considerarse valorar el alta o espaciar las sesiones'
  } as const;
</script>

{#if sessions >= 2 && session.magic}
  <section class="card p-5 sm:p-6" aria-labelledby="rehab-adv-h">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 id="rehab-adv-h" class="display text-2xl">Sugerencia de avance</h2>
        <p class="mt-1 max-w-xl text-sm text-app-muted">
          La IA revisa la evolución del dolor y la movilidad (calculada por el sistema) y ofrece una sugerencia sobre continuar, ajustar o valorar el alta. Usa 1 uso de magia.
        </p>
      </div>
      <button type="button" class="btn-secondary" disabled={op.phase === 'loading'} onclick={run}>
        {#if op.phase === 'loading'}<span class="spin"></span>Revisando la evolución…{:else}<Icon name="sparkles" size={18} />{result ? 'Actualizar' : 'Sugerir avance'}{/if}
      </button>
    </div>
    <p class="mt-3 rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-900" role="note"><strong>Solo es una sugerencia.</strong> La valoración clínica y la decisión son del especialista.</p>
    <OpError {op} class="mt-3" />
    {#if result}
      <div class="mt-4 space-y-4">
        <div class="rounded-xl border border-dashed border-app-ink/25 p-4">
          <p class="section-title">Sugerencia de la IA</p>
          <p class="mt-1 font-medium">{STANCE[result.suggestion.stance]}</p>
          <p class="mt-1 text-[15px] leading-relaxed">{result.suggestion.summary}</p>
          {#if result.suggestion.reasons.length}
            <ul class="mt-2 list-disc space-y-0.5 pl-5 text-sm">{#each result.suggestion.reasons as it}<li>{it}</li>{/each}</ul>
          {/if}
        </div>
        <div>
          <p class="section-title mb-1">Qué verificar en tu valoración</p>
          <ul class="list-disc space-y-0.5 pl-5 text-sm">{#each result.suggestion.verify as it}<li>{it}</li>{/each}</ul>
        </div>
        <details class="text-sm"><summary class="cursor-pointer text-app-muted">Cifras en las que se basa (calculadas por el sistema)</summary><p class="mt-1 whitespace-pre-line">{result.trend}</p></details>
        <p class="text-xs text-app-muted">Generado {dateTime(result.generated_at)} con las últimas {result.based_on} sesiones. {result.disclaimer}</p>
      </div>
    {/if}
  </section>
{/if}
