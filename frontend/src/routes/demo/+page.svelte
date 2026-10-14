<script lang="ts">
  import { theme } from '$lib/theme.svelte';
  import { request, ApiError } from '$lib/api';
  import Footer from '$lib/landing/Footer.svelte';
  import Nav from '$lib/landing/Nav.svelte';
  import { contactEmail, demoMailto } from '$lib/landing/data';

  $effect(() => theme.init());

  const SPECIALTIES = ['Odontología', 'Medicina general', 'Nutrición', 'Psicología', 'Fisioterapia', 'Pediatría', 'Ginecología', 'Veterinaria', 'Otra'];

  let name = $state('');
  let email = $state('');
  let phone = $state('');
  let clinic = $state('');
  let specialty = $state('');
  let message = $state('');
  let website = $state(''); // trampa para robots: oculto, una persona nunca lo llena
  let status = $state<'idle' | 'sending' | 'done' | 'error'>('idle');
  let error = $state('');

  const ready = $derived(name.trim().length > 0 && /^\S+@\S+\.\S+$/.test(email.trim()));

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!ready || status === 'sending') return;
    status = 'sending';
    error = '';
    try {
      await request<void>('POST', '/public/demo', { name, email, phone, clinic, specialty, message, website });
      status = 'done';
    } catch (err) {
      error = err instanceof ApiError ? err.message : 'No se pudo enviar. Inténtalo de nuevo.';
      status = 'error';
    }
  }

  const field =
    'mt-2 w-full rounded-2xl bg-paper px-4 py-3.5 text-[16px] text-ink ring-1 ring-ink/15 transition placeholder:text-ink-faint focus:outline-none focus:ring-2 focus:ring-signal';
  const label = 'block font-mono text-[11px] uppercase tracking-[0.14em] text-ink-soft';
</script>

<svelte:head>
  <title>Solicitar demo · Caresia</title>
  <meta name="description" content="Pide una demostración de Caresia: te mostramos cómo se ve en tu consultorio, sin compromiso." />
  <link rel="canonical" href="https://caresia.mx/demo" />
  <meta property="og:title" content="Solicitar demo · Caresia" />
  <meta property="og:description" content="Pide una demostración de Caresia: te mostramos cómo se ve en tu consultorio, sin compromiso." />
  <meta property="og:url" content="https://caresia.mx/demo" />
  <meta property="og:site_name" content="Caresia" />
</svelte:head>

<div class="landing grain min-h-screen overflow-x-clip bg-paper font-body text-ink antialiased selection:bg-signal selection:text-white" data-theme={theme.mode}>
  <Nav home="/" />

  <main class="mx-auto max-w-6xl px-5 pb-28 pt-32 sm:px-8 sm:pt-40">
    <div class="grid gap-14 lg:grid-cols-[1fr_1.1fr] lg:gap-24">
      <div>
        <p class="flex items-center gap-2 font-mono text-[11px] uppercase tracking-[0.16em] text-ink-soft"><span class="h-1.5 w-1.5 rounded-full bg-signal"></span>Demo</p>
        <h1 class="mt-6 font-display text-[clamp(3rem,9vw,6rem)] font-normal leading-[0.92] tracking-[-0.035em]">
          Mira Caresia <em class="italic text-signal">en tu consultorio.</em>
        </h1>
        <p class="mt-6 max-w-md text-lg leading-relaxed text-ink-soft [text-wrap:pretty]">
          Cuéntanos de tu consultorio y te mostramos cómo se vería con tu agenda, tus expedientes y tus cobros. Es una conversación corta y sin compromiso.
        </p>
        <ul class="mt-8 space-y-3 text-[15px] text-ink-soft">
          <li class="flex gap-3"><span class="mt-2 h-1.5 w-1.5 flex-none rounded-full bg-signal"></span>Te escribimos al correo que dejes aquí.</li>
          <li class="flex gap-3"><span class="mt-2 h-1.5 w-1.5 flex-none rounded-full bg-signal"></span>Vemos tus necesidades antes de enseñarte nada.</li>
          <li class="flex gap-3"><span class="mt-2 h-1.5 w-1.5 flex-none rounded-full bg-signal"></span>Sin tarjeta y sin compromiso.</li>
        </ul>
      </div>

      <div class="rounded-[28px] bg-panel p-6 ring-1 ring-ink/10 sm:p-9">
        {#if status === 'done'}
          <div role="status" class="py-8 text-center">
            <p class="font-display text-4xl leading-tight">¡Gracias, {name.trim().split(' ')[0]}!</p>
            <p class="mx-auto mt-4 max-w-sm text-ink-soft">Recibimos tu solicitud y te escribimos pronto a <strong class="font-semibold text-ink">{email.trim()}</strong>.</p>
            <a href="/" class="mt-8 inline-flex h-12 items-center rounded-full bg-ink px-7 text-[15px] font-medium text-paper transition-colors hover:bg-signal">Volver al inicio</a>
          </div>
        {:else}
          <form onsubmit={submit} novalidate class="space-y-5" aria-label="Solicitar una demostración">
            <div>
              <label class={label} for="d-name">Tu nombre</label>
              <input id="d-name" class={field} bind:value={name} autocomplete="name" maxlength="120" required />
            </div>
            <div>
              <label class={label} for="d-email">Correo</label>
              <input id="d-email" class={field} type="email" bind:value={email} autocomplete="email" inputmode="email" maxlength="200" required />
            </div>
            <div class="grid gap-5 sm:grid-cols-2">
              <div>
                <label class={label} for="d-phone">Teléfono <span class="normal-case tracking-normal text-ink-faint">(opcional)</span></label>
                <input id="d-phone" class={field} type="tel" bind:value={phone} autocomplete="tel" inputmode="tel" maxlength="40" />
              </div>
              <div>
                <label class={label} for="d-specialty">Especialidad</label>
                <select id="d-specialty" class={field} bind:value={specialty}>
                  <option value="">Elige una</option>
                  {#each SPECIALTIES as s (s)}<option value={s}>{s}</option>{/each}
                </select>
              </div>
            </div>
            <div>
              <label class={label} for="d-clinic">Nombre de tu consultorio <span class="normal-case tracking-normal text-ink-faint">(opcional)</span></label>
              <input id="d-clinic" class={field} bind:value={clinic} autocomplete="organization" maxlength="160" />
            </div>
            <div>
              <label class={label} for="d-message">¿Qué te gustaría resolver? <span class="normal-case tracking-normal text-ink-faint">(opcional)</span></label>
              <textarea id="d-message" class="{field} min-h-28 resize-y" bind:value={message} maxlength="1500"></textarea>
            </div>

            <!-- trampa para robots: fuera de la vista y del orden de tabulación -->
            <div aria-hidden="true" class="absolute -left-[9999px] h-0 w-0 overflow-hidden">
              <label>No lo llenes <input tabindex="-1" autocomplete="off" bind:value={website} /></label>
            </div>

            {#if status === 'error'}
              <p role="alert" class="rounded-2xl bg-[#b42318]/10 px-4 py-3 text-[15px] text-[#b42318]">
                {error} Si prefieres, escríbenos a <a class="underline" href={demoMailto}>{contactEmail}</a>.
              </p>
            {/if}

            <button
              type="submit"
              disabled={!ready || status === 'sending'}
              class="inline-flex h-14 w-full items-center justify-center rounded-full bg-ink text-[16px] font-medium text-paper transition-[background-color,transform] duration-200 hover:bg-signal active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-50"
            >
              {status === 'sending' ? 'Enviando…' : 'Solicitar demo'}
            </button>
            <p class="text-center text-[13px] leading-relaxed text-ink-soft">
              Usamos tus datos solo para responderte. Consulta el <a href="/privacidad" class="underline underline-offset-2">aviso de privacidad</a>.
            </p>
          </form>
        {/if}
      </div>
    </div>
  </main>

  <Footer home="/" />
</div>
