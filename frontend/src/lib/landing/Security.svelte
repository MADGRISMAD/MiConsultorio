<script lang="ts">
  import Icon from './Icon.svelte';
  import SectionHead from './SectionHead.svelte';
  import { reveal } from './motion';
  import Permisos from './screens/Permisos.svelte';

  // Solo lo verificable en el código: permisos por rol y por persona, 2FA, registro de quién abre cada expediente (record_access),
  // cifrado AES-256-GCM por campo (fieldcrypt), exportación y respaldo diario. No prometer de más.
  const points = [
    { title: 'Define quién ve qué', text: 'Cada rol accede solo a lo que le toca, y puedes activar la verificación en dos pasos.' },
    { title: 'Sabes quién abrió cada expediente', text: 'Queda registro de cada persona que abre un expediente y cuándo lo hizo.' },
    { title: 'Los datos sensibles se guardan cifrados', text: 'Notas de consulta, diagnósticos, antecedentes y alergias se almacenan cifrados con AES-256.' }
  ];
</script>

<section id="seguridad" class="mx-auto max-w-6xl scroll-mt-24 px-5 py-16 sm:px-8 lg:py-24">
  <SectionHead title={'Información clínica,\n*con acceso controlado.*'} blurb="Un expediente es de lo más privado que existe. Por eso no basta con una contraseña." />
  <div class="mt-10 grid gap-10 lg:grid-cols-[1.05fr_1fr] lg:items-center lg:gap-16">
    <div use:reveal={{ y: 30 }} class="relative rounded-[26px] bg-panel p-6 ring-1 ring-ink/10 sm:p-7" aria-hidden="true">
      <span class="absolute right-5 top-5 rounded-full bg-ink/5 px-2.5 py-1 font-mono text-[10px] uppercase tracking-[0.12em] text-ink-soft">Ejemplo</span>
      <Permisos />
    </div>
    <ul class="grid gap-7">
      {#each points as p, i}
        <li use:reveal={{ y: 20, delay: i * 0.07 }} class="flex gap-4">
          <span class="mt-0.5 grid h-8 w-8 shrink-0 place-items-center rounded-full bg-signal-soft text-signal"><Icon name="check" class="h-4 w-4" strokeWidth={2.4} /></span>
          <div>
            <h3 class="font-display text-[1.7rem] leading-tight">{p.title}</h3>
            <p class="mt-1.5 text-[16px] leading-relaxed text-ink-soft">{p.text}</p>
          </div>
        </li>
      {/each}
      <li class="border-t border-ink/10 pt-6 text-[15px] leading-relaxed text-ink-soft">Además: respaldo diario automático y descarga de toda tu información cuando la necesites.</li>
    </ul>
  </div>
</section>
