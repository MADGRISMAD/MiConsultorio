# Ayudas con IA

Todas usan Google Gemini (`GEMINI_API_KEY`, `GEMINI_MODEL`) y **1 uso de magia** del plan por solicitud (se devuelve si la IA falla). Nada se guarda solo: todo llega como **borrador** o **sugerencia** que el profesional revisa y decide, con un aviso fijo que pone el servidor.

## Reglas comunes

- El modelo **no recibe el nombre ni el contacto** del paciente; sí recibe edad, sexo, alergias, medicación crónica, notas visibles (nunca las privadas de otro) y mediciones.
- Las **cifras** (tendencias, percentiles, semanas de embarazo, conteos, mg/kg) las calcula el servidor; la IA solo las redacta.
- Lenguaje de posibilidad en las sugerencias clínicas («Podría considerarse…») y lista de qué verificar.
- El audio del dictado y las fotos se envían en la solicitud y **no se guardan** por esta función.
- Cada tarea tiene prueba en `ai_tasks_test.go` (una por tarea; una tarea nueva sin caso de prueba hace fallar la suite).

## Por paciente (`backend/internal/api/ai_defs.go`)

| Tarea | Giros |
|---|---|
| Mensaje para el paciente, hoja de indicaciones | todos |
| Borrador de receta (con alertas de alergias e interacciones), códigos CIE-10 | los que recetan (CIE-10: también psicología, nutrición, fisioterapia y quiropráctica) |
| Preguntas para la próxima consulta | medicina general, interna, pediatría, dermatología, ginecología, ortopedia |
| Tendencia de laboratorio | general, interna, pediatría, ginecología, nutrición, ortopedia |
| Presupuesto sugerido (precios del catálogo), nota del periodontograma, indicaciones posteriores | odontología |
| Lectura del crecimiento (percentiles) | pediatría |
| Lectura del recordatorio de 24 h, avance del paciente | nutrición |
| Resumen de sesión (SOAP/DAP/BIRP), avance de objetivos, tendencia de escalas, borrador de objetivos y tareas | psicología (confidencial: solo el giro) |
| Rutina de cuidado de la piel, comparar fotos antes/después (no diagnostica) | dermatología |
| Resumen del control prenatal, estudios según la semana | ginecología |
| Rehabilitación por etapas; ejercicios, avance/alta, evolución (`rehab_ai.go`) | fisioterapia, quiropráctica, ortopedia |
| Revisión de dosis por peso, calendario de vacunas y desparasitación | veterinaria |
| Ordenar notas sueltas en la nota de consulta; dictado por voz | todos |

## De la clínica (`clinic_ai.go`)

Resumen del periodo y pacientes que dejaron de venir (en Indicadores; solo administración y recepción para el segundo), respuesta a una calificación y textos de la página pública (en Ajustes › Perfil).

## Antes de producción

- Configurar `GEMINI_API_KEY` y `GEMINI_MODEL`.
- Que el abogado revise el aviso de privacidad (sección de proveedores): ya describe que se envían datos clínicos sin nombre ni contacto, audio y fotos.
- Las referencias que cita el modelo (rangos de dosis, estudios por semana, etapas de rehabilitación) son orientativas: el profesional las verifica.
