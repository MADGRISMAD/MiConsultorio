<script lang="ts">
  import Alert from '$lib/components/ui/Alert.svelte';
  import { goto } from '$app/navigation';
  import { session } from '$lib/session.svelte';
  import { Op } from '$lib/op.svelte';
  import AuthLayout from '$lib/components/AuthLayout.svelte';
  import Spinner from '$lib/components/Spinner.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';

  let clinicName = $state('');
  let phone = $state('');
  let email = $state('');
  let personName = $state('');
  let username = $state('');
  let password = $state('');
  let confirm = $state('');
  let showPass = $state(false);
  let accepted = $state(false);
  let touched = $state(false);
  const op = new Op();

  $effect(() => {
    if (session.status === 'authenticated') goto(session.home, { replaceState: true });
  });

  // 0..3 — length, mixed case/digits, symbols or long
  const strength = $derived.by(() => {
    let s = 0;
    if (password.length >= 8) s++;
    if (/[a-z]/.test(password) && /[A-Z]/.test(password) && /\d/.test(password)) s++;
    if (password.length >= 12 || /[^A-Za-z0-9]/.test(password)) s++;
    return password ? Math.max(1, s) : 0;
  });
  const strengthLabel = ['', 'Débil', 'Aceptable', 'Fuerte'];

  const problems = $derived({
    clinicName: clinicName.trim().length < 2 ? 'Escribe el nombre de tu consultorio.' : '',
    email: !/^\S+@\S+\.\S+$/.test(email.trim()) ? 'Escribe un correo válido.' : '',
    personName: personName.trim().length < 2 ? 'Escribe tu nombre.' : '',
    username: !/^[A-Za-z0-9._-]{3,64}$/.test(username.trim()) ? 'Mínimo 3 caracteres: letras, números, punto, guion o guion bajo.' : '',
    password: password.length < 8 ? 'Mínimo 8 caracteres.' : '',
    confirm: confirm !== password ? 'Las contraseñas no coinciden.' : '',
    accepted: !accepted ? 'Debes aceptar los términos.' : ''
  });
  const valid = $derived(Object.values(problems).every((p) => !p));

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    touched = true;
    if (!valid) {
      op.fail('Revisa los campos marcados.');
      return;
    }
    await op.run(() =>
      session.register({ clinic_name: clinicName.trim(), phone: phone.trim(), name: personName.trim(), email: email.trim(), username: username.trim(), password })
    );
  }
</script>

<svelte:head><title>Crear mi consultorio · Caresia</title></svelte:head>

{#snippet err(key: keyof typeof problems)}
  {#if touched && problems[key]}<p class="mt-1 text-[13px] font-semibold text-app-danger">{problems[key]}</p>{/if}
{/snippet}

{#if session.status === 'loading'}
  <Spinner />
{:else}
  <AuthLayout wide>
    <header class="mb-4">
      <h1 class="display text-[2.2rem] leading-none">Crea tu <em class="italic text-app-primary">consultorio</em></h1>
      <p class="mt-2 text-sm text-app-muted">Toma menos de un minuto. Después te ayudamos a configurar todo lo demás.</p>
    </header>

    <form class="grid gap-3.5" novalidate onsubmit={submit}>
      {#if op.phase === 'error'}
        <Alert>{op.message}</Alert>
      {/if}

      <div class="grid gap-3.5 sm:grid-cols-2">
        <div>
          <label class="label" for="r-name">Nombre del consultorio</label>
          <input id="r-name" class="field" bind:value={clinicName} placeholder="Ej. Clínica Sol" autocomplete="organization" aria-invalid={touched && !!problems.clinicName} />
          {@render err('clinicName')}
        </div>
        <div>
          <label class="label" for="r-phone">Celular <span class="font-normal text-app-muted">(opcional)</span></label>
          <input id="r-phone" class="field" type="tel" bind:value={phone} placeholder="55 1234 5678" autocomplete="tel" />
        </div>
      </div>

      <div class="grid gap-3.5 border-t border-app-ink/10 pt-3.5 sm:grid-cols-2">
        <div class="sm:col-span-2">
          <label class="label" for="r-person">Tu nombre <span class="font-normal text-app-muted">(serás el administrador)</span></label>
          <input id="r-person" class="field" bind:value={personName} placeholder="Nombre y apellido" autocomplete="name" aria-invalid={touched && !!problems.personName} />
          {@render err('personName')}
        </div>
        <div>
          <label class="label" for="r-email">Correo electrónico</label>
          <input id="r-email" class="field" type="email" bind:value={email} placeholder="consultorio@correo.com" autocomplete="email" autocapitalize="none" aria-invalid={touched && !!problems.email} />
          {@render err('email')}
        </div>
        <div>
          <label class="label" for="r-user">Usuario</label>
          <input id="r-user" class="field" bind:value={username} placeholder="admin" autocomplete="username" autocapitalize="none" aria-invalid={touched && !!problems.username} />
          {@render err('username')}
        </div>
        <div>
          <label class="label" for="r-pass">Contraseña</label>
          <div class="relative">
            <input id="r-pass" class="field pr-12" type={showPass ? 'text' : 'password'} bind:value={password} placeholder="Mínimo 8 caracteres" autocomplete="new-password" maxlength="72" aria-invalid={touched && !!problems.password} />
            <button type="button" class="icon-btn absolute right-1 top-1/2 -translate-y-1/2" aria-label={showPass ? 'Ocultar contraseña' : 'Mostrar contraseña'} aria-pressed={showPass} onclick={() => (showPass = !showPass)}>
              <Icon name={showPass ? 'eye-off' : 'eye'} size={20} />
            </button>
          </div>
          {#if password}
            <div class="mt-2 grid grid-cols-[repeat(3,1fr)_auto] items-center gap-1.5" aria-label="Seguridad de la contraseña: {strengthLabel[strength]}">
              {#each [1, 2, 3] as bar}
                <i class="h-1.5 rounded-full {strength >= bar ? (strength === 1 ? 'bg-app-danger' : strength === 2 ? 'bg-app-warning' : 'bg-app-success') : 'bg-app-ink/12'}"></i>
              {/each}
              <span class="ml-1 text-xs font-semibold text-app-muted">{strengthLabel[strength]}</span>
            </div>
          {/if}
          {@render err('password')}
        </div>
        <div>
          <label class="label" for="r-confirm">Confirmar contraseña</label>
          <input id="r-confirm" class="field" type={showPass ? 'text' : 'password'} bind:value={confirm} autocomplete="new-password" maxlength="72" aria-invalid={touched && !!problems.confirm} />
          {@render err('confirm')}
        </div>
      </div>

      <div>
        <label class="flex cursor-pointer items-start gap-2.5 text-sm text-app-muted">
          <input type="checkbox" class="mt-0.5 h-[1.15rem] w-[1.15rem] flex-none accent-[rgb(var(--app-primary))]" bind:checked={accepted} />
          <span>Acepto los <a href="/terminos" target="_blank" rel="noopener" class="font-semibold text-app-primary hover:underline">términos</a> y el <a href="/privacidad" target="_blank" rel="noopener" class="font-semibold text-app-primary hover:underline">aviso de privacidad</a>.</span>
        </label>
        {@render err('accepted')}
      </div>

      <button type="submit" class="btn-primary btn-lg" disabled={op.phase === 'loading'}>
        {#if op.phase === 'loading'}<span class="spin"></span>Creando…{:else}Crear mi consultorio{/if}
      </button>
    </form>

    <p class="mt-4 border-t border-app-ink/10 pt-4 text-center text-sm text-app-muted">
      ¿Ya tienes cuenta? <a href="/login" class="font-semibold text-app-primary hover:underline">Inicia sesión</a> · <a href="/" class="font-semibold hover:text-app-ink">Volver al inicio</a>
    </p>
  </AuthLayout>
{/if}
