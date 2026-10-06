<script lang="ts">
  interface Props {
    kind: 'loading' | 'success' | 'error';
    message: string;
  }
  let { kind, message }: Props = $props();
  const src = $derived(kind === 'loading' ? '/loading.gif' : kind === 'success' ? '/ok.png' : '/error.png');
  const alt = $derived(kind === 'loading' ? 'Cargando' : kind === 'success' ? 'Éxito' : 'Error');
</script>

<div class="flex flex-col items-center gap-2 text-center" role={kind === 'error' ? 'alert' : 'status'}>
  <img class="h-20 w-20 object-contain" {src} {alt} />
  <span class="text-sm text-ink-soft">{message}</span>
</div>
