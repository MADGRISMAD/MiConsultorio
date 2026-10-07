# Paquete de revisión clínica

Caresia incluye catálogos de referencia que ayudan a llenar más rápido una receta o una cartilla: medicamentos (humanos y veterinarios), un CIE-10 parcial, esquemas de vacunación y desparasitación sugeridos, y textos de consentimiento. **Son material de apoyo, no criterio clínico.** Antes de que un consultorio los use como base, deben ser revisados por profesionales con cédula.

Este paquete deja esos catálogos en archivos CSV que se abren en Excel, Google Sheets o LibreOffice, para que quien revisa pueda marcar lo que está bien, lo que hay que corregir y lo que falta. **El paquete solo exporta: nadie modifica una dosis desde aquí.** Las correcciones se aplican después, en el código, por quien mantiene el sistema.

> Estado: los archivos de `docs/revision-clinica/` se generaron automáticamente del código y **no han sido revisados por ningún profesional**. Hasta que se llenen las columnas «Revisado por / Fecha de revisión / Observación», debe tratarse todo como no validado.

## Qué contiene `docs/revision-clinica/`

| Archivo | Contenido | Quién lo revisa |
| --- | --- | --- |
| `medicamentos-humanos.csv` | Denominación genérica, categoría, control (antibiótico, fracción III…), vía, presentaciones, dosis típica, mg/kg por dosis, máximo mg/kg por día y concentraciones de líquidos (para calcular mL) | Médico (y odontólogo, para anestésicos locales, analgésicos y antibióticos de uso dental) |
| `medicamentos-veterinarios.csv` | Lo mismo, por especie (perro, gato, conejo…) | Médico veterinario zootecnista |
| `cie10-parcial.csv` | Códigos CIE-10 y su descripción (lista parcial de diagnósticos frecuentes) | Médico |
| `vacunacion-desparasitacion.csv` | Esquema básico de personas y vacunas y desparasitantes sugeridos por especie, con el intervalo de refuerzo que se propone al registrar | Médico (personas), MVZ (animales) |
| `consentimientos.csv` | Textos sugeridos de consentimiento (procedimiento, aviso de privacidad, telemedicina, psicología, animal, plan de tratamiento) | Médico, odontólogo, psicólogo, MVZ y un abogado |

Todos los archivos terminan con tres columnas vacías para el revisor: **Revisado por**, **Fecha de revisión** y **Observación**. Están codificados en UTF-8 con marca BOM para que Excel respete los acentos.

## Cómo regenerar los archivos

Desde la carpeta `backend/`, sin variables de entorno ni base de datos:

```
go run ./cmd/exportcatalogs
```

Escribe en `docs/revision-clinica/` (o en otra carpeta con `-out ruta`). Es seguro repetirlo: la salida es la misma mientras no cambien los catálogos. Las fuentes son `internal/api/rx_catalog_human.go`, `rx_catalog_vet.go`, `rx_icd10.go`, `vaccinations.go` y los textos de `frontend/src/lib/components/specialty/consentTexts.ts`. Otros textos que conviene revisar y que no están en los CSV: el aviso de privacidad y la carta de consentimiento imprimibles (`frontend/src/lib/print/avisos.ts`).

## Checklist de revisión

Marca cada punto en el CSV (columna «Observación») o en una copia de este documento.

### Médico (medicina general y especialidades)

- [ ] Cada denominación es genérica y correcta; no hay duplicados ni nombres comerciales mezclados.
- [ ] Vía de administración y **presentaciones** existen en México y están bien escritas (concentración, forma farmacéutica).
- [ ] **Dosis típica** de adulto y pediátrica coincide con la fuente que usas (Cuadro Básico, IPP, guías clínicas). Anota la fuente.
- [ ] **mg/kg por dosis** y **máximo mg/kg por día**: son los valores con los que el sistema alerta de sobredosis. Verifica que el máximo sea por día y no por dosis, y que no queden demasiado bajos (falsas alertas) ni demasiado altos (sin protección).
- [ ] **Concentraciones** de suspensiones y soluciones (mg/mL) son correctas: de ahí sale el volumen en mL.
- [ ] **Control** correcto: antibióticos (receta retenida, vigencia 30 días), fracción III (retenida), fracciones I y II (el sistema no debe emitirlos). Falta o sobra alguno.
- [ ] Notas de precaución (embarazo, función renal o hepática, interacciones) suficientes o faltantes.
- [ ] Medicamentos frecuentes que faltan en el catálogo; medicamentos que ya no deberían sugerirse.
- [ ] **CIE-10:** los códigos y descripciones son correctos; diagnósticos frecuentes que faltan.
- [ ] **Vacunación de personas:** el esquema y los intervalos coinciden con el esquema nacional vigente.
- [ ] **Consentimientos:** el texto es claro, suficiente para la NOM-004-SSA3-2012 y no promete nada indebido.

### Médico veterinario zootecnista

- [ ] Las dosis en mg/kg son correctas **por especie** (perro, gato, conejo…); especialmente AINE en gatos, antiparasitarios y anestésicos.
- [ ] Hay medicamentos que **no deben** usarse en alguna especie y aparecen sugeridos (p. ej. paracetamol en gatos, ivermectina en razas sensibles).
- [ ] Presentaciones y concentraciones corresponden a productos veterinarios disponibles en México.
- [ ] La clasificación de control es coherente con la NOM-064-ZOO-2000 y tu práctica.
- [ ] **Vacunas y desparasitantes:** esquema por especie (cachorros y gatitos, refuerzos, rabia, productos de producción como bovinos, porcinos y equinos) e intervalos sugeridos.
- [ ] Textos de consentimiento y autorización del propietario.

### Odontólogo

- [ ] Anestésicos locales (lidocaína, articaína, mepivacaína…): concentraciones, presentaciones, dosis máxima por kg y precauciones (vasoconstrictor, embarazo, cardiopatías).
- [ ] Analgésicos, antiinflamatorios y antibióticos de uso dental: dosis y duración de tratamientos típicos, profilaxis antibiótica.
- [ ] Colutorios y tópicos habituales (clorhexidina, fluoruros) que faltan o sobran.
- [ ] CIE-10 de salud bucal (grupo K00–K14 y afines): faltan códigos de uso frecuente.
- [ ] Textos de consentimiento para procedimientos odontológicos (extracciones, endodoncia, cirugía): suficientes y comprensibles.

### Psicólogo / nutriólogo (si aplica)

- [ ] El consentimiento de atención psicológica refleja el encuadre y los límites de la confidencialidad que usas.
- [ ] No hay sugerencias de fármacos para giros que no recetan.

## Qué anotar al revisar

En la columna **Observación** escribe de forma concreta: qué valor es el incorrecto, cuál es el correcto y **la fuente** (guía, ficha técnica, edición y año). Si todo está bien, escribe «OK» y tu nombre y fecha. Una línea sin revisar es una línea sin validar.

## Cómo reportar correcciones

1. Guarda tus CSV revisados (con tus tres columnas llenas) y compártelos con quien mantiene Caresia: por el canal que use tu equipo, o como un *issue* en el repositorio con el archivo adjunto.
2. Prioriza: marca primero lo que puede dañar a un paciente (dosis, máximos, especies inadecuadas, control de antibióticos).
3. Quien mantiene el sistema aplica la corrección **en el catálogo del código** (no en la base de datos), agrega una prueba cuando es una regla de seguridad, y vuelve a ejecutar `go run ./cmd/exportcatalogs` para que el CSV refleje el cambio. Cada corrección debe citar la observación y la fuente.
4. Cuando un catálogo quede revisado, anota en este documento la fecha y quién lo revisó (sección siguiente).

## Registro de revisiones

| Catálogo | Revisor y profesión | Cédula | Fecha | Resultado |
| --- | --- | --- | --- | --- |
| _(pendiente)_ | | | | |

Descargo: el catálogo apoya la captura y las alertas, pero la responsabilidad de cada receta es de quien la emite.
