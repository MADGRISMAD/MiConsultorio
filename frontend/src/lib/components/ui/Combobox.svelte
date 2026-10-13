<script module lang="ts">
  export interface Option<V> {
    key: string;
    value: V;
    disabled?: boolean;
  }
</script>

<script lang="ts" generics="T">
  import type { Snippet } from 'svelte';
  import Icon from './Icon.svelte';

  /**
   * A search box with a list of results (ARIA combobox with a listbox): arrow keys, Home/End, Enter and Escape, the active option
   * announced through aria-activedescendant, and a status line (loading, error, empty) read aloud politely. The options are not
   * buttons: they are picked from the input, which keeps focus the whole time. The caller does the searching and gives the options.
   */
  interface Props {
    id: string;
    label: string;
    /** the label is read by screen readers but not shown */
    hideLabel?: boolean;
    placeholder?: string;
    query: string;
    options: Option<T>[];
    /** text under the list: «Buscando…», an error, «Sin resultados». Empty hides it. */
    status?: string;
    statusTone?: 'muted' | 'danger';
    /** the list stays closed until this many characters were typed */
    minChars?: number;
    type?: 'text' | 'search';
    /** how to draw one option */
    option: Snippet<[Option<T>, boolean]>;
    onpick: (o: Option<T>) => void;
    /** the person typed (the caller searches here, after its own delay) */
    oninput?: (text: string) => void;
    /** the box got focus (the caller may load a first list) */
    onopen?: () => void;
    class?: string;
  }
  let { id, label, hideLabel = false, placeholder = '', query = $bindable(), options, status = '', statusTone = 'muted', minChars = 0, type = 'text', option, onpick, oninput, onopen, class: cls = '' }: Props = $props();

  let open = $state(false);
  let active = $state(0);
  const expanded = $derived(open && query.trim().length >= minChars);

  const enabled = (i: number) => !!options[i] && !options[i].disabled;
  const firstEnabled = () => options.findIndex((o) => !o.disabled);

  // a new list starts on its first usable option
  $effect(() => {
    void options;
    active = Math.max(0, firstEnabled());
  });

  function step(dir: 1 | -1) {
    if (!options.length) return;
    let i = active;
    for (let n = 0; n < options.length; n++) {
      i = (i + dir + options.length) % options.length;
      if (enabled(i)) {
        active = i;
        return;
      }
    }
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (!open) open = true;
      else step(1);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      step(-1);
    } else if (e.key === 'Home' && expanded) {
      e.preventDefault();
      active = Math.max(0, firstEnabled());
    } else if (e.key === 'End' && expanded) {
      e.preventDefault();
      active = options.length - 1 - [...options].reverse().findIndex((o) => !o.disabled);
    } else if (e.key === 'Enter') {
      if (!expanded || !enabled(active)) return;
      e.preventDefault();
      pick(options[active]);
    } else if (e.key === 'Escape' && open) {
      e.stopPropagation();
      open = false;
    }
  }

  function pick(o: Option<T>) {
    if (o.disabled) return;
    open = false;
    onpick(o);
  }

  function onfocusout(e: FocusEvent) {
    const next = e.relatedTarget as Node | null;
    if (!next || !(e.currentTarget as HTMLElement).contains(next)) open = false;
  }
</script>

<div class="relative text-left {cls}">
  <label class={hideLabel ? 'sr-only' : 'label'} for={id}>{label}</label>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="relative" {onfocusout}>
    <Icon name="search" size={17} class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-app-muted" />
    <input
      {id}
      {type}
      class="field pl-10"
      role="combobox"
      autocomplete="off"
      {placeholder}
      aria-expanded={expanded}
      aria-controls="{id}-list"
      aria-autocomplete="list"
      aria-activedescendant={expanded && enabled(active) ? `${id}-o${active}` : undefined}
      bind:value={query}
      oninput={(e) => {
        open = true;
        // the text is passed along: the bound query may not have reached the parent yet
        oninput?.(e.currentTarget.value);
      }}
      onfocus={() => {
        open = true;
        onopen?.();
      }}
      {onkeydown}
    />
    {#if expanded}
      <div class="absolute inset-x-0 top-full z-30 mt-1.5 overflow-hidden rounded-2xl border border-app-ink/10 bg-app-panel shadow-app">
        <ul id="{id}-list" role="listbox" aria-label={label} class="max-h-64 overflow-y-auto p-1.5">
          {#each options as o, i (o.key)}
            <!-- the input keeps the focus; the keyboard works there, the mouse picks here -->
            <li
              id="{id}-o{i}"
              role="option"
              aria-selected={active === i}
              aria-disabled={o.disabled}
              class="rounded-xl {o.disabled ? 'cursor-not-allowed opacity-55' : 'cursor-pointer'} {active === i && !o.disabled ? 'bg-app-primary/10' : o.disabled ? '' : 'hover:bg-app-ink/5'}"
              onmousedown={(e) => e.preventDefault()}
              onclick={() => pick(o)}
              onkeydown={() => {}}
              onmousemove={() => !o.disabled && (active = i)}
            >
              {@render option(o, active === i)}
            </li>
          {/each}
        </ul>
        {#if status}
          <p class="border-t border-app-ink/8 px-4 py-2 text-xs {statusTone === 'danger' ? 'text-app-danger' : 'text-app-muted'}" aria-live="polite">{status}</p>
        {/if}
      </div>
    {/if}
  </div>
</div>
