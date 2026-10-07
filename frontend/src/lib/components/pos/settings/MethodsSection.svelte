<script lang="ts">
  import { PAY_METHODS, type PayMethod, type PosSettings, type ProviderStatus } from '$lib/types';
  import Icon from '$lib/components/ui/Icon.svelte';

  let { s = $bindable(), providers }: { s: PosSettings; providers: ProviderStatus | null } = $props();

  const order = Object.keys(PAY_METHODS) as PayMethod[];
  const active = (m: PayMethod) => s.methods.includes(m);
  const needsMp = (m: PayMethod) => m === 'mp_point' || m === 'mp_link';

  function toggle(m: PayMethod) {
    if (active(m)) {
      if (s.methods.length <= 1) return; // at least one must stay on
      s.methods = s.methods.filter((x) => x !== m);
    } else {
      s.methods = order.filter((x) => x === m || s.methods.includes(x));
    }
  }
</script>

<section class="card p-6">
  <h2 class="display text-2xl">Métodos de pago</h2>
  <p class="mt-1 text-sm text-app-muted">Los que estén activos aparecerán al cobrar. Debe haber al menos uno.</p>
  <ul class="mt-4 divide-y divide-app-ink/10">
    {#each order as m (m)}
      {@const on = active(m)}
      {@const last = on && s.methods.length <= 1}
      <li class="py-2">
        <label class="flex items-start justify-between gap-4 {last ? 'cursor-not-allowed' : 'cursor-pointer'}">
          <span class="min-w-0">
            <span class="block text-sm font-medium">{PAY_METHODS[m].label}</span>
            <span class="block text-xs text-app-muted">{PAY_METHODS[m].hint}{last ? ' · es el único activo' : ''}</span>
            {#if needsMp(m) && on && providers && !providers.point_connected}
              <span class="mt-1 flex items-center gap-1 text-xs text-app-warning"><Icon name="alert" size={14} />Conecta tu cuenta de Mercado Pago (más abajo) para poder usarlo.</span>
            {/if}
          </span>
          <input type="checkbox" role="switch" class="peer sr-only" checked={on} disabled={last} onchange={() => toggle(m)} />
          <span class="relative mt-0.5 h-6 w-11 shrink-0 rounded-full bg-app-ink/20 transition peer-checked:bg-app-primary peer-disabled:opacity-60 peer-focus-visible:outline peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-app-primary after:absolute after:left-0.5 after:top-0.5 after:h-5 after:w-5 after:rounded-full after:bg-white after:shadow after:transition-transform peer-checked:after:translate-x-5" aria-hidden="true"></span>
        </label>
      </li>
    {/each}
  </ul>
</section>
