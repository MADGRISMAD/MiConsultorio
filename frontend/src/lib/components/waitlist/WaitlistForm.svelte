<script lang="ts">
  import Alert from '$lib/components/ui/Alert.svelte';
  import { untrack } from 'svelte';
  import { waitlistApi } from '$lib/api/waitlist';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { Professional } from '$lib/types/agenda';
  import type { ServiceDuration, WaitlistEntry, WaitlistInput } from '$lib/types/waitlist';
  import Modal from '$lib/components/Modal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';

  interface Props {
    open: boolean;
    /** Entry being edited; null to add someone. */
    entry: WaitlistEntry | null;
    pros: Professional[];
    services: ServiceDuration[];
    onsaved: (e: WaitlistEntry) => void;
    onclose: () => void;
  }
  let { open, entry, pros, services, onsaved, onclose }: Props = $props();

  const uid = $props.id();
  const op = new Op();
  // Monday first, as the agenda shows it; values are JS weekdays (0 = Sunday).
  const DAYS: [number, string][] = [[1, 'Lun'], [2, 'Mar'], [3, 'Mié'], [4, 'Jue'], [5, 'Vie'], [6, 'Sáb'], [0, 'Dom']];

  let name = $state('');
  let phone = $state('');
  let email = $state('');
  let professionalId = $state('');
  let serviceId = $state('');
  let days = $state<number[]>([]);
  let fromTime = $state('');
  let toTime = $state('');
  let notes = $state('');
  let consent = $state(false);
  let error = $state('');

  $effect(() => {
    if (!open) return;
    untrack(() => {
      const e = entry;
      name = e?.name ?? '';
      phone = e?.phone ?? '';
      email = e?.email ?? '';
      professionalId = e?.professional_id ?? '';
      serviceId = e?.service_id ?? '';
      days = [...(e?.days ?? [])];
      fromTime = e?.from_time ?? '';
      toTime = e?.to_time ?? '';
      notes = e?.notes ?? '';
      consent = e?.consent ?? false;
      error = '';
      op.reset();
    });
  });

  function toggleDay(d: number) {
    days = days.includes(d) ? days.filter((x) => x !== d) : [...days, d];
  }

  async function submit(ev: SubmitEvent) {
    ev.preventDefault();
    error = '';
    if (!name.trim()) return void (error = 'Escribe el nombre de la persona.');
    if (!phone.trim() && !email.trim()) return void (error = 'Escribe un teléfono o un correo para poder avisarle.');
    if (!!fromTime !== !!toTime) return void (error = 'Indica la hora de inicio y la de fin, o deja ambas vacías.');
    if (fromTime && toTime <= fromTime) return void (error = 'La hora de fin debe ser posterior a la de inicio.');
    const input: WaitlistInput = {
      patient_id: entry?.patient_id ?? null,
      name: name.trim(),
      phone: phone.trim(),
      email: email.trim(),
      professional_id: professionalId || null,
      service_id: serviceId || null,
      days,
      from_time: fromTime,
      to_time: toTime,
      notes: notes.trim(),
      consent
    };
    let saved: WaitlistEntry | undefined;
    const editing = entry;
    const ok = await op.run(async () => {
      saved = editing ? await waitlistApi.update(editing.id, input) : await waitlistApi.create(input);
    });
    if (!ok || !saved) return;
    toast.show(editing ? 'Cambios guardados' : 'Agregado a la lista de espera');
    onsaved(saved);
  }
</script>

<Modal {open} title={entry ? 'Editar lista de espera' : 'Agregar a la lista de espera'} {onclose}>
  <form id="{uid}-form" class="space-y-4" onsubmit={submit} novalidate>
    {#if error || op.phase === 'error'}<Alert>{error || op.message}</Alert>{/if}
    <div>
      <label class="label" for="{uid}-name">Nombre</label>
      <input id="{uid}-name" class="field" bind:value={name} maxlength="200" autocomplete="off" required />
    </div>
    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <label class="label" for="{uid}-phone">Teléfono</label>
        <input id="{uid}-phone" class="field" type="tel" inputmode="tel" bind:value={phone} autocomplete="off" placeholder="55 1234 5678" />
      </div>
      <div>
        <label class="label" for="{uid}-email">Correo</label>
        <input id="{uid}-email" class="field" type="email" bind:value={email} autocomplete="off" autocapitalize="none" />
      </div>
    </div>
    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <label class="label" for="{uid}-pro">Profesional</label>
        <select id="{uid}-pro" class="field" bind:value={professionalId}>
          <option value="">Cualquiera</option>
          {#each pros as p (p.id)}<option value={p.id}>{p.name}</option>{/each}
        </select>
      </div>
      <div>
        <label class="label" for="{uid}-svc">Servicio</label>
        <select id="{uid}-svc" class="field" bind:value={serviceId}>
          <option value="">Cualquiera</option>
          {#each services as s (s.id)}<option value={s.id}>{s.name}{s.duration_minutes ? ` · ${s.duration_minutes} min` : ''}</option>{/each}
        </select>
      </div>
    </div>
    <fieldset>
      <legend class="label">Días que le acomodan <span class="font-normal text-app-muted">(ninguno = cualquiera)</span></legend>
      <div class="flex flex-wrap gap-2">
        {#each DAYS as [d, label]}
          <label class="grid min-h-10 min-w-12 cursor-pointer place-items-center rounded-xl border border-app-ink/12 px-3 text-sm font-medium has-[:checked]:border-app-ink has-[:checked]:bg-app-ink has-[:checked]:text-app-surface has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-app-primary/60">
            <input type="checkbox" class="sr-only" checked={days.includes(d)} onchange={() => toggleDay(d)} />{label}
          </label>
        {/each}
      </div>
    </fieldset>
    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <label class="label" for="{uid}-from">Desde <span class="font-normal text-app-muted">(opcional)</span></label>
        <input id="{uid}-from" class="field" type="time" bind:value={fromTime} />
      </div>
      <div>
        <label class="label" for="{uid}-to">Hasta</label>
        <input id="{uid}-to" class="field" type="time" bind:value={toTime} />
      </div>
    </div>
    <div>
      <label class="label" for="{uid}-notes">Notas <span class="font-normal text-app-muted">(opcional)</span></label>
      <input id="{uid}-notes" class="field" bind:value={notes} maxlength="500" />
    </div>
    <label class="flex cursor-pointer items-start gap-3 text-sm">
      <input type="checkbox" class="mt-0.5 h-5 w-5 flex-none accent-[rgb(var(--app-primary))]" bind:checked={consent} />
      <span>La persona aceptó que le avisemos por correo cuando se libere un lugar.
        <span class="block text-xs text-app-muted">Sin su permiso o sin correo el sistema no le envía nada: solo te avisa a ti para que le llames.</span></span>
    </label>
  </form>
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    <button type="submit" form="{uid}-form" class="btn-primary" disabled={op.phase === 'loading'}>
      {#if op.phase === 'loading'}<span class="spin"></span>{/if}Guardar
    </button>
  {/snippet}
</Modal>
