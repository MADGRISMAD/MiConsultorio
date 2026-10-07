<script lang="ts">
  import { waitlistApi } from '$lib/api/waitlist';
  import { Op } from '$lib/op.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';

  interface Props {
    slug: string;
    clinicName: string;
    professionalId: string;
    serviceId: string;
  }
  let { slug, clinicName, professionalId, serviceId }: Props = $props();

  const uid = $props.id();
  const DAYS: [number, string][] = [[1, 'Lun'], [2, 'Mar'], [3, 'Mié'], [4, 'Jue'], [5, 'Vie'], [6, 'Sáb'], [0, 'Dom']];

  let open = $state(false);
  let done = $state(false);
  let names = $state('');
  let lastNames = $state('');
  let email = $state('');
  let phone = $state('');
  let days = $state<number[]>([]);
  let fromTime = $state('');
  let toTime = $state('');
  let acceptPrivacy = $state(false);
  let acceptNotices = $state(false);
  let website = $state('');
  const op = new Op();

  const toggle = (d: number) => (days = days.includes(d) ? days.filter((x) => x !== d) : [...days, d]);

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!names.trim() || !lastNames.trim()) return op.fail('Escribe tu nombre y apellidos.');
    if (!email.trim()) return op.fail('Escribe tu correo: ahí te avisaremos si se libera un lugar.');
    if (!!fromTime !== !!toTime) return op.fail('Indica la hora de inicio y la de fin, o deja ambas vacías.');
    if (fromTime && toTime <= fromTime) return op.fail('La hora de fin debe ser posterior a la de inicio.');
    if (!acceptPrivacy) return op.fail('Debes aceptar el aviso de privacidad para anotarte.');
    if (!acceptNotices) return op.fail('Necesitamos tu permiso para avisarte por correo.');
    const ok = await op.run(() =>
      waitlistApi.join(slug, {
        names: names.trim(), last_names: lastNames.trim(), email: email.trim(), phone: phone.trim(),
        professional_id: professionalId || undefined, service_id: serviceId || undefined,
        days, from_time: fromTime, to_time: toTime, notes: '', accept_privacy: acceptPrivacy, accept_notices: acceptNotices, website
      })
    );
    if (ok) done = true;
  }
</script>

<div class="mt-4 rounded-2xl border border-app-primary/25 bg-app-primary/6 p-4 sm:p-5" aria-live="polite">
  {#if done}
    <p class="flex items-start gap-2 text-sm"><Icon name="check" size={18} class="mt-0.5 flex-none text-app-accent" /><span><strong>Listo, estás en la lista.</strong> Te avisaremos por correo si se libera un lugar que te acomode. Te enviamos un mensaje con un enlace para salir de la lista cuando quieras.</span></p>
  {:else if !open}
    <p class="text-sm font-medium">¿Ningún horario te acomoda?</p>
    <p class="mt-1 text-sm text-app-muted">Anótate en la lista de espera y te avisamos si se libera un lugar.</p>
    <button type="button" class="btn-secondary mt-3" onclick={() => (open = true)}><Icon name="clock-plus" size={18} />Avísame si se libera un lugar</button>
  {:else}
    <form class="grid gap-4" novalidate onsubmit={submit}>
      <p class="text-sm font-medium">Lista de espera de {clinicName}</p>
      {#if op.phase === 'error'}<p class="alert" role="alert"><Icon name="alert" size={18} />{op.message}</p>{/if}
      <div class="grid gap-4 sm:grid-cols-2">
        <div><label class="label" for="{uid}-n">Nombre(s)</label><input id="{uid}-n" class="field" bind:value={names} autocomplete="given-name" maxlength="100" required /></div>
        <div><label class="label" for="{uid}-l">Apellidos</label><input id="{uid}-l" class="field" bind:value={lastNames} autocomplete="family-name" maxlength="100" required /></div>
        <div><label class="label" for="{uid}-e">Correo</label><input id="{uid}-e" class="field" type="email" bind:value={email} autocomplete="email" autocapitalize="none" required /></div>
        <div><label class="label" for="{uid}-p">Teléfono <span class="font-normal text-app-muted">(opcional)</span></label><input id="{uid}-p" class="field" type="tel" inputmode="tel" bind:value={phone} autocomplete="tel" /></div>
      </div>
      <fieldset>
        <legend class="label">¿Qué días te acomodan? <span class="font-normal text-app-muted">(ninguno = cualquiera)</span></legend>
        <div class="flex flex-wrap gap-2">
          {#each DAYS as [d, label]}
            <label class="grid min-h-10 min-w-12 cursor-pointer place-items-center rounded-xl border border-app-ink/12 px-3 text-sm font-medium has-[:checked]:border-app-ink has-[:checked]:bg-app-ink has-[:checked]:text-app-surface has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-app-primary/60">
              <input type="checkbox" class="sr-only" checked={days.includes(d)} onchange={() => toggle(d)} />{label}
            </label>
          {/each}
        </div>
      </fieldset>
      <div class="grid grid-cols-2 gap-4">
        <div><label class="label" for="{uid}-f">Desde <span class="font-normal text-app-muted">(opcional)</span></label><input id="{uid}-f" class="field" type="time" bind:value={fromTime} /></div>
        <div><label class="label" for="{uid}-t">Hasta</label><input id="{uid}-t" class="field" type="time" bind:value={toTime} /></div>
      </div>
      <div class="absolute -left-[9999px]" aria-hidden="true">
        <label for="{uid}-w">Sitio web</label><input id="{uid}-w" tabindex="-1" autocomplete="off" bind:value={website} />
      </div>
      <div class="grid gap-3 text-sm">
        <label class="flex cursor-pointer items-start gap-3">
          <input type="checkbox" class="mt-0.5 h-5 w-5 flex-none accent-[rgb(var(--app-primary))]" bind:checked={acceptPrivacy} required />
          <span>He leído y acepto el <a href="/privacidad" target="_blank" rel="noopener" class="text-app-primary underline">aviso de privacidad</a>.</span>
        </label>
        <label class="flex cursor-pointer items-start gap-3">
          <input type="checkbox" class="mt-0.5 h-5 w-5 flex-none accent-[rgb(var(--app-primary))]" bind:checked={acceptNotices} required />
          <span>Acepto que {clinicName} me avise por correo cuando se libere un lugar. Puedo salir de la lista cuando quiera.</span>
        </label>
      </div>
      <div class="flex flex-wrap gap-2">
        <button class="btn-primary" type="submit" disabled={op.phase === 'loading'}>{#if op.phase === 'loading'}<span class="spin"></span>{/if}Anotarme en la lista</button>
        <button class="btn-secondary" type="button" onclick={() => (open = false)}>Cancelar</button>
      </div>
    </form>
  {/if}
</div>
