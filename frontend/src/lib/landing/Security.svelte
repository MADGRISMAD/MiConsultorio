<script lang="ts">
  import Icon from './Icon.svelte';
  import Split from './Split.svelte';
  import { parallax, reveal } from './motion';
  import Permisos from './screens/Permisos.svelte';

  // Solo lo verificable en el código: permisos por rol y por persona, 2FA, registro de quién abre cada expediente (record_access),
  // cifrado AES-256-GCM por campo (fieldcrypt), exportación y respaldo diario. No prometer de más.
  const points = [
    { title: 'Define quién ve qué', text: 'Cada rol accede solo a lo que le toca, y puedes activar la verificación en dos pasos.' },
    { title: 'Sabes quién abrió cada expediente', text: 'Queda registro de cada persona que abre un expediente y cuándo lo hizo.' },
    { title: 'Los datos sensibles se guardan cifrados', text: 'Notas de consulta, diagnósticos, antecedentes y alergias se almacenan cifrados con AES-256.' }
  ];
</script>

<section id="seguridad" class="scroll-mt-24 px-3 py-10 sm:px-5 lg:py-14">
  <div class="l-deep relative isolate overflow-hidden rounded-[36px] bg-ink text-paper">

    <div class="mx-auto grid max-w-6xl gap-12 px-5 py-14 sm:px-8 lg:grid-cols-[1.05fr_1fr] lg:items-center lg:gap-16 lg:py-20">
      <div class="relative order-2 px-2 py-8 lg:order-1 sm:px-6" aria-hidden="true">
        <div use:parallax={0.05} class="relative rounded-[26px] bg-panel p-6 text-ink shadow-[0_50px_90px_-30px_rgb(0_0_0/0.65)] ring-1 ring-paper/20 sm:p-7">
          <span class="absolute right-5 top-5 rounded-full bg-ink/5 px-2.5 py-1 font-mono text-[10px] uppercase tracking-[0.12em] text-ink-soft">Ejemplo</span>
          <Permisos />
        </div>
        <div use:parallax={-0.09} class="absolute -right-1 bottom-0 hidden max-w-[17rem] sm:block lg:-right-6">
          <div class="rounded-2xl bg-ink px-4 py-3 shadow-[0_24px_48px_-16px_rgb(0_0_0/0.6)] ring-1 ring-paper/20">
            <p class="font-mono text-[10px] uppercase tracking-[0.12em] text-paper/55">Así se guarda una nota</p>
            <p class="mt-1.5 break-all font-mono text-[11px] leading-relaxed text-mint">enc:v1:AV8yQ2nR0kXwYh3Lq9Tf…</p>
          </div>
        </div>
        <div use:parallax={0.1} class="absolute -left-1 -top-3 hidden sm:block lg:-left-6">
          <div class="flex items-center gap-2.5 rounded-2xl bg-paper px-3.5 py-2.5 text-[13px] font-medium text-ink shadow-[0_24px_48px_-16px_rgb(0_0_0/0.5)]">
            <span class="grid h-6 w-6 place-items-center rounded-full bg-mint text-white"><Icon name="check" class="h-3.5 w-3.5" strokeWidth={2.4} /></span>Acceso registrado · hace 2 min
          </div>
        </div>
      </div>

      <div class="order-1 lg:order-2">
        <p class="flex items-center gap-2 font-mono text-[11px] uppercase tracking-[0.16em] text-paper/60"><span class="h-1.5 w-1.5 rounded-full bg-mint"></span>Seguridad</p>
        <Split text={'Información clínica,\n*con acceso controlado.*'} class="mt-5 font-display text-[clamp(2.25rem,4.5vw,4rem)] leading-[0.98] tracking-[-0.03em]" accent="italic text-signal-soft" />
        <p class="mt-5 max-w-md text-base leading-relaxed text-paper/65 sm:text-lg">Un expediente es de lo más privado que existe. Por eso no basta con una contraseña.</p>
        <ul class="mt-8 grid gap-6">
          {#each points as p, i}
            <li use:reveal={{ y: 20, delay: i * 0.07 }} class="flex gap-4">
              <span class="mt-0.5 grid h-8 w-8 shrink-0 place-items-center rounded-full bg-paper/12 text-paper"><Icon name="check" class="h-4 w-4" strokeWidth={2.4} /></span>
              <div>
                <h3 class="font-display text-[1.6rem] leading-tight">{p.title}</h3>
                <p class="mt-1.5 text-[15px] leading-relaxed text-paper/65">{p.text}</p>
              </div>
            </li>
          {/each}
        </ul>
        <p class="mt-7 border-t border-paper/15 pt-5 text-[14px] leading-relaxed text-paper/55">Además: respaldo diario automático y descarga de toda tu información cuando la necesites.</p>
      </div>
    </div>
  </div>
</section>
