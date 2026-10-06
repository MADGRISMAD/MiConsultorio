<div class="space-y-3 p-5" role="status" aria-label="Cargando">
  {#each [0, 1, 2, 3] as i}
    <div class="h-10 animate-pulse rounded-lg bg-app-ink/8" style="animation-delay: {i * 120}ms"></div>
  {/each}
</div>
