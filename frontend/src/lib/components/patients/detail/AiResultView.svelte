<script lang="ts">
  import type { Snippet } from 'svelte';
  import { dateTime, moneyCents } from '$lib/format';
  import { toast } from '$lib/toast.svelte';
  import type { AiResult } from '$lib/types/specialty';
  import Icon from '../../ui/Icon.svelte';

  let { result, onprint, children }: { result: AiResult; onprint?: () => void; children?: Snippet } = $props();
  const r = $derived(result.result);

  async function copy() {
    const text = r.text || [r.title, r.summary, ...r.sections.flatMap((s) => [s.heading, ...s.items.map((i) => `• ${i}`)])].filter(Boolean).join('\n');
    try {
      await navigator.clipboard.writeText(text);
      toast.show('Copiado');
    } catch {
      toast.show('No se pudo copiar; selecciona el texto a mano');
    }
  }
</script>

<div class="space-y-4" aria-live="polite">
  <p class="rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-900" role="note"><strong>{result.draft ? 'Borrador.' : 'Solo es una sugerencia.'}</strong> {result.disclaimer}</p>
  {#if r.title}<h3 class="display text-xl">{r.title}</h3>{/if}
  {#if r.summary}<p class="text-[15px] leading-relaxed">{r.summary}</p>{/if}
  {#each r.sections as s (s.heading)}
    <div>
      <p class="section-title mb-1">{s.heading}</p>
      <ul class="list-disc space-y-0.5 pl-5 text-sm">{#each s.items as it}<li>{it}</li>{/each}</ul>
    </div>
  {/each}
  {#if r.text}
    <div>
      <p class="section-title mb-1">Texto</p>
      <p class="whitespace-pre-wrap rounded-xl bg-app-ink/5 p-3 text-sm">{r.text}</p>
    </div>
  {/if}
  {#if r.lines.length}
    <ul class="divide-y divide-app-ink/8 rounded-xl border border-app-ink/12 text-sm">
      {#each r.lines as l, i (i)}
        <li class="px-3 py-2">
          <p class="font-medium">{l.name}{#if l.qty > 1} <span class="text-app-muted">× {l.qty}</span>{/if}{#if l.unit_price_cents} <span class="float-right">{moneyCents(l.unit_price_cents * l.qty)}</span>{/if}</p>
          {#if l.detail}<p class="whitespace-pre-wrap text-app-muted">{l.detail}</p>{/if}
          {#each l.alerts ?? [] as a}<p class="mt-1 rounded bg-red-50 px-2 py-1 text-red-800" role="alert">{a}</p>{/each}
        </li>
      {/each}
    </ul>
  {/if}
  {@render children?.()}
  {#each r.notices ?? [] as n}<p class="text-sm text-app-muted">{n}</p>{/each}
  {#if result.basis}
    <details class="text-sm"><summary class="cursor-pointer text-app-muted">Cifras en las que se basa (calculadas por el sistema)</summary><p class="mt-1 whitespace-pre-line">{result.basis}</p></details>
  {/if}
  <div class="flex flex-wrap gap-2">
    <button type="button" class="btn-secondary" onclick={copy}><Icon name="file" size={16} />Copiar</button>
    {#if onprint}<button type="button" class="btn-secondary" onclick={onprint}><Icon name="receipt" size={16} />Imprimir</button>{/if}
  </div>
  <p class="text-xs text-app-muted">Generado {dateTime(result.generated_at)}.</p>
</div>
