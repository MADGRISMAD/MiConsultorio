<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/state';

  // Old address (also where Mercado Pago used to send people back): keep the query, e.g. ?mp=ok
  $effect(() => {
    const q = new URLSearchParams(page.url.search);
    q.set('s', 'terminal');
    void goto(`/ajustes?${q}`, { replaceState: true });
  });
</script>
