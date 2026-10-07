<script lang="ts">
  import { theme, type Theme } from '$lib/theme.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';

  const options: { id: Theme; label: string; hint: string; icon: 'sun' | 'moon' }[] = [
    { id: 'light', label: 'Claro', hint: 'El papel blanco y tranquilo de Caresia', icon: 'sun' },
    { id: 'dark', label: 'Oscuro', hint: 'Menos brillo para consultorios con poca luz', icon: 'moon' }
  ];
  function pick(t: Theme) {
    if (theme.mode !== t) theme.toggle();
  }
</script>

<section class="card p-6">
  <h2 class="display text-2xl">Tema</h2>
  <p class="mt-1 text-sm text-app-muted">Se guarda en este dispositivo.</p>
  <div class="mt-4 grid gap-3 sm:grid-cols-2" role="radiogroup" aria-label="Tema de la app">
    {#each options as o}
      <button
        type="button"
        role="radio"
        aria-checked={theme.mode === o.id}
        class="flex items-center gap-3 rounded-2xl border-2 p-4 text-left transition {theme.mode === o.id ? 'border-app-primary bg-app-primary/8' : 'border-app-ink/10 hover:border-app-ink/25'}"
        onclick={() => pick(o.id)}
      >
        <span class="grid h-11 w-11 flex-none place-items-center rounded-xl {theme.mode === o.id ? 'bg-app-primary text-white' : 'bg-app-ink/8 text-app-muted'}"><Icon name={o.icon} size={22} /></span>
        <span class="min-w-0">
          <strong class="block text-[15px] {theme.mode === o.id ? 'text-app-primary' : ''}">{o.label}</strong>
          <small class="block text-xs text-app-muted">{o.hint}</small>
        </span>
      </button>
    {/each}
  </div>
</section>
