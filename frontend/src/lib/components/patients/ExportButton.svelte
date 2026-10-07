<script lang="ts">
  import { ApiError } from '$lib/api';
  import { securityApi } from '$lib/api/security';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';

  /** Descarga el expediente completo en JSON (solicitudes ARCO y portabilidad). Solo para administradores y profesionales con historial. */
  let { patientId, fileNumber }: { patientId: string; fileNumber?: number } = $props();
  const op = new Op();

  async function download() {
    const ok = await op.run(async () => {
      let res: Response;
      try {
        res = await fetch(securityApi.exportUrl(patientId), { credentials: 'same-origin' });
      } catch {
        throw new ApiError('No se pudo conectar con el servidor.', 0);
      }
      if (!res.ok) {
        const data = await res.json().catch(() => null);
        throw new ApiError(data?.message ?? 'No se pudo exportar el expediente.', res.status, data?.code ?? '');
      }
      const url = URL.createObjectURL(await res.blob());
      const a = document.createElement('a');
      a.href = url;
      a.download = `expediente-${fileNumber ?? patientId}.json`;
      a.click();
      URL.revokeObjectURL(url);
    });
    if (ok) toast.show('Expediente exportado. La descarga quedó registrada en los accesos.');
  }
</script>

<button type="button" class="btn-secondary" disabled={op.phase === 'loading'} onclick={download} title="Descarga todo el expediente en JSON (acceso y portabilidad de datos)">
  {#if op.phase === 'loading'}<span class="spin"></span>{:else}<Icon name="download" size={16} />{/if}Exportar expediente
</button>
{#if op.phase === 'error'}<p class="mt-2 text-sm text-app-danger" role="alert">{op.message}</p>{/if}
