<script lang="ts">
  import QRCode from 'qrcode';

  let { value, size = 208 }: { value: string; size?: number } = $props();
  let src = $state('');

  $effect(() => {
    const v = value;
    let stale = false;
    // White background on purpose: scanners need contrast whatever the theme.
    QRCode.toDataURL(v, { margin: 1, width: size * 2, color: { dark: '#0b2540', light: '#ffffff' } })
      .then((u) => {
        if (!stale) src = u;
      })
      .catch(() => {
        if (!stale) src = '';
      });
    return () => (stale = true);
  });
</script>

{#if src}
  <img {src} alt="Código QR de la liga de pago" width={size} height={size} class="rounded-xl border border-app-ink/10 bg-white p-1" style="width:{size}px;height:{size}px" />
{:else}
  <div class="grid place-items-center rounded-xl bg-app-elevated" style="width:{size}px;height:{size}px"><span class="spin"></span></div>
{/if}
