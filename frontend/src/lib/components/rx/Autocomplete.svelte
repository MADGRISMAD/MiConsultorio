<script lang="ts" generics="T">
  // Text input with a suggestion list fed by an async search (catalog medicines, CIE-10).
  // The typed text stays free: picking a suggestion is optional.
  interface Props {
    id: string;
    value: string;
    search: (q: string) => Promise<T[]>;
    title: (item: T) => string;
    detail?: (item: T) => string;
    onpick: (item: T) => void;
    placeholder?: string;
    minChars?: number;
    required?: boolean;
    describedby?: string;
    /** the prescriber typed (as opposed to picking a suggestion) */
    oninput?: () => void;
  }
  let { id, value = $bindable(), search, title, detail, onpick, placeholder = '', minChars = 2, required = false, describedby, oninput }: Props = $props();

  let results = $state<T[]>([]);
  let open = $state(false);
  let active = $state(-1);
  let timer: ReturnType<typeof setTimeout> | undefined;
  let seq = 0;

  function onInput() {
    oninput?.();
    clearTimeout(timer);
    const q = value.trim();
    if (q.length < minChars) {
      results = [];
      open = false;
      return;
    }
    const mine = ++seq;
    timer = setTimeout(async () => {
      try {
        const r = await search(q);
        if (mine !== seq) return;
        results = r;
        active = -1;
        open = r.length > 0;
      } catch {
        if (mine === seq) open = false;
      }
    }, 180);
  }

  function pick(item: T) {
    seq++;
    open = false;
    results = [];
    onpick(item);
  }

  function onKey(ev: KeyboardEvent) {
    if (!open) return;
    if (ev.key === 'ArrowDown') {
      ev.preventDefault();
      active = (active + 1) % results.length;
    } else if (ev.key === 'ArrowUp') {
      ev.preventDefault();
      active = (active - 1 + results.length) % results.length;
    } else if (ev.key === 'Enter' && active >= 0) {
      ev.preventDefault();
      pick(results[active]);
    } else if (ev.key === 'Escape') {
      ev.stopPropagation();
      open = false;
    }
  }
</script>

<div class="relative">
  <input
    {id}
    class="field"
    role="combobox"
    aria-expanded={open}
    aria-controls="{id}-list"
    aria-autocomplete="list"
    aria-activedescendant={active >= 0 ? `${id}-opt-${active}` : undefined}
    aria-describedby={describedby}
    autocomplete="off"
    {placeholder}
    {required}
    bind:value
    oninput={onInput}
    onkeydown={onKey}
    onblur={() => {
      clearTimeout(timer);
      seq++; // a search still in flight must not reopen the list
      setTimeout(() => (open = false), 120);
    }}
  />
  {#if open}
    <ul id="{id}-list" role="listbox" class="absolute z-30 mt-1 max-h-64 w-full overflow-y-auto rounded-xl border border-app-ink/10 bg-app-surface p-1 shadow-lg">
      {#each results as item, i (i)}
        <li
          id="{id}-opt-{i}"
          role="option"
          aria-selected={i === active}
          class="cursor-pointer rounded-lg px-3 py-2 text-sm {i === active ? 'bg-app-primary/10' : 'hover:bg-app-elevated'}"
          onmousedown={(e) => {
            e.preventDefault();
            pick(item);
          }}
        >
          <span class="font-medium">{title(item)}</span>
          {#if detail}<span class="block truncate text-xs text-app-muted">{detail(item)}</span>{/if}
        </li>
      {/each}
    </ul>
  {/if}
</div>
