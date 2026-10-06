<script lang="ts">
  import { theme } from '$lib/theme.svelte';
  import BrandMark from '$lib/components/ui/BrandMark.svelte';
  import BrandName from '$lib/components/ui/BrandName.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';

  $effect(() => theme.init());
</script>

<svelte:head><title>Recuperar contraseña · Caresia</title></svelte:head>

<div
  class="app fixed inset-0 grid place-items-center overflow-y-auto px-4 py-8"
  data-theme={theme.mode}
  style="background: radial-gradient(ellipse 70% 45% at 10% 10%, rgba(224,138,30,.22), transparent 60%), radial-gradient(ellipse 60% 50% at 95% 95%, rgba(91,154,232,.3), transparent 60%), linear-gradient(160deg, #0a1a30 0%, #123056 45%, #1e5aa8 100%)"
>
  <div class="card page-in w-full max-w-md px-6 py-8 text-center sm:px-8">
    <a href="/" class="mx-auto inline-flex flex-col items-center gap-2" aria-label="Caresia, ir al inicio">
      <BrandMark size={52} />
      <span class="text-2xl"><BrandName /></span>
    </a>

    <h1 class="mt-5 text-lg font-extrabold">Recuperar contraseña</h1>
    <p class="mt-2 text-sm text-app-muted">El envío de enlaces por correo todavía no está disponible. Mientras tanto, restablecer tu contraseña es muy sencillo:</p>

    <ol class="mt-5 space-y-3 text-left text-sm">
      <li class="flex gap-3 rounded-xl bg-app-elevated p-3.5">
        <span class="grid h-6 w-6 flex-none place-items-center rounded-full bg-app-primary text-xs font-extrabold text-app-on-primary">1</span>
        Pídele a un administrador de tu consultorio que entre a <strong>Usuarios y permisos</strong>.
      </li>
      <li class="flex gap-3 rounded-xl bg-app-elevated p-3.5">
        <span class="grid h-6 w-6 flex-none place-items-center rounded-full bg-app-primary text-xs font-extrabold text-app-on-primary">2</span>
        Que presione el ícono de editar junto a tu usuario y escriba una contraseña nueva.
      </li>
      <li class="flex gap-3 rounded-xl bg-app-elevated p-3.5">
        <span class="grid h-6 w-6 flex-none place-items-center rounded-full bg-app-primary text-xs font-extrabold text-app-on-primary">3</span>
        Entra con la contraseña nueva y cámbiala cuando quieras.
      </li>
    </ol>

    <p class="mt-4 flex items-start gap-2 text-left text-xs text-app-muted"><Icon name="info" size={16} class="mt-px flex-none" />¿Eres el único administrador? Contacta al soporte de Caresia para recuperar el acceso.</p>

    <a href="/login" class="btn-primary btn-lg mt-6">Volver al login</a>
  </div>
</div>
