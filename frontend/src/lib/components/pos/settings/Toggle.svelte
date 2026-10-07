<script lang="ts">
  interface Props {
    checked: boolean;
    label: string;
    hint?: string;
    disabled?: boolean;
    onchange?: () => void;
  }
  let { checked = $bindable(), label, hint = '', disabled = false, onchange }: Props = $props();
</script>

<label class="flex cursor-pointer items-start justify-between gap-4 py-2 {disabled ? 'cursor-not-allowed opacity-60' : ''}">
  <span class="min-w-0">
    <span class="block text-sm font-medium">{label}</span>
    {#if hint}<span class="mt-0.5 block text-xs text-app-muted">{hint}</span>{/if}
  </span>
  <input type="checkbox" role="switch" class="peer sr-only" bind:checked {disabled} {onchange} />
  <span class="relative mt-0.5 h-6 w-11 shrink-0 rounded-full bg-app-ink/20 transition peer-checked:bg-app-primary peer-focus-visible:outline peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-app-primary after:absolute after:left-0.5 after:top-0.5 after:h-5 after:w-5 after:rounded-full after:bg-white after:shadow after:transition-transform peer-checked:after:translate-x-5" aria-hidden="true"></span>
</label>
