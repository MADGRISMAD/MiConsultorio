<script lang="ts">
  import { page } from '$app/state';

  /** Catálogo groups two screens: what is sold (servicios y precios) and what is in stock (inventario). */
  const links = [
    { href: '/pos/servicios', label: 'Servicios y precios' },
    { href: '/pos/inventario', label: 'Inventario' }
  ];
</script>

<nav class="mb-5 flex gap-2" aria-label="Catálogo">
  {#each links as l (l.href)}
    <a href={l.href} class="min-h-10 px-4 {page.url.pathname === l.href ? 'btn-primary' : 'btn-secondary'}" aria-current={page.url.pathname === l.href ? 'page' : undefined}>{l.label}</a>
  {/each}
</nav>
