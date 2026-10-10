<script lang="ts">
  import { session } from '$lib/session.svelte';
  import { publicOrigin } from '$lib/site';
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { agendaApi } from '$lib/api/agenda';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import { PRO_COLORS, type AgendaSettings, type DayKey, type HoursMap, type Professional } from '$lib/types/agenda';
  import Icon from '../ui/Icon.svelte';
  import LoadingRows from '../ui/LoadingRows.svelte';

  const DAYS: [DayKey, string][] = [['mon', 'Lunes'], ['tue', 'Martes'], ['wed', 'Miércoles'], ['thu', 'Jueves'], ['fri', 'Viernes'], ['sat', 'Sábado'], ['sun', 'Domingo']];
  const SLOTS = [10, 15, 20, 30, 45, 60];

  let s = $state<AgendaSettings | null>(null);
  let pros = $state<Professional[]>([]);
  let loadError = $state('');
  let newRoom = $state('');
  let hoursText = $state('24, 2');
  const saveOp = new Op();

  onMount(async () => {
    try {
      const [st, p] = await Promise.all([agendaApi.settings(), agendaApi.professionals()]);
      s = st;
      pros = p;
      hoursText = st.remind_hours.join(', ');
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'No se pudieron cargar los ajustes.';
    }
  });

  const origin = publicOrigin();
  const slugPreview = $derived((s?.booking_slug ?? '').trim().toLowerCase());
  const slugOk = $derived(/^[a-z0-9][a-z0-9-]{1,62}$/.test(slugPreview));

  function addRoom() {
    const r = newRoom.trim();
    if (s && r && !s.rooms.includes(r)) s.rooms.push(r);
    newRoom = '';
  }

  async function save(e: SubmitEvent) {
    e.preventDefault();
    if (!s) return;
    const hours = hoursText.split(/[,\s]+/).filter(Boolean).map(Number);
    if (hours.some((h) => !Number.isInteger(h))) {
      saveOp.fail('Escribe las horas de los recordatorios como números separados por comas, por ejemplo 24, 2.');
      return;
    }
    const body = { ...$state.snapshot(s), remind_hours: hours };
    if (await saveOp.run(async () => {
      s = await agendaApi.saveSettings(body);
      hoursText = s.remind_hours.join(', ');
    })) toast.show('Ajustes de agenda guardados');
  }

  // ----- professionals -----
  const proOp = new Op();
  let savingPro = $state('');
  async function savePro(p: Professional) {
    savingPro = p.id;
    if (await proOp.run(async () => {
      const r = await agendaApi.saveProfessional(p.id, { bookable: p.bookable, consults: p.consults, video_url: p.video_url ?? '', slot_minutes: p.slot_minutes, hours: $state.snapshot(p.hours) as HoursMap, color: p.color });
      Object.assign(p, r);
    })) toast.show(`Agenda de ${p.name} guardada`);
    savingPro = '';
  }
  const ranges = (p: Professional, d: DayKey) => (p.hours[d] ??= []);
  function addRange(p: Professional, d: DayKey) {
    ranges(p, d).push(['09:00', '14:00']);
  }
  function removeRange(p: Professional, d: DayKey, i: number) {
    ranges(p, d).splice(i, 1);
  }
  const colorOf = (p: Professional, i: number) => p.color || PRO_COLORS[i % PRO_COLORS.length];
</script>

{#if loadError}
  <Alert>{loadError}</Alert>
{:else if !s}
  <LoadingRows />
{:else}
  <form onsubmit={save} class="grid gap-10">
    <section aria-labelledby="ag-general">
      <h3 id="ag-general" class="section-title mb-3">Citas y salas</h3>
      <div class="grid gap-4">
        <div>
          <span class="label">Intervalo de citas</span>
          <p class="hint mb-2 !mt-0">Tamaño de cada hueco de la agenda y duración que se propone al agendar.</p>
          <div class="flex flex-wrap items-center gap-2">
            {#each SLOTS as m}
              <button type="button" aria-pressed={s.slot_minutes === m} class="rounded-full px-4 py-1.5 text-sm font-medium transition {s.slot_minutes === m ? 'bg-app-ink text-app-surface' : 'bg-app-ink/6 text-app-muted hover:bg-app-ink/10 hover:text-app-ink'}" onclick={() => s && (s.slot_minutes = m)}>{m} min</button>
            {/each}
          </div>
        </div>
        <div>
          <label class="label" for="ag-room">Salas o consultorios</label>
          <p class="hint mb-2 !mt-0">Una sala no puede tener dos citas al mismo tiempo.</p>
          <div class="mb-2 flex flex-wrap gap-2">
            {#each s.rooms as r, i (r)}
              <span class="pill !py-1">{r}<button type="button" class="ml-1 rounded-full hover:text-app-danger" aria-label="Quitar la sala {r}" onclick={() => s?.rooms.splice(i, 1)}><Icon name="x" size={14} /></button></span>
            {/each}
            {#if s.rooms.length === 0}<span class="text-sm text-app-muted">Sin salas: las citas no llevan sala.</span>{/if}
          </div>
          <div class="flex gap-2">
            <input id="ag-room" type="text" class="field" maxlength="40" placeholder="Consultorio 1" bind:value={newRoom} onkeydown={(e) => e.key === 'Enter' && (e.preventDefault(), addRoom())} />
            <button type="button" class="btn-secondary" onclick={addRoom}>Agregar</button>
          </div>
        </div>
      </div>
    </section>

    <section aria-labelledby="ag-book">
      <h3 id="ag-book" class="section-title mb-3">Reserva en línea</h3>
      <label class="mb-4 flex cursor-pointer items-center gap-3 text-sm font-medium">
        <input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={s.booking_enabled} />
        Permitir que los pacientes reserven su cita desde una página pública
      </label>
      <div class="grid gap-4 sm:grid-cols-2">
        <div class="sm:col-span-2">
          <label class="label" for="ag-slug">Dirección de tu página de reservas</label>
          <input id="ag-slug" type="text" class="field" bind:value={s.booking_slug} maxlength="63" placeholder="mi-consultorio" autocomplete="off" aria-invalid={slugPreview !== '' && !slugOk} />
          <p class="hint">
            {#if slugPreview && slugOk}Tus pacientes entrarán a <strong class="break-all text-app-ink">{origin}/reservar/{slugPreview}</strong>
            {:else}Letras minúsculas, números y guiones (2 a 63 caracteres).{/if}
          </p>
        </div>
        <label class="block">
          <span class="label">Anticipación mínima (horas)</span>
          <input type="number" min="0" max="720" class="field" bind:value={s.booking_lead_hours} />
        </label>
        <label class="block">
          <span class="label">Reservas hasta dentro de (días)</span>
          <input type="number" min="1" max="365" class="field" bind:value={s.booking_horizon_days} />
        </label>
        <label class="block sm:col-span-2">
          <span class="label">Mensaje para quien reserva</span>
          <textarea class="field min-h-20" maxlength="500" bind:value={s.booking_message} placeholder="Llega 10 minutos antes. Si necesitas cancelar, avísanos con tiempo."></textarea>
        </label>
        <label class="flex cursor-pointer items-start gap-3 text-sm sm:col-span-2">
          <input type="checkbox" class="mt-0.5 h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={s.booking_show_prices} />
          <span><span class="font-medium">Mostrar precios de los servicios</span><span class="block text-app-muted">Los precios del catálogo aparecen en la página pública.</span></span>
        </label>
        <label class="block">
          <span class="label">Cancelar hasta (horas antes)</span>
          <input type="number" min="0" max="720" class="field" bind:value={s.cancel_min_hours} />
          <p class="hint">El paciente puede cancelar desde su enlace hasta esta anticipación.</p>
        </label>
        <label class="flex cursor-pointer items-start gap-3 text-sm sm:col-span-2">
          <input type="checkbox" class="mt-0.5 h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={s.booking_requires_confirmation} />
          <span><span class="font-medium">Confirmar manualmente cada reserva</span><span class="block text-app-muted">La cita queda como programada hasta que recepción la confirme.</span></span>
        </label>
      </div>
    </section>

    <section aria-labelledby="ag-remind">
      <h3 id="ag-remind" class="section-title mb-3">Recordatorios</h3>
      <p class="hint mb-3 !mt-0">Solo se envían a pacientes que aceptaron recibir recordatorios y que tienen correo o teléfono.</p>
      <div class="grid gap-4 sm:grid-cols-2">
        <label class="flex cursor-pointer items-center gap-3 text-sm font-medium"><input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={s.remind_email} />Por correo</label>
        <label class="flex cursor-pointer items-center gap-3 text-sm font-medium {session.whatsapp || s.remind_whatsapp ? '' : 'opacity-60'}"><input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={s.remind_whatsapp} disabled={!session.whatsapp && !s.remind_whatsapp} />Por WhatsApp{#if !session.whatsapp} <span class="badge">Desde Crecimiento</span>{/if}</label>
        <div class="sm:col-span-2">
          <label class="label" for="ag-hours">Cuántas horas antes de la cita</label>
          <input id="ag-hours" type="text" class="field" bind:value={hoursText} inputmode="numeric" placeholder="24, 2" />
          <p class="hint">Hasta 3 valores entre 1 y 168 horas, separados por comas. Por ejemplo, «24, 2» avisa un día y dos horas antes.</p>
        </div>
        <label class="block sm:col-span-2">
          <span class="label">Texto del recordatorio <span class="font-normal text-app-muted">(opcional)</span></span>
          <textarea class="field min-h-24" maxlength="1000" bind:value={s.reminder_template} placeholder={'Hola {{nombre}}, te recordamos tu cita el {{fecha}} a las {{hora}}.'}></textarea>
        </label>
      </div>
    </section>

    <OpError op={saveOp} />
    <div class="flex justify-end">
      <button type="submit" class="btn-primary" disabled={saveOp.phase === 'loading'}>{#if saveOp.phase === 'loading'}<span class="spin"></span>{/if}Guardar ajustes</button>
    </div>
  </form>

  <section aria-labelledby="ag-pros">
    <h3 id="ag-pros" class="section-title mb-1">Agenda de cada profesional</h3>
    <p class="hint mb-4 !mt-0">Si dejas los horarios vacíos se usa el horario del consultorio.</p>
    {#if pros.length === 0}
      <p class="text-sm text-app-muted">Aún no hay médicos ni administradores activos.</p>
    {/if}
    <div class="grid gap-4">
      {#each pros as p, i (p.id)}
        <details class="card !rounded-2xl p-4" open={pros.length === 1}>
          <summary class="flex cursor-pointer items-center gap-3 font-semibold">
            <span class="h-3.5 w-3.5 rounded-full" style="background:{colorOf(p, i)}"></span>{p.name}
            {#if p.bookable}<span class="badge">En línea</span>{/if}
          </summary>
          <div class="mt-4 grid gap-4">
            <div>
              <label class="flex cursor-pointer items-center gap-3 text-sm font-medium">
                <input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={p.consults} onchange={() => { if (!p.consults) p.bookable = false; }} />Atiendo consultas
              </label>
              <p class="hint !mt-1">Desmárcalo si eres solo el dueño o administrador y no atiendes pacientes: así no te agendan citas ni apareces en la agenda.</p>
            </div>
            <label class="flex items-center gap-3 text-sm font-medium {p.consults ? 'cursor-pointer' : 'opacity-50'}">
              <input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={p.bookable} disabled={!p.consults} />Aparece en la reserva en línea
            </label>
            <label class="block">
              <span class="label">Enlace de videollamada <span class="font-normal text-app-muted">(opcional)</span></span>
              <input type="url" class="field" bind:value={p.video_url} maxlength="300" placeholder="https://meet.google.com/..." />
              <span class="hint">Para consultas en línea: el enlace sale en los correos de la cita del paciente.</span>
            </label>
            <div class="grid gap-4 sm:grid-cols-2">
              <label class="block">
                <span class="label">Intervalo de citas (min)</span>
                <input type="number" min="5" max="480" step="5" class="field" bind:value={p.slot_minutes} />
              </label>
              <div>
                <span class="label" id="color-{p.id}">Color en la agenda</span>
                <div class="flex flex-wrap items-center gap-2" role="group" aria-labelledby="color-{p.id}">
                  {#each PRO_COLORS as c}
                    <button type="button" class="h-7 w-7 rounded-full ring-offset-2 ring-offset-app-panel transition {colorOf(p, i) === c ? 'ring-2 ring-app-ink' : ''}" style="background:{c}" aria-label="Color {c}" aria-pressed={colorOf(p, i) === c} onclick={() => (p.color = c)}></button>
                  {/each}
                </div>
              </div>
            </div>
            <fieldset>
              <legend class="label">Horario semanal</legend>
              <ul class="divide-y divide-app-ink/8 overflow-hidden rounded-xl border border-app-ink/10">
                {#each DAYS as [d, label]}
                  <li class="flex flex-wrap items-center gap-x-4 gap-y-2 px-3.5 py-2.5 text-sm">
                    <span class="w-full font-medium sm:w-24">{label}</span>
                    <div class="flex flex-1 flex-wrap items-center gap-2">
                      {#each p.hours[d] ?? [] as r, ri}
                        <span class="flex w-full items-center gap-1.5 sm:w-auto">
                          <input type="time" step="900" class="field !min-h-9 !w-auto min-w-0 flex-1 !py-1 sm:flex-none" bind:value={r[0]} aria-label="{label}, inicio del tramo {ri + 1}" />
                          <span class="text-app-muted">a</span>
                          <input type="time" step="900" class="field !min-h-9 !w-auto min-w-0 flex-1 !py-1 sm:flex-none" bind:value={r[1]} aria-label="{label}, fin del tramo {ri + 1}" />
                          <button type="button" class="icon-btn danger !h-8 !w-8" aria-label="Quitar el tramo {ri + 1} del {label}" onclick={() => removeRange(p, d, ri)}><Icon name="x" size={16} /></button>
                        </span>
                      {/each}
                      {#if (p.hours[d]?.length ?? 0) === 0}<span class="text-app-muted">Horario del consultorio</span>{/if}
                      {#if (p.hours[d]?.length ?? 0) < 4}
                        <button type="button" class="btn-ghost !min-h-8 !px-3 text-[13px]" onclick={() => addRange(p, d)}><Icon name="plus" size={15} />Tramo</button>
                      {/if}
                    </div>
                  </li>
                {/each}
              </ul>
            </fieldset>
            {#if proOp.phase === 'error' && savingPro === ''}<p class="alert" role="alert">{proOp.message}</p>{/if}
            <div class="flex justify-end">
              <button type="button" class="btn-primary" disabled={savingPro === p.id} onclick={() => savePro(p)}>{#if savingPro === p.id}<span class="spin"></span>{/if}Guardar a {p.name}</button>
            </div>
          </div>
        </details>
      {/each}
    </div>
  </section>
{/if}
