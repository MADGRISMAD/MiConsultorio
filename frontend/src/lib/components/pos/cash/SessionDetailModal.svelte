<script lang="ts">
  import Modal from '$lib/components/Modal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { api } from '$lib/api';
  import { toast } from '$lib/toast.svelte';
  import { printHtml } from '$lib/printer/ticket';
  import type { CashSession } from '$lib/types';
  import CashBreakdown from './CashBreakdown.svelte';
  import { corteHtml } from './corte';

  interface Props {
    id: string | null;
    businessName: string;
    paperWidth: 58 | 80;
    onclose: () => void;
  }
  let { id, businessName, paperWidth, onclose }: Props = $props();

  let detail = $state<CashSession | null>(null);
  let error = $state('');

  $effect(() => {
    const current = id;
    detail = null;
    error = '';
    if (!current) return;
    let stale = false;
    api.pos
      .cashSession(current)
      .then((s) => {
        if (!stale) detail = s;
      })
      .catch((e) => {
        if (!stale) error = e instanceof Error ? e.message : 'No se pudo cargar el turno.';
      });
    return () => (stale = true);
  });

  const title = $derived(detail ? `Turno del ${new Date(detail.opened_at).toLocaleDateString('es-MX', { day: 'numeric', month: 'long', year: 'numeric' })}` : 'Detalle del turno');

  async function print() {
    if (!detail) return;
    try {
      await printHtml(corteHtml(detail, businessName, paperWidth));
    } catch {
      toast.show('No se pudo imprimir el corte.', 'error');
    }
  }
</script>

<Modal open={id != null} {title} {onclose} wide>
  {#if error}
    <p class="alert" role="alert"><Icon name="alert" size={18} />{error}</p>
  {:else if detail}
    <p class="mb-4 text-sm text-app-muted">
      Abrió {detail.opened_by || '—'} · {new Date(detail.opened_at).toLocaleTimeString('es-MX', { hour: '2-digit', minute: '2-digit' })}
      {#if detail.closed_at}· cerró {detail.closed_by || '—'} · {new Date(detail.closed_at).toLocaleTimeString('es-MX', { hour: '2-digit', minute: '2-digit' })}{/if}
    </p>
    <CashBreakdown session={detail} />
  {:else}
    <div class="grid place-items-center py-12" role="status"><span class="spin text-app-primary"></span><span class="sr-only">Cargando…</span></div>
  {/if}
  {#snippet footer()}
    {#if detail?.closed_at}<button type="button" class="btn-secondary" onclick={print}><Icon name="receipt" size={16} />Imprimir corte</button>{/if}
    <button type="button" class="btn-primary" onclick={onclose}>Cerrar</button>
  {/snippet}
</Modal>
