<script lang="ts">
  import { onMount } from 'svelte';
  import { ApiError } from '$lib/api';
  import { waitlistApi } from '$lib/api/waitlist';
  import { Op } from '$lib/op.svelte';
  import type { PublicWaitlist } from '$lib/types/waitlist';
  import Icon from '$lib/components/ui/Icon.svelte';

  let { token }: { token: string } = $props();

  let data = $state<PublicWaitlist | null>(null);
  let missing = $state(false);
  let result = $state<'' | 'accepted' | 'declined' | 'left'>('');
  let now = $state(Date.now());
  const load = new Op();
  const act = new Op();

  const dateLong = (d: string) => {
    const [y, m, day] = d.split('-').map(Number);
    return new Date(y, m - 1, day).toLocaleDateString('es-MX', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' });
  };

  const left = $derived.by(() => {
    if (!data?.offer) return 0;
    return Math.max(0, Math.floor((new Date(data.offer.expires_at).getTime() - now) / 1000));
  });
  const clock = $derived(`${Math.floor(left / 60)}:${String(left % 60).padStart(2, '0')}`);

  onMount(() => {
    const t = setInterval(() => (now = Date.now()), 1000);
    load
      .run(async () => {
        data = await waitlistApi.view(token);
      })
      .then((ok) => {
        if (!ok) missing = load.message.length > 0;
      });
    return () => clearInterval(t);
  });

  async function respond(action: 'accept' | 'decline' | 'leave') {
    const ok = await act.run(async () => {
      data = await waitlistApi.act(token, action);
    });
    if (ok) result = action === 'accept' ? 'accepted' : action === 'decline' ? 'declined' : 'left';
    else if (/disponible|terminó|ya no/i.test(act.message)) {
      try {
        data = await waitlistApi.view(token);
      } catch (e) {
        if (e instanceof ApiError && e.status === 404) missing = true;
      }
    }
  }
</script>

{#if load.phase === 'loading' || (load.phase === 'idle' && !data)}
  <div class="card px-6 py-10 text-center text-sm text-app-muted" role="status">Cargando…</div>
{:else if !data}
  <section class="card px-6 py-9" role="alert">
    <div class="grid h-12 w-12 place-items-center rounded-full bg-app-ink/8 text-app-muted"><Icon name="clock" size={24} /></div>
    <h1 class="display mt-4 text-3xl leading-tight">No encontramos esta solicitud</h1>
    <p class="mt-3 text-app-muted">Revisa que el enlace esté completo o comunícate directamente con el consultorio.</p>
  </section>
{:else}
  <header class="mb-5 mt-2">
    <p class="section-title flex items-center gap-1.5"><Icon name="clock-plus" size={14} />Lista de espera</p>
    <h1 class="display mt-2 text-[2rem] leading-[1.05] sm:text-[2.4rem]">{data.clinic.name}</h1>
  </header>

  <section class="card page-in px-5 py-6 sm:px-8" aria-live="polite">
    {#if result === 'accepted' || data.status === 'booked'}
      <div class="grid h-12 w-12 place-items-center rounded-full bg-app-accent/14 text-app-accent"><Icon name="check" size={26} /></div>
      <h2 class="display mt-4 text-3xl leading-tight">Tu cita quedó agendada</h2>
      <p class="mt-3 text-app-muted">Te enviamos los detalles por correo, con un enlace para confirmarla, cambiarla o cancelarla.</p>
    {:else if result === 'declined'}
      <h2 class="display text-3xl leading-tight">Lo guardamos para alguien más</h2>
      <p class="mt-3 text-app-muted">Seguirás en la lista: si se libera otro lugar que te acomode, te avisaremos.</p>
      <button type="button" class="btn-secondary mt-5" onclick={() => respond('leave')} disabled={act.phase === 'loading'}>Mejor sácame de la lista</button>
    {:else if result === 'left' || data.status === 'cancelled'}
      <h2 class="display text-3xl leading-tight">Ya no estás en la lista</h2>
      <p class="mt-3 text-app-muted">No recibirás más avisos de lugares liberados.</p>
    {:else if data.status === 'expired'}
      <h2 class="display text-3xl leading-tight">Tu solicitud venció</h2>
      <p class="mt-3 text-app-muted">Pasó mucho tiempo sin lugar disponible. Puedes volver a anotarte desde la página de citas.</p>
    {:else if data.offer}
      <h2 class="display text-3xl leading-tight">Se liberó un lugar para ti</h2>
      <dl class="mt-5 grid gap-2 rounded-xl bg-app-elevated p-4 text-sm">
        <div><dt class="text-xs text-app-muted">Fecha</dt><dd class="font-semibold first-letter:uppercase">{dateLong(data.offer.date)}</dd></div>
        <div><dt class="text-xs text-app-muted">Hora</dt><dd class="font-semibold">{data.offer.start} h</dd></div>
        {#if data.offer.professional}<div><dt class="text-xs text-app-muted">Atiende</dt><dd class="font-semibold">{data.offer.professional}</dd></div>{/if}
        {#if data.clinic.address}<div><dt class="text-xs text-app-muted">Dirección</dt><dd class="font-semibold">{data.clinic.address}</dd></div>{/if}
      </dl>
      <p class="mt-4 text-sm text-app-muted">Lo guardamos para ti durante <span class="font-mono font-semibold text-app-ink" role="timer">{clock}</span> minutos más.</p>
      {#if act.phase === 'error'}<p class="alert mt-4" role="alert"><Icon name="alert" size={18} />{act.message}</p>{/if}
      <div class="mt-5 flex flex-wrap gap-2">
        <button type="button" class="btn-primary btn-lg" onclick={() => respond('accept')} disabled={act.phase === 'loading' || left === 0}>
          {#if act.phase === 'loading'}<span class="spin"></span>{/if}Aceptar este lugar
        </button>
        <button type="button" class="btn-secondary btn-lg" onclick={() => respond('decline')} disabled={act.phase === 'loading'}>No me acomoda</button>
      </div>
    {:else}
      <h2 class="display text-3xl leading-tight">{data.expired ? 'El tiempo para aceptar terminó' : `Hola${data.name ? ', ' + data.name : ''}, sigues en la lista`}</h2>
      <p class="mt-3 text-app-muted">
        {data.expired ? 'Ese lugar pasó a la siguiente persona. Seguirás en la lista por si se libera otro.' : 'Si se libera un lugar que te acomode, te avisaremos por correo.'}
      </p>
      {#if act.phase === 'error'}<p class="alert mt-4" role="alert"><Icon name="alert" size={18} />{act.message}</p>{/if}
      <button type="button" class="btn-secondary mt-5" onclick={() => respond('leave')} disabled={act.phase === 'loading'}>Salir de la lista de espera</button>
    {/if}
    {#if data.clinic.phone}<p class="mt-5 text-sm text-app-muted">¿Dudas? Llama al <a class="text-app-primary underline" href="tel:{data.clinic.phone}">{data.clinic.phone}</a>.</p>{/if}
  </section>
{/if}
