<script lang="ts">
  import { contactEmail } from '$lib/landing/data';
  import { session } from '$lib/session.svelte';
  import { dateShort } from '$lib/format';
  import Icon from './ui/Icon.svelte';

  const b = $derived(session.user?.billing);
  const copy = $derived(
    b?.state === 'suspended'
      ? { title: 'Este consultorio está suspendido', text: b.suspended_reason ? `Motivo: ${b.suspended_reason}.` : 'Un administrador de Caresia suspendió el acceso.' }
      : b?.state === 'past_due'
        ? { title: 'Hay un pago pendiente', text: `El periodo pagado venció${b.current_period_end ? ` el ${dateShort(b.current_period_end)}` : ''}. Regulariza el pago para volver a usar Caresia.` }
        : { title: 'Tu prueba gratuita terminó', text: `La prueba terminó${b?.trial_ends_at ? ` el ${dateShort(b.trial_ends_at)}` : ''}. Elige un plan para seguir usando Caresia.` }
  );
  const isAdmin = $derived(session.user?.role === 'admin');
</script>

<div class="card mx-auto mt-6 max-w-lg px-6 py-12 text-center sm:px-10">
  <span class="mx-auto grid h-14 w-14 place-items-center rounded-2xl bg-app-warning/14 text-app-warning"><Icon name="lock" size={26} /></span>
  <h1 class="display mt-5 text-4xl leading-none">{copy.title}</h1>
  <p class="mt-3 text-[15px] text-app-muted">{copy.text}</p>
  <p class="mt-2 text-sm text-app-muted">
    {isAdmin ? 'Tus datos están a salvo y no se pierde nada.' : 'Avísale al administrador de tu consultorio.'}
  </p>
  {#if isAdmin}
    <a class="btn-primary mt-7" href="/suscripcion"><Icon name="wallet" size={18} />Elegir plan y pagar</a>
    <a class="btn-ghost mt-2" href="mailto:{contactEmail}?subject={encodeURIComponent('Suscripción de mi consultorio en Caresia')}"><Icon name="mail" size={18} />Hablar con Caresia</a>
  {:else}
  <a class="btn-primary mt-7" href="mailto:{contactEmail}?subject={encodeURIComponent('Suscripción de mi consultorio en Caresia')}"><Icon name="mail" size={18} />Contactar a Caresia</a>
  {/if}
</div>
