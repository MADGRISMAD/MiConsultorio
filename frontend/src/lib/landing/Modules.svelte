<script lang="ts">
  import Icon from './Icon.svelte';
  import Split from './Split.svelte';
  import Carousel from './Carousel.svelte';
  import { reveal } from './motion';

  interface Group {
    n: string;
    title: string;
    blurb: string;
    items: string[];
    tone: 'plain' | 'mint' | 'signal';
    badge?: string;
  }
  const groups: Group[] = [
    {
      n: '01',
      title: 'Agenda y citas',
      blurb: 'Varios profesionales atendiendo al mismo tiempo, sin empalmes ni llamadas perdidas.',
      items: [
        'Vista por día, semana, mes y lista, por profesional y por sala',
        'Reprograma arrastrando la cita',
        'Reserva en línea con la liga de tu consultorio',
        'Recordatorios automáticos por correo',
        'Lista de espera: avisa al paciente cuando se libera un lugar',
        'Panel «En proceso»: quién espera y quién está en consulta',
        'Citas de seguimiento recomendadas por el profesional, por confirmar',
        'Bloqueos, vacaciones y duración distinta por servicio'
      ],
      tone: 'plain'
    },
    {
      n: '02',
      title: 'Expediente clínico',
      blurb: 'El historial completo de cada paciente, ordenado y protegido.',
      items: [
        'Antecedentes y signos vitales según tu especialidad',
        'Notas de consulta que no se alteran (NOM-004), con adendas',
        'Diagnósticos CIE-10 y códigos de tu catálogo',
        'Radiografías, estudios y fotos, con visor DICOM',
        'Consentimientos y aviso de privacidad firmados en pantalla',
        'Historial de versiones y comparación entre cambios',
        'Imprime o exporta el expediente completo'
      ],
      tone: 'mint'
    },
    {
      n: '03',
      title: 'Recetas',
      blurb: 'Recetas listas para imprimir o enviar, con verificación.',
      items: [
        'Catálogo de medicamentos y el tuyo propio',
        'Alertas de alergias antes de recetar',
        'Calculadora de dosis por peso',
        'Código QR para verificar la receta',
        'Hoja de indicaciones para las especialidades que no recetan'
      ],
      tone: 'plain'
    },
    {
      n: '04',
      title: 'Cobros y caja',
      blurb: 'De la consulta al cobro sin capturar dos veces.',
      items: [
        'Al terminar la consulta, la cuenta pasa sola a caja',
        'Punto de venta, caja con corte y abonos',
        'Cobro con terminal y ligas de pago de Mercado Pago',
        'Servicios e inventario con lotes y caducidades',
        'Devoluciones, comisiones por profesional y por servicio',
        'Facturación electrónica (CFDI)',
        'Ticket, comprobante tamaño carta y envío por correo'
      ],
      tone: 'signal',
      badge: 'Plan Crecimiento'
    },
    {
      n: '05',
      title: 'Inteligencia artificial',
      blurb: 'Ahorra tiempo en lo repetitivo y revisa siempre el resultado.',
      items: [
        'Plan nutricional de 7 días según objetivo, antecedentes y gustos',
        'Respeta alergias y los alimentos que no le gustan al paciente',
        'Inventario mágico: una lista o una foto se vuelve catálogo',
        'Precios sugeridos a partir de tus costos'
      ],
      tone: 'mint',
      badge: 'Plan Crecimiento'
    },
    {
      n: '06',
      title: 'Equipo y seguridad',
      blurb: 'Cada persona ve y hace lo que le toca.',
      items: [
        'Roles y permisos personalizados por persona',
        'Verificación en dos pasos',
        'Bitácora de actividad y de quién abrió cada expediente',
        'Datos sensibles cifrados y respaldos',
        'Varias sucursales con reportes del grupo',
        'Descarga tus datos cuando quieras'
      ],
      tone: 'plain'
    },
    {
      n: '07',
      title: 'Portal del paciente',
      blurb: 'Menos llamadas, pacientes mejor informados.',
      items: ['Citas, recetas y vacunas en su portal', 'Reserva y confirmación de citas en línea', 'Carnet de vacunación digital', 'Solicitudes ARCO con seguimiento'],
      tone: 'signal'
    },
    {
      n: '08',
      title: 'Reportes',
      blurb: 'Cómo va tu consultorio, con números reales.',
      items: [
        'Consultas, ausentismo, ocupación y tiempos de espera',
        'Pacientes nuevos, recurrentes y los que no vuelven',
        'Ventas, inventario y comisiones',
        'Exporta a CSV'
      ],
      tone: 'plain'
    },
    {
      n: '09',
      title: 'En tu bolsillo',
      blurb: 'Se instala como una app y se adapta a cualquier pantalla.',
      items: ['Funciona en computadora, tableta y celular', 'Instalable, sin pasar por tiendas de apps', 'Español e inglés', 'Modo claro y oscuro'],
      tone: 'mint'
    }
  ];
  const tones = { plain: 'bg-panel ring-1 ring-ink/10', mint: 'bg-mint-soft', signal: 'bg-signal-soft' } as const;
</script>

<section id="incluye" class="mx-auto max-w-6xl px-5 py-28 sm:px-8 lg:py-40">
  <div class="grid gap-6 border-b border-ink/15 pb-10 md:grid-cols-[1.7fr_1fr] md:items-end">
    <Split text={'Todo lo que\n*ya incluye.*'} class="font-display text-[clamp(2.75rem,6.5vw,5.25rem)] leading-[0.95] tracking-[-0.03em]" accent="italic text-signal" />
    <p class="max-w-sm text-lg leading-relaxed text-ink-soft md:justify-self-end">Del primer mensaje de la recepción al reporte del mes: lo que necesita un consultorio, en un solo sistema.</p>
  </div>
  <div class="mt-10" use:reveal={{ y: 50 }}>
    <Carousel items={groups} label={(g) => g.title} name="Lo que incluye" id="incluye">
      {#snippet panel(g)}
        <div class="rounded-[28px] p-7 sm:p-9 {tones[g.tone]}">
          <div class="grid gap-8 md:grid-cols-[1fr_1.4fr]">
            <div>
              <div class="flex items-center gap-3">
                <p class="font-mono text-sm text-signal">{g.n} <span class="text-ink-faint">/ {String(groups.length).padStart(2, '0')}</span></p>
                {#if g.badge}<span class="rounded-full bg-ink px-2.5 py-1 font-mono text-[10px] uppercase tracking-[0.12em] text-paper">{g.badge}</span>{/if}
              </div>
              <h3 class="mt-6 font-display text-[clamp(2.25rem,4.5vw,3.5rem)] leading-[0.95] tracking-[-0.02em]">{g.title}</h3>
              <p class="mt-4 max-w-md text-[16px] leading-relaxed text-ink-soft">{g.blurb}</p>
            </div>
            <ul class="grid content-start gap-x-6 gap-y-2.5 text-[15px] leading-snug sm:grid-cols-2">
              {#each g.items as t}
                <li class="flex gap-2.5">
                  <Icon name="check" class="mt-0.5 h-4 w-4 flex-none text-mint" strokeWidth={2.4} />
                  <span>{t}</span>
                </li>
              {/each}
            </ul>
          </div>
        </div>
      {/snippet}
    </Carousel>
  </div>
</section>
