# Aviso de privacidad y términos: borrador pendiente de revisión legal

Las páginas públicas `/privacidad` (aviso de privacidad de Caresia) y `/terminos` (términos de uso) son un **BORRADOR** redactado para un software de salud en México. **No son asesoría legal y no deben presentarse como definitivas hasta que las revise un abogado** con experiencia en protección de datos personales (LFPDPPP) y en servicios de salud. Las páginas muestran un aviso visible de «Borrador pendiente de revisión legal» y resaltan cada dato que aún falta.

> No se inventó ningún dato legal. Todo lo que depende de quien opera Caresia está marcado entre corchetes.

## Qué tiene que completar el operador

Los datos viven en un solo archivo: `frontend/src/lib/legal/operator.ts`. Mientras un valor siga entre corchetes, la página lo resalta.

| Dato | Campo | Dónde se usa |
| --- | --- | --- |
| Razón social o denominación de quien opera Caresia | `razonSocial` | Aviso (responsable) y términos (partes) |
| Domicilio completo para oír y recibir notificaciones | `domicilio` | Aviso y términos |
| Correo de privacidad (derechos ARCO y revocación de la plataforma) | `correoPrivacidad` | Aviso y términos |
| Teléfono de contacto | `telefono` | Términos |
| Ciudad y estado cuyos tribunales serán competentes | `jurisdiccion` | Términos |
| Fecha de última actualización | `actualizado` | Encabezado de ambas páginas |

Otros pendientes que están marcados dentro del texto, en las propias páginas:

- **Proveedores de hospedaje** (aviso, sección 6): confirmar quién aloja el sitio y la base de datos.
- **Denominación vigente de la autoridad** de protección de datos (aviso, sección 14): la autoridad y la ley han cambiado y debe confirmarla el abogado.
- **Política de reembolsos**, **plazo de conservación posterior a la terminación** y **límite de responsabilidad** (términos, secciones 6, 10 y 11).

## Qué debe validar el abogado

1. **Roles:** que sea correcta la separación del aviso entre Caresia como **responsable** (datos de cuentas y uso) y como **encargado** (datos de pacientes, cuyo responsable es cada consultorio), y que los términos sirvan como contrato de encargo o si se necesita uno adicional.
2. **Finalidades primarias y secundarias** y el mecanismo para negarse a las secundarias.
3. **Transferencias y proveedores** listados (Mercado Pago, correo SMTP, WhatsApp/Meta, Facturama, Gemini para el inventario, estadísticas sin cookies, Google Fonts): que la lista sea completa y exacta para la operación real.
4. **Plazos ARCO** (20 días hábiles para responder y 15 para hacer efectiva la determinación) y su cómputo. El sistema los cuenta en días hábiles de lunes a viernes **sin calendario de festivos**; ver `backend/internal/api/arco_deadlines.go`.
5. **Conservación y cancelación de expedientes** (NOM-004-SSA3-2012, 5 años): redacción del bloqueo de datos y de la supresión posterior.
6. **Cookies y analítica:** que la descripción (cookie de sesión necesaria, almacenamiento local, estadísticas sin cookies, Google Fonts) coincida con lo que el sitio realmente carga en producción.
7. **Medidas de seguridad** declaradas: que no prometan más de lo que el sistema hace (cifrado en reposo parcial; ver `docs/CUMPLIMIENTO-MX.md`, §4).
8. **Términos:** limitación de responsabilidad, periodo de prueba y renovación, jurisdicción, aplicabilidad de la protección al consumidor y cláusulas de suspensión y terminación.
9. **Notificación de vulneraciones** y actualización por cambios legales (incluida la reforma de la LFPDPPP).

## Aviso de privacidad del consultorio

El aviso que cada consultorio imprime y entrega a sus pacientes (`frontend/src/lib/print/avisos.ts`) es **otro documento** y también debe revisarlo un abogado. Ahora incluye el enlace al **formulario público de solicitudes ARCO** del consultorio (`/arco/<id>`) cuando el sistema conoce su dirección. El consultorio debe llenar sus datos de privacidad en Ajustes → Cumplimiento.

## Cuando esté revisado

1. Completar `operator.ts` y ajustar el texto de las páginas con los cambios del abogado.
2. Quitar el aviso de borrador de `frontend/src/lib/legal/LegalPage.svelte` y el comentario «BORRADOR» en `routes/privacidad/+page.svelte` y `routes/terminos/+page.svelte`.
3. Anotar aquí la fecha de revisión y quién la hizo.

| Documento | Revisado por | Fecha |
| --- | --- | --- |
| Aviso de privacidad de Caresia | _(pendiente)_ | |
| Términos de uso | _(pendiente)_ | |
| Aviso de privacidad del consultorio (impreso) | _(pendiente)_ | |
