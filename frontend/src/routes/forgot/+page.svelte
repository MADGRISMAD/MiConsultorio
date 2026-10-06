<script lang="ts">
  import { theme } from '$lib/theme.svelte';
  import Brand from '$lib/components/ui/Brand.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';

  $effect(() => theme.init());

  const steps = [
    'Pídele a un administrador de tu consultorio que entre a Usuarios y permisos.',
    'Que presione el candado junto a tu usuario y escriba una contraseña nueva.',
    'Entra con la contraseña nueva y cámbiala cuando quieras.'
  ];
</script>

<svelte:head><title>Recuperar contraseña · Caresia</title></svelte:head>

<div
  class="app fixed inset-0 grid place-items-center overflow-y-auto px-4 py-8"
  data-theme={theme.mode}
  style="background-image: radial-gradient(ellipse 60% 420px at 50% 0%, rgb(var(--app-primary) / 0.1), transparent 75%); background-repeat: no-repeat"
>
  <div class="card page-in w-full max-w-md px-6 py-9 sm:px-9">
    <a href="/" class="inline-flex" aria-label="Caresia, ir al inicio"><Brand size={32} class="text-[1.05rem]" /></a>

    <h1 class="display mt-7 text-[2.2rem] leading-none">Recuperar <em class="italic text-app-primary">contraseña</em></h1>
    <p class="mt-3 text-[15px] text-app-muted">El envío de enlaces por correo todavía no está disponible. Mientras tanto, restablecerla es muy sencillo:</p>

    <ol class="mt-6 space-y-2.5 text-sm">
      {#each steps as step, i}
        <li class="flex items-start gap-3 rounded-xl bg-app-elevated p-3.5">
          <span class="grid h-6 w-6 flex-none place-items-center rounded-full bg-app-ink font-mono text-xs text-app-surface">{i + 1}</span>
          <span class="pt-0.5">{step}</span>
        </li>
      {/each}
    </ol>

    <p class="mt-5 flex items-start gap-2 text-xs text-app-muted"><Icon name="info" size={16} class="mt-px flex-none" />¿Eres el único administrador? Contacta al soporte de Caresia para recuperar el acceso.</p>

    <a href="/login" class="btn-primary btn-lg mt-7">Volver al login</a>
  </div>
</div>
