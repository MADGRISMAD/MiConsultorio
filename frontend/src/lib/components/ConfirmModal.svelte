<script lang="ts">
  import type { Snippet } from 'svelte';
  import Modal from './Modal.svelte';
  import Icon from './ui/Icon.svelte';
  import type { Op } from '$lib/op.svelte';

  interface Props {
    open: boolean;
    title: string;
    op: Op;
    onconfirm: () => void;
    onclose: () => void;
    confirmLabel?: string;
    children?: Snippet;
  }
  let { open, title, op, onconfirm, onclose, confirmLabel = 'Confirmar', children }: Props = $props();
</script>

<Modal {open} {title} {onclose}>
  <div class="text-app-muted">{@render children?.()}</div>
  {#if op.phase === 'error'}
    <p class="alert mt-4" role="alert"><Icon name="alert" size={18} />{op.message}</p>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    <button type="button" class="btn-danger" disabled={op.phase === 'loading'} onclick={onconfirm}>
      {#if op.phase === 'loading'}<span class="spin"></span>{/if}{confirmLabel}
    </button>
  {/snippet}
</Modal>
