<script lang="ts">
  import Alert from '$lib/components/ui/Alert.svelte';
  import Modal from '$lib/components/Modal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';

  interface Props {
    open: boolean;
    message: string;
    busy: boolean;
    onclose: () => void;
    onconfirm: (reason: string) => void;
  }
  let { open, message, busy, onclose, onconfirm }: Props = $props();
  const uid = $props.id();
  let reason = $state('');
  let error = $state('');

  $effect(() => {
    if (open) {
      reason = '';
      error = '';
    }
  });

  function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!reason.trim()) return void (error = 'Escribe el motivo.');
    onconfirm(reason.trim());
  }
</script>

<Modal {open} title="Lotes caducados" {onclose}>
  <form id="{uid}-f" class="space-y-4" onsubmit={submit} novalidate>
    <Alert>{message}</Alert>
    <p class="text-sm text-app-muted">Como administrador puedes venderlos de todos modos. Queda registrado en la bitácora de actividad con tu motivo.</p>
    <div>
      <label class="label" for="{uid}-r">Motivo</label>
      <!-- svelte-ignore a11y_autofocus -->
      <input id="{uid}-r" class="field" bind:value={reason} maxlength="200" autocomplete="off" aria-invalid={!!error} autofocus placeholder="Ej. Uso interno autorizado, sin riesgo" />
      {#if error}<p class="hint text-app-danger" role="alert">{error}</p>{/if}
    </div>
  </form>
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    <button type="submit" form="{uid}-f" class="btn-primary" disabled={busy}>{#if busy}<span class="spin"></span>{/if}Vender de todos modos</button>
  {/snippet}
</Modal>
