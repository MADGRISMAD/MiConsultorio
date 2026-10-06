<script lang="ts">
  import type { Snippet } from 'svelte';
  import Modal from './Modal.svelte';
  import Notice from './Notice.svelte';
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
  let { open, title, op, onconfirm, onclose, confirmLabel = 'Sí', children }: Props = $props();
</script>

<Modal {open} {title} {onclose}>
  {@render children?.()}
  {#if op.phase !== 'idle'}
    <div class="mt-4"><Notice kind={op.phase} message={op.phase === 'loading' ? 'Cargando...' : op.message} /></div>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>No</button>
    <button type="button" class="btn-danger" disabled={op.phase === 'loading'} onclick={onconfirm}>{confirmLabel}</button>
  {/snippet}
</Modal>
