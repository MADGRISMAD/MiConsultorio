<script lang="ts">
  // BORRADOR: este aviso de privacidad debe ser revisado por un abogado antes de publicarse como versión definitiva.
  // Los datos del operador (razón social, domicilio, correo de privacidad) viven en $lib/legal/operator.ts y están
  // marcados con corchetes mientras falten. No se inventan datos legales. Ver docs/AVISOS-REVISION-LEGAL.md.
  import LegalPage from '$lib/legal/LegalPage.svelte';
  import PrivacyContent from '$lib/legal/PrivacyContent.svelte';

  const toc = [
    { id: 'responsable', label: 'Quién es el responsable' },
    { id: 'roles', label: 'Caresia y los consultorios: dos papeles distintos' },
    { id: 'datos', label: 'Datos que tratamos' },
    { id: 'sensibles', label: 'Datos sensibles (de salud)' },
    { id: 'finalidades', label: 'Finalidades primarias y secundarias' },
    { id: 'transferencias', label: 'Transferencias y proveedores' },
    { id: 'arco', label: 'Derechos ARCO y cómo ejercerlos' },
    { id: 'revocacion', label: 'Revocación del consentimiento y limitación del uso' },
    { id: 'seguridad', label: 'Medidas de seguridad' },
    { id: 'conservacion', label: 'Conservación de los datos' },
    { id: 'cookies', label: 'Cookies y analítica' },
    { id: 'menores', label: 'Menores de edad' },
    { id: 'cambios', label: 'Cambios a este aviso' },
    { id: 'autoridad', label: 'Autoridad y quejas' }
  ];
</script>

<LegalPage
  title="Aviso de privacidad"
  intro="Este aviso explica cómo Caresia, plataforma para consultorios y clínicas, trata datos personales, y cómo puedes ejercer tus derechos conforme a la Ley Federal de Protección de Datos Personales en Posesión de los Particulares (LFPDPPP) y su normatividad aplicable."
  {toc}>
  <PrivacyContent />
</LegalPage>
