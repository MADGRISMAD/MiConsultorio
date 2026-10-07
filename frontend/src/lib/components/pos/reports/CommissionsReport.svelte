<script lang="ts">
  import { pos2 } from '$lib/api/pos2';
  import { moneyCents } from '$lib/format';
  import { session } from '$lib/session.svelte';
  import type { CommissionReport, Professional } from '$lib/types/pos2';
  import Icon from '$lib/components/ui/Icon.svelte';

  interface Props {
    from: string;
    to: string;
  }
  let { from, to }: Props = $props();

  const uid = $props.id();
  let pros = $state<Professional[]>([]);
  let professional = $state('');
  let report = $state<CommissionReport | null>(null);
  let loading = $state(true);
  let error = $state('');
  let showLines = $state(false);
  let seq = 0;

  $effect(() => {
    pos2
      .professionals()
      .then((r) => (pros = r))
      .catch(() => {
        /* filter stays hidden */
      });
  });

  $effect(() => {
    const f = from;
    const t = to;
    const p = professional;
    if (!f || !t || f > t) return;
    const my = ++seq;
    loading = true;
    pos2
      .commissions(f, t, p)
      .then(
        (r) => {
          if (my === seq) (report = r), (error = '');
        },
        (e) => {
          if (my === seq) error = e instanceof Error ? e.message : 'No se pudo cargar el reporte de comisiones.';
        }
      )
      .finally(() => {
        if (my === seq) loading = false;
      });
  });

  const csvUrl = $derived(pos2.commissionsCsvUrl(from, to, professional));
  const pct = (n: number) => `${n.toLocaleString('es-MX', { maximumFractionDigits: 2 })} %`;
</script>

<section class="card p-5" aria-labelledby="{uid}-t">
  <div class="flex flex-wrap items-center justify-between gap-3">
    <h2 id="{uid}-t" class="display text-2xl">Comisiones</h2>
    <div class="flex flex-wrap items-center gap-2">
      <label class="sr-only" for="{uid}-p">Profesional</label>
      <select id="{uid}-p" class="field !min-h-9 !w-auto !py-1" bind:value={professional}>
        <option value="">Todos los profesionales</option>
        {#each pros as p (p.id)}<option value={p.id}>{p.name}</option>{/each}
        <option value="none">Sin profesional</option>
      </select>
      <a href={csvUrl} download class="btn-secondary min-h-9"><Icon name="download" size={16} />CSV</a>
      {#if session.has('posManage')}<a href="/pos/comisiones" class="btn-ghost min-h-9">Reglas<Icon name="arrow-right" size={16} /></a>{/if}
    </div>
  </div>
  {#if error}
    <p class="alert mt-3" role="alert"><Icon name="alert" size={18} />{error}</p>
  {:else if loading && !report}
    <p class="mt-3 text-sm text-app-muted" role="status">Cargando…</p>
  {:else if report}
    {#if !report.totals.length}
      <p class="mt-3 text-sm text-app-muted">Sin comisiones en este periodo. {#if session.has('posManage')}<a class="underline" href="/pos/comisiones">Define las reglas</a> y elige al profesional al cobrar.{/if}</p>
    {:else}
      <div class="mt-3 overflow-x-auto">
        <table class="w-full min-w-[420px]">
          <thead><tr><th class="th pl-0">Profesional</th><th class="th text-right">Conceptos</th><th class="th text-right">Base sin IVA</th><th class="th pr-0 text-right">Comisión</th></tr></thead>
          <tbody class="divide-y divide-app-ink/10">
            {#each report.totals as t (t.professional_id ?? 'none')}
              <tr><td class="td pl-0">{t.professional}</td><td class="td text-right">{t.lines}</td><td class="td text-right">{moneyCents(t.base_cents)}</td><td class="td pr-0 text-right font-medium">{moneyCents(t.commission_cents)}</td></tr>
            {/each}
          </tbody>
          <tfoot><tr><td class="td pl-0 font-medium" colspan="3">Total</td><td class="td pr-0 text-right font-medium" data-testid="commission-total">{moneyCents(report.commission_cents)}</td></tr></tfoot>
        </table>
      </div>
      <button type="button" class="btn-ghost mt-2 min-h-9" aria-expanded={showLines} onclick={() => (showLines = !showLines)}>{showLines ? 'Ocultar' : 'Ver'} el detalle ({report.lines.length})</button>
      {#if showLines}
        <div class="mt-2 overflow-x-auto">
          <table class="w-full min-w-[560px] text-sm">
            <thead><tr><th class="th pl-0">Venta</th><th class="th">Profesional</th><th class="th">Concepto</th><th class="th text-right">Base</th><th class="th text-right">%</th><th class="th pr-0 text-right">Comisión</th></tr></thead>
            <tbody class="divide-y divide-app-ink/10">
              {#each report.lines as l, n (n)}
                <tr><td class="td pl-0">#{l.folio}</td><td class="td">{l.professional}</td><td class="td">{l.item}</td><td class="td text-right">{moneyCents(l.base_cents)}</td><td class="td text-right">{pct(l.percent)}</td><td class="td pr-0 text-right">{moneyCents(l.commission_cents)}</td></tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    {/if}
  {/if}
</section>
