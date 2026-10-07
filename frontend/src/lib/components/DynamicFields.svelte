<script lang="ts">
  import type { FieldDef, FieldValues } from '$lib/types';

  interface Props {
    fields: FieldDef[];
    values: FieldValues;
    /** prefix for element ids, unique per form on the page */
    id?: string;
    /** field key → message, shown under the field */
    errors?: Record<string, string>;
    /** put each group in its own card-less block with a heading */
    headings?: boolean;
  }
  let { fields, values = $bindable(), id = 'f', errors = {}, headings = true }: Props = $props();

  // Fields come grouped by `group`, keeping the server's order.
  const groups = $derived.by(() => {
    const out: { name: string; items: FieldDef[] }[] = [];
    for (const f of fields) {
      let g = out.find((x) => x.name === f.group);
      if (!g) out.push((g = { name: f.group, items: [] }));
      g.items.push(f);
    }
    return out;
  });

  const str = (k: string) => (values[k] == null ? '' : String(values[k]));
  const list = (k: string): string[] => (Array.isArray(values[k]) ? (values[k] as string[]) : []);
  const set = (k: string, v: unknown) => (values = { ...values, [k]: v });
  function toggle(k: string, opt: string) {
    const cur = list(k);
    set(k, cur.includes(opt) ? cur.filter((x) => x !== opt) : [...cur, opt]);
  }
  /** short option lists read better as buttons than as a dropdown */
  const asChips = (f: FieldDef) => f.type === 'select' && (f.options?.length ?? 0) <= 4;
  const wide = (f: FieldDef) => f.type === 'longtext' || f.type === 'multiselect';
</script>

{#each groups as g (g.name)}
  <fieldset class="min-w-0 border-0 p-0">
    {#if headings}<legend class="section-title mb-3">{g.name}</legend>{/if}
    <div class="grid gap-4 sm:grid-cols-2">
      {#each g.items as f (f.key)}
        {@const fid = `${id}-${f.key}`}
        <div class={wide(f) || asChips(f) ? 'sm:col-span-2' : ''}>
          {#if f.type === 'select' && asChips(f)}
            <span class="label" id="{fid}-l">{f.label}{#if f.required} <span class="text-app-danger" aria-hidden="true">*</span>{/if}</span>
            <div class="flex flex-wrap gap-2" role="radiogroup" aria-labelledby="{fid}-l">
              {#each f.options ?? [] as o}
                <button
                  type="button"
                  role="radio"
                  aria-checked={str(f.key) === o}
                  class="min-h-10 rounded-full border px-4 text-sm font-medium transition {str(f.key) === o ? 'border-app-primary bg-app-primary/10 text-app-primary' : 'border-app-ink/15 text-app-muted hover:border-app-ink/30 hover:text-app-ink'}"
                  onclick={() => set(f.key, str(f.key) === o ? '' : o)}
                >{o}</button>
              {/each}
            </div>
          {:else if f.type === 'multiselect'}
            <span class="label" id="{fid}-l">{f.label}</span>
            <div class="flex flex-wrap gap-2" role="group" aria-labelledby="{fid}-l">
              {#each f.options ?? [] as o}
                <button
                  type="button"
                  aria-pressed={list(f.key).includes(o)}
                  class="min-h-10 rounded-full border px-3.5 text-sm font-medium transition {list(f.key).includes(o) ? 'border-app-primary bg-app-primary/10 text-app-primary' : 'border-app-ink/15 text-app-muted hover:border-app-ink/30 hover:text-app-ink'}"
                  onclick={() => toggle(f.key, o)}
                >{o}</button>
              {/each}
            </div>
          {:else}
            <label class="label" for={fid}>{f.label}{#if f.unit} <span class="font-normal text-app-muted">({f.unit})</span>{/if}{#if f.required} <span class="text-app-danger" aria-hidden="true">*</span>{/if}</label>
            {#if f.type === 'longtext'}
              <textarea id={fid} class="field min-h-24" rows="3" value={str(f.key)} oninput={(e) => set(f.key, e.currentTarget.value)} placeholder={f.placeholder ?? ''} aria-invalid={!!errors[f.key]} aria-describedby={f.hint || errors[f.key] ? `${fid}-h` : undefined}></textarea>
            {:else if f.type === 'select'}
              <select id={fid} class="field" value={str(f.key)} onchange={(e) => set(f.key, e.currentTarget.value)} aria-invalid={!!errors[f.key]} aria-describedby={f.hint || errors[f.key] ? `${fid}-h` : undefined}>
                <option value="">Sin indicar</option>
                {#each f.options ?? [] as o}<option value={o}>{o}</option>{/each}
              </select>
            {:else if f.type === 'date'}
              <input id={fid} class="field" type="date" value={str(f.key)} oninput={(e) => set(f.key, e.currentTarget.value)} aria-invalid={!!errors[f.key]} aria-describedby={f.hint || errors[f.key] ? `${fid}-h` : undefined} />
            {:else if f.type === 'number'}
              <input id={fid} class="field" inputmode="decimal" autocomplete="off" value={str(f.key)} oninput={(e) => set(f.key, e.currentTarget.value)} placeholder={f.placeholder ?? ''} aria-invalid={!!errors[f.key]} aria-describedby={f.hint || errors[f.key] ? `${fid}-h` : undefined} />
            {:else}
              <input id={fid} class="field" autocomplete="off" value={str(f.key)} oninput={(e) => set(f.key, e.currentTarget.value)} placeholder={f.placeholder ?? ''} aria-invalid={!!errors[f.key]} aria-describedby={f.hint || errors[f.key] ? `${fid}-h` : undefined} />
            {/if}
          {/if}
          {#if errors[f.key]}<p id="{fid}-h" class="mt-1 text-xs text-app-danger" role="alert">{errors[f.key]}</p>{:else if f.hint}<p id="{fid}-h" class="hint">{f.hint}</p>{/if}
        </div>
      {/each}
    </div>
  </fieldset>
{/each}
