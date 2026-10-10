<script lang="ts">
  import Icon from './Icon.svelte';
  import Split from './Split.svelte';
  import { reveal } from './motion';

  // Todo lo de aquí es verificable en el código: fieldcrypt (AES-256-GCM por campo, atado a tabla/columna/fila),
  // record_access (quién abre cada expediente), 2FA, exportación y respaldo diario. No prometer de más.
  const points = [
    { title: 'Cifrado campo por campo', text: 'Notas de consulta, diagnósticos, indicaciones, antecedentes, alergias y notas de laboratorio se guardan cifrados con AES-256, el mismo estándar que usa la banca.' },
    { title: 'Ilegible fuera de su lugar', text: 'Cada dato queda sellado a su paciente y a su campo. Si alguien copia la base de datos o mueve un dato a otro expediente, no se puede abrir.' },
    { title: 'Sabes quién vio qué', text: 'Queda registro de cada persona que abre un expediente y cuándo lo hizo.' },
    { title: 'Acceso con candado', text: 'Cada clínica ve solo a sus pacientes, cada rol solo lo que le toca, y puedes activar la verificación en dos pasos.' },
    { title: 'Tus datos son tuyos', text: 'Respaldo diario automático y descarga de toda tu información cuando la necesites.' }
  ];
</script>

<section id="seguridad" class="relative overflow-hidden bg-paper px-5 py-28 sm:px-8 lg:py-36">
  <div class="mx-auto max-w-6xl">
    <div class="grid gap-6 md:grid-cols-[1.6fr_1fr] md:items-end">
      <Split text={'Lo que tu paciente te cuenta\n*se queda entre ustedes.*'} class="font-display text-[clamp(2.4rem,5.6vw,4.6rem)] leading-[0.98] tracking-[-0.03em]" accent="italic text-signal" />
      <p class="max-w-sm text-lg leading-relaxed text-ink/60 md:justify-self-end">Un expediente clínico es lo más privado que existe. Por eso no basta con una contraseña.</p>
    </div>

    <div class="mt-16 grid gap-10 lg:grid-cols-[1.05fr_1fr] lg:items-start">
      <!-- Lo que ve el doctor contra lo que queda guardado -->
      <div use:reveal class="rounded-[28px] bg-ink p-6 text-paper sm:p-8">
        <p class="text-xs font-semibold uppercase tracking-[0.18em] text-paper/60">Lo que ves en Caresia</p>
        <div class="mt-3 rounded-2xl bg-paper/[0.06] p-5 ring-1 ring-paper/10">
          <p class="text-[13px] text-paper/50">Nota de consulta · Alergias</p>
          <p class="mt-2 text-[17px] leading-relaxed">Refiere dolor en molar inferior derecho desde hace 3 días. Alérgica a la penicilina.</p>
        </div>
        <div class="my-5 flex items-center gap-3 text-paper/40" aria-hidden="true">
          <span class="h-px flex-1 bg-paper/15"></span>
          <span class="font-display text-sm italic text-signal">se guarda así</span>
          <span class="h-px flex-1 bg-paper/15"></span>
        </div>
        <p class="text-xs font-semibold uppercase tracking-[0.18em] text-paper/60">Lo que queda en la base de datos</p>
        <p class="mt-3 break-all rounded-2xl bg-paper/[0.06] p-5 font-mono text-[13px] leading-relaxed text-paper/70 ring-1 ring-paper/10">enc:v1:AV8yQ2nR0kXwYh3Lq9TfZcB7mJp1sUeD4vGa6HtNiKo2WxErF5yCbM0zLjQ8uPdS3gVhAnTk7RwXe9OiYm…</p>
        <p class="mt-5 text-sm leading-relaxed text-paper/55">Ejemplo ilustrativo. Sin la llave, que vive separada de la base de datos, el texto no se puede leer.</p>
      </div>

      <ul class="grid gap-7">
        {#each points as p, i}
          <li use:reveal={{ delay: i * 0.07 }} class="flex gap-4">
            <span class="mt-1 grid h-7 w-7 shrink-0 place-items-center rounded-full bg-signal-soft text-signal"><Icon name="check" /></span>
            <div>
              <h3 class="font-display text-2xl leading-tight">{p.title}</h3>
              <p class="mt-1.5 text-[16px] leading-relaxed text-ink/65">{p.text}</p>
            </div>
          </li>
        {/each}
      </ul>
    </div>
  </div>
</section>
