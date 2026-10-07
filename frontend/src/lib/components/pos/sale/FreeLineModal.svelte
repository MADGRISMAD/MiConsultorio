<script lang="ts">
  import Modal from '$lib/components/Modal.svelte';
  import MoneyInput from './MoneyInput.svelte';

  interface Props {
    open: boolean;
    defaultTax: number;
    onclose: () => void;
    onadd: (name: string, priceCents: number, taxRate: number) => void;
  }
  let { open, defaultTax, onclose, onadd }: Props = $props();

  let name = $state('');
  let price = $state<number | null>(null);
  let tax = $state('');
  let error = $state('');

  $effect(() => {
    if (open) {
      name = '';
      price = null;
      tax = String(defaultTax);
      error = '';
    }
  });

  function submit(e: SubmitEvent) {
    e.preventDefault();
    const rate = tax.trim() === '' ? 0 : Number(tax);
    if (!name.trim()) error = 'Escribe el nombre del concepto.';
    else if (!price || price <= 0) error = 'Escribe un precio mayor a cero.';
    else if (!Number.isFinite(rate) || rate < 0 || rate > 100) error = 'El IVA debe estar entre 0 y 100.';
    else {
      onadd(name.trim(), price, rate);
      onclose();
    }
  }
</script>

<Modal {open} title="Concepto libre" {onclose}>
  <form id="free-line" onsubmit={submit} class="space-y-4">
    <div>
      <label class="label" for="fl-name">Concepto</label>
      <input id="fl-name" class="field" bind:value={name} maxlength="160" placeholder="Ej. Consulta de seguimiento" autocomplete="off" />
    </div>
    <div class="grid gap-4 sm:grid-cols-2">
      <MoneyInput id="fl-price" label="Precio (con IVA)" bind:cents={price} />
      <div>
        <label class="label" for="fl-tax">IVA incluido (%)</label>
        <input id="fl-tax" class="field text-right tabular-nums" bind:value={tax} inputmode="decimal" placeholder="0" />
        <p class="hint">Usa 0 si el concepto no causa IVA.</p>
      </div>
    </div>
    {#if error}<p class="alert" role="alert">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    <button type="submit" form="free-line" class="btn-primary">Agregar a la cuenta</button>
  {/snippet}
</Modal>
