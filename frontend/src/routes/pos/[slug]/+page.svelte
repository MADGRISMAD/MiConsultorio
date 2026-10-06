<script lang="ts">
  import { page } from '$app/state';
  import { POS_WINDOWS } from '$lib/pos';
  import Guard from '$lib/components/Guard.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';

  const win = $derived(POS_WINDOWS.find((w) => w.slug === page.params.slug));
</script>

<svelte:head><title>{win?.title ?? 'No encontrado'} · Caresia</title></svelte:head>

<Guard title={win?.title} cobros>
  {#if win}
    <div class="mb-6 flex flex-wrap items-center gap-4">
      <span class="grid h-14 w-14 place-items-center rounded-2xl bg-app-primary/12 text-app-primary"><Icon name={win.icon} size={28} /></span>
      <div class="min-w-0 flex-1">
        <div class="flex flex-wrap items-center gap-3">
          <h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">{win.title}</h1>
          <span class="badge-soon">Próximamente</span>
        </div>
        <p class="mt-1 text-sm text-app-muted">{win.summary}</p>
      </div>
    </div>

    <div class="card mb-6 flex items-start gap-3 border-dashed p-4 text-sm text-app-muted">
      <Icon name="info" size={20} class="mt-0.5 flex-none text-app-accent" />
      <p>Esta ventana ya está en el menú para que veas dónde vivirá. El módulo se construirá más adelante; esto es lo que incluirá:</p>
    </div>

    <ul class="grid gap-4 sm:grid-cols-2">
      {#each win.features as f, i}
        <li class="card page-in p-5 opacity-90" style="animation-delay: {i * 60}ms">
          <h2 class="flex items-center gap-2 text-base font-semibold"><Icon name="sparkles" size={17} class="text-app-accent" />{f.title}</h2>
          <p class="mt-2 text-sm leading-relaxed text-app-muted">{f.text}</p>
        </li>
      {/each}
    </ul>
  {:else}
    <div class="card"><EmptyState icon="search" title="Ventana no encontrada" text="Esta sección no existe."><a href="/" class="btn-primary">Volver al inicio</a></EmptyState></div>
  {/if}
</Guard>
