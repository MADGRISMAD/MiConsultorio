<script lang="ts">
  // Horizontal bars with a label and a value on each row.
  interface Row {
    label: string;
    value: number;
    text?: string;
    sub?: string;
  }
  interface Props {
    rows: Row[];
    tone?: 'primary' | 'warning';
  }
  let { rows, tone = 'primary' }: Props = $props();
  const max = $derived(Math.max(1, ...rows.map((r) => r.value)));
</script>

<ul class="grid gap-3">
  {#each rows as r, i (i)}
    <li>
      <div class="flex justify-between gap-3 text-sm"><span class="min-w-0 truncate">{r.label}{#if r.sub} <span class="text-app-muted">· {r.sub}</span>{/if}</span><span class="shrink-0 font-medium">{r.text ?? r.value}</span></div>
      <div class="mt-1.5 h-2 overflow-hidden rounded-full bg-app-ink/8"><div class="h-full rounded-full {tone === 'warning' ? 'bg-app-warning' : 'bg-app-accent'}" style="width: {(r.value / max) * 100}%"></div></div>
    </li>
  {/each}
</ul>
