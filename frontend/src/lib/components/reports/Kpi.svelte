<script lang="ts">
  /** One indicator with its change against the previous period. `lowerIsBetter` flips the color of the change. */
  interface Props {
    label: string;
    value: string;
    now: number | null;
    before: number | null;
    /** the change is shown as points (for rates) or as a percentage */
    unit?: 'pct' | 'pts';
    lowerIsBetter?: boolean;
    hint?: string;
  }
  let { label, value, now, before, unit = 'pct', lowerIsBetter = false, hint = '' }: Props = $props();

  const delta = $derived.by(() => {
    if (now == null || before == null) return null;
    if (unit === 'pts') return { text: `${Math.abs(now - before).toFixed(1)} pts`, diff: now - before };
    if (before === 0) return now === 0 ? { text: '0%', diff: 0 } : { text: 'nuevo', diff: 1 };
    const d = ((now - before) / before) * 100;
    return { text: `${Math.abs(d).toFixed(0)}%`, diff: d };
  });
  const tone = $derived(!delta || Math.abs(delta.diff) < 0.05 ? 'text-app-muted' : (delta.diff > 0) !== lowerIsBetter ? 'text-app-success' : 'text-app-danger');
  const arrow = $derived(!delta || Math.abs(delta.diff) < 0.05 ? '→' : delta.diff > 0 ? '↑' : '↓');
</script>

<div class="card p-4">
  <p class="text-xs text-app-muted">{label}</p>
  <p class="display mt-1 text-3xl">{value}</p>
  {#if delta}<p class="mt-1 text-xs {tone}"><span aria-hidden="true">{arrow}</span> {delta.text} <span class="text-app-muted">vs. periodo anterior</span></p>{/if}
  {#if hint}<p class="mt-1 text-xs text-app-muted">{hint}</p>{/if}
</div>
