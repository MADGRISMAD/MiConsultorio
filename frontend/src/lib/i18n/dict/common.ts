// Shared strings, error pages, offline page, install/update prompts and the language selector.
export const es = {
  'common.loading': 'Cargando…',
  'common.retry': 'Reintentar',
  'common.cancel': 'Cancelar',
  'common.date': 'Fecha',
  'common.time': 'Hora',
  'common.address': 'Dirección',
  'common.phone': 'Teléfono',
  'common.service': 'Servicio',
  'common.attends': 'Atiende',
  'common.firstNames': 'Nombre(s)',
  'common.lastNames': 'Apellidos',
  'common.email': 'Correo',
  'common.website': 'Sitio web',
  'common.privacyLink': 'aviso de privacidad',
  'common.hourSuffix': '{time} h',

  'lang.label': 'Idioma',

  'shell.bookedWith': 'Agenda con',

  'error.notFoundTitle': 'No encontramos esta página',
  'error.notFoundText': 'Revisa que el enlace esté completo o vuelve al inicio.',
  'error.genericTitle': 'Algo salió mal',
  'error.genericText': 'No se pudo mostrar esta página. Inténtalo de nuevo en un momento.',
  'error.home': 'Ir al inicio',

  'offline.title': 'Sin conexión',
  'offline.text': 'No hay conexión a internet. Caresia necesita red para mostrar datos del consultorio; por seguridad no se guardan copias de información clínica en este dispositivo.',
  'offline.retry': 'Reintentar',
  'offline.pageTitle': 'Sin conexión · Caresia',

  'pwa.install': 'Instalar la aplicación',
  'pwa.installText': 'Instala Caresia en tu dispositivo para abrirla como una aplicación.',
  'pwa.installDismiss': 'Ahora no',
  'pwa.iosTitle': 'Instalar en iPhone o iPad',
  'pwa.iosStep1': 'Toca el botón Compartir de Safari.',
  'pwa.iosStep2': 'Elige «Agregar a pantalla de inicio».',
  'pwa.iosStep3': 'Confirma con «Agregar».',
  'pwa.update': 'Hay una versión nueva de Caresia.',
  'pwa.updateAction': 'Actualizar',
  'pwa.updateLater': 'Después'
};

export const en: Record<keyof typeof es, string> = {
  'common.loading': 'Loading…',
  'common.retry': 'Try again',
  'common.cancel': 'Cancel',
  'common.date': 'Date',
  'common.time': 'Time',
  'common.address': 'Address',
  'common.phone': 'Phone',
  'common.service': 'Service',
  'common.attends': 'Seen by',
  'common.firstNames': 'First name(s)',
  'common.lastNames': 'Last name(s)',
  'common.email': 'Email',
  'common.website': 'Website',
  'common.privacyLink': 'privacy notice',
  'common.hourSuffix': '{time}',

  'lang.label': 'Language',

  'shell.bookedWith': 'Scheduling by',

  'error.notFoundTitle': 'We could not find this page',
  'error.notFoundText': 'Check that the link is complete or go back to the start.',
  'error.genericTitle': 'Something went wrong',
  'error.genericText': 'This page could not be shown. Please try again in a moment.',
  'error.home': 'Go to the start',

  'offline.title': 'You are offline',
  'offline.text': 'There is no internet connection. Caresia needs a network to show clinic data; for your security no copies of clinical information are stored on this device.',
  'offline.retry': 'Try again',
  'offline.pageTitle': 'Offline · Caresia',

  'pwa.install': 'Install the app',
  'pwa.installText': 'Install Caresia on your device to open it like an app.',
  'pwa.installDismiss': 'Not now',
  'pwa.iosTitle': 'Install on iPhone or iPad',
  'pwa.iosStep1': 'Tap the Share button in Safari.',
  'pwa.iosStep2': 'Choose “Add to Home Screen”.',
  'pwa.iosStep3': 'Confirm with “Add”.',
  'pwa.update': 'There is a new version of Caresia.',
  'pwa.updateAction': 'Update',
  'pwa.updateLater': 'Later'
};
