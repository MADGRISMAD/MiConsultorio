<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import { api } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';

  interface Props {
    saleId: string;
    /** pre-filled address, e.g. the patient's */
    email?: string;
  }
  let { saleId, email = '' }: Props = $props();

  let open = $state(false);
  let address = $state('');
  let sent = $state(false);
  const op = new Op();
  const uid = $props.id();

  $effect(() => {
    address = email;
  });

  async function send(e: SubmitEvent) {
    e.preventDefault();
    const ok = await op.run(() => api.pos.emailSale(saleId, address.trim()));
    if (ok) {
      sent = true;
      open = false;
      toast.show('Ticket enviado por correo.');
    }
  }
</script>

{#if !open}
  <button type="button" class="btn-secondary min-h-12 w-full" onclick={() => (open = true)}>
    <Icon name="mail" size={18} />{sent ? 'Enviar a otro correo' : 'Enviar por correo'}
  </button>
{:else}
  <form class="grid gap-2 rounded-xl border border-app-ink/10 p-3 text-left" onsubmit={send}>
    <label class="label" for="{uid}-mail">Correo del paciente</label>
    <input id="{uid}-mail" class="field" type="email" inputmode="email" autocomplete="off" bind:value={address} placeholder="paciente@correo.com" required />
    <OpError op={op} />
    <div class="flex justify-end gap-2">
      <button type="button" class="btn-ghost" onclick={() => (open = false)}>Cancelar</button>
      <button type="submit" class="btn-primary" disabled={op.phase === 'loading'}>{#if op.phase === 'loading'}<span class="spin"></span>{/if}Enviar ticket</button>
    </div>
  </form>
{/if}
