<script lang="ts">
  import Icon from './Icon.svelte';

  interface Props {
    /** the date as YYYY-MM-DD ('' when empty or not complete yet) */
    value: string;
    id?: string;
    /** latest allowed date, YYYY-MM-DD */
    max?: string;
    /** true while something was typed that is not a complete, real date */
    invalid?: boolean;
    describedby?: string;
    ariaInvalid?: boolean;
  }
  let { value = $bindable(''), id, max, invalid = $bindable(false), describedby, ariaInvalid = false }: Props = $props();

  const show = (iso: string) => (/^\d{4}-\d{2}-\d{2}$/.test(iso) ? `${iso.slice(8, 10)}/${iso.slice(5, 7)}/${iso.slice(0, 4)}` : '');
  /** DD/MM/AAAA to YYYY-MM-DD, or '' when it is not a real date */
  function parse(t: string): string {
    const m = /^(\d{2})\/(\d{2})\/(\d{4})$/.exec(t);
    if (!m) return '';
    const [d, mo, y] = [Number(m[1]), Number(m[2]), Number(m[3])];
    const dt = new Date(Date.UTC(y, mo - 1, d));
    if (y < 1900 || dt.getUTCFullYear() !== y || dt.getUTCMonth() !== mo - 1 || dt.getUTCDate() !== d) return '';
    return `${m[3]}-${m[2]}-${m[1]}`;
  }

  let text = $state(show(value));
  // a value set from outside (the form loads, the owner is picked...) shows up in the box
  $effect(() => {
    if (value !== parse(text)) {
      text = show(value);
      invalid = false;
    }
  });

  function onInput(e: Event) {
    const el = e.currentTarget as HTMLInputElement;
    // digits only; the slashes put themselves: 12051990 becomes 12/05/1990
    const d = el.value.replace(/\D/g, '').slice(0, 8);
    let t = d;
    if (d.length > 4) t = `${d.slice(0, 2)}/${d.slice(2, 4)}/${d.slice(4)}`;
    else if (d.length > 2) t = `${d.slice(0, 2)}/${d.slice(2)}`;
    text = t;
    el.value = t;
    const iso = parse(t);
    value = iso;
    invalid = t !== '' && iso === '';
  }

  let picker = $state<HTMLInputElement>();
  function pick(e: Event) {
    const iso = (e.currentTarget as HTMLInputElement).value;
    if (!iso) return;
    value = iso;
    text = show(iso);
    invalid = false;
  }
</script>

<div class="relative">
  <input
    {id}
    class="field pr-11"
    type="text"
    inputmode="numeric"
    autocomplete="bday"
    placeholder="DD/MM/AAAA"
    maxlength="10"
    value={text}
    oninput={onInput}
    aria-invalid={ariaInvalid || (invalid && text.length === 10)}
    aria-describedby={describedby}
  />
  <button type="button" class="absolute right-1.5 top-1/2 grid h-8 w-8 -translate-y-1/2 place-items-center rounded-lg text-app-muted transition hover:bg-app-ink/6 hover:text-app-ink" aria-label="Elegir en el calendario" onclick={() => picker?.showPicker?.()}>
    <Icon name="calendar" size={18} />
  </button>
  <input bind:this={picker} type="date" class="pointer-events-none absolute bottom-0 right-0 h-0 w-0 opacity-0" tabindex="-1" aria-hidden="true" {max} value={value} onchange={pick} />
</div>
{#if invalid && text.length === 10}<p class="hint !text-app-danger" role="alert">Esa fecha no existe. Revisa el día, el mes y el año.</p>{/if}
