<script lang="ts">
  import { toCents, centsText } from './money';

  interface Props {
    /** Amount in cents; null while empty or invalid. */
    cents: number | null;
    id?: string;
    label?: string;
    placeholder?: string;
    autofocus?: boolean;
    disabled?: boolean;
    class?: string;
    onenter?: () => void;
  }
  let { cents = $bindable(null), id, label, placeholder = '0.00', autofocus = false, disabled = false, class: cls = '', onenter }: Props = $props();

  let text = $state(centsText(cents));
  let input = $state<HTMLInputElement>();

  // Keep the text in step when the parent changes the amount (quick buttons, defaults…)
  $effect(() => {
    if (cents !== toCents(text)) text = centsText(cents);
  });
  $effect(() => {
    if (autofocus) {
      input?.focus();
      input?.select();
    }
  });
</script>

<div class={cls}>
  {#if label}<label class="label" for={id}>{label}</label>{/if}
  <div class="relative">
    <span class="pointer-events-none absolute inset-y-0 left-3.5 grid place-items-center text-app-muted" aria-hidden="true">$</span>
    <input
      bind:this={input}
      {id}
      {disabled}
      class="field pl-8 text-right tabular-nums"
      type="text"
      inputmode="decimal"
      autocomplete="off"
      {placeholder}
      value={text}
      oninput={(e) => {
        text = e.currentTarget.value;
        cents = toCents(text);
      }}
      onkeydown={(e) => {
        if (e.key === 'Enter' && onenter) {
          e.preventDefault();
          onenter();
        }
      }}
    />
  </div>
</div>
