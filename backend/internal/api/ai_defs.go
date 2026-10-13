package api

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	medGiros    = []string{"GENERAL_MEDICAL", "INTERNAL_MEDICINE", "PEDIATRICS", "DERMATOLOGY", "GYNECOLOGY", "ORTHOPEDICS"}
	prescribers = []string{"GENERAL_MEDICAL", "DENTAL", "PEDIATRICS", "INTERNAL_MEDICINE", "DERMATOLOGY", "GYNECOLOGY", "ORTHOPEDICS", "VETERINARY"}
	todayISO    = func() string { return time.Now().Format("2006-01-02") }
	doseRe      = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*mg\b`)
)

func aiProblem(msg string) aiPlan { return aiPlan{Problem: msg} }

// audience says who reads the text: the owner of an animal, the guardian of a minor, or the patient.
func (c *aiCtx) audience() string {
	switch {
	case c.isAnimal():
		return "el propietario del animal (trátalo de usted)"
	case c.pat.Age != nil && *c.pat.Age < 18:
		return "la madre, el padre o tutor de un menor (trátalo de usted)"
	}
	return "el paciente (trátalo de usted)"
}

func init() {
	aiTasks = []aiTask{
		// ---- every giro ----
		{
			Key: "patient_message", Label: "Mensaje para el paciente", Desc: "Redacta un mensaje breve para enviar por correo o copiar.", Draft: true,
			Param: &aiParam{Label: "Tipo de mensaje", Required: true, Options: []string{"Recordatorio de cita", "Indicaciones después de la consulta", "Seguimiento", "Resultados listos", "Motivación", "Vacunas o desparasitación pendientes"}},
			Build: func(c *aiCtx, param string) aiPlan {
				data := c.notesBlock(2, 400)
				computed := ""
				if strings.HasPrefix(param, "Vacunas") {
					computed = c.vaccineBlock()
					data += "\nVACUNAS:\n" + computed
				}
				return aiPlan{Instr: "Escribe en `text` un mensaje de «" + param + "» dirigido a " + c.audience() + ", de máximo 110 palabras, cálido y claro. " +
					"Empieza con «Hola [nombre],» (el nombre real no lo conoces: deja el marcador). No des diagnósticos, dosis ni cifras que no estén en el plan registrado. Termina invitándole a escribir al consultorio si tiene dudas. Firma «El equipo del consultorio». Deja `lines` vacío y en `sections` pon como máximo una lista «Puntos incluidos».",
					Data: data, Computed: computed}
			},
		},
		{
			Key: "patient_instructions", Label: "Hoja de indicaciones", Desc: "Convierte el plan de la última consulta en indicaciones con palabras sencillas, para imprimir.", Draft: true, MinNotes: 1,
			Build: func(c *aiCtx, param string) aiPlan {
				data := c.notesBlock(3, 600)
				computed := ""
				if blk, ok := c.prenatalBlock(); ok && c.hasKind("GYNECOLOGY") {
					computed = blk
					data += "\nEMBARAZO (cifras del sistema):\n" + blk
				}
				return aiPlan{Instr: "Con el plan y la valoración de la consulta MÁS RECIENTE, prepara una hoja de indicaciones para " + c.audience() + " en lenguaje sencillo, sin tecnicismos. " +
					"En `sections`: «Qué encontramos», «Qué hacer», «Cuidados», «Cuándo volver o avisar». Usa SOLO las indicaciones y señales de alarma que el profesional escribió; si no escribió señales de alarma, pon en «Cuándo volver o avisar» solamente: «Si algo empeora o tienes dudas, comunícate con el consultorio». " +
					"En `text` deja el mismo contenido como texto corrido listo para imprimir. `lines` vacío.", Data: data, Computed: computed}
			},
		},
		{
			Key: "visit_questions", Label: "Preguntas para la próxima consulta", Desc: "Repaso de pendientes, preguntas al paciente y datos por verificar.", Giros: medGiros, MinNotes: 1,
			Build: func(c *aiCtx, param string) aiPlan {
				tr, lab := c.measureTrends(), c.labTrends()
				data := c.notesBlock(6, 500) + "\nTENDENCIAS DE MEDICIONES:\n" + tr
				if lab != "" {
					data += "\nLABORATORIO:\n" + lab
				}
				return aiPlan{Instr: "Prepara el repaso para la PRÓXIMA consulta. `sections`: «Preguntas para el paciente», «Pendientes por cerrar» (estudios, controles o indicaciones sin seguimiento), «Datos a verificar». Hasta 5 puntos por lista. Son posibilidades para que el profesional las considere, no diagnósticos. `text` y `lines` vacíos.", Data: data, Computed: tr + lab}
			},
		},
		{
			Key: "lab_trends", Label: "Tendencia de laboratorio", Desc: "Compara los estudios y marca lo que sale de referencia.", Giros: []string{"GENERAL_MEDICAL", "INTERNAL_MEDICINE", "PEDIATRICS", "GYNECOLOGY", "NUTRITION", "ORTHOPEDICS"}, Subject: "person",
			Build: func(c *aiCtx, param string) aiPlan {
				lab := c.labTrends()
				if lab == "" {
					return aiProblem("Aún no hay resultados de laboratorio con valores numéricos.")
				}
				return aiPlan{Instr: "Describe la tendencia de los estudios de laboratorio. `summary`: 2 oraciones. `sections`: «Fuera de referencia», «Cambios importantes» y «Podría revisarse» (qué considerar dar seguimiento). Cita las cifras tal cual vienen. `text` y `lines` vacíos.", Data: lab, Computed: lab}
			},
		},
		{
			Key: "rx_draft", Label: "Borrador de receta", Desc: "Propone medicamentos para la valoración; cada uno se revisa contra alergias e interacciones.", Giros: prescribers, MinNotes: 1,
			Param: &aiParam{Label: "Diagnóstico o indicación (opcional)", Hint: "Si lo dejas vacío se usa la última valoración."},
			Build: func(c *aiCtx, param string) aiPlan {
				data := c.notesBlock(2, 500)
				if param != "" {
					data += "\nINDICACIÓN QUE ESCRIBIÓ EL PROFESIONAL: " + param
				}
				return aiPlan{Instr: "Propone en `lines` hasta 5 medicamentos que podrían considerarse para la valoración y la indicación. `name`: nombre genérico; `detail`: dosis, frecuencia y duración habituales para la edad o el peso registrados, más una indicación breve. NUNCA propongas algo a lo que el paciente sea alérgico ni que interactúe con su medicación crónica. Si los datos no bastan, deja `lines` vacío y explica en `summary`. `sections` puede traer «Medidas no farmacológicas».", Data: data}
			},
			Post: func(c *aiCtx, out *aiOut) {
				var items []rxItem
				for _, l := range out.Lines {
					items = append(items, rxItem{Medicine: l.Name})
				}
				if len(items) == 0 {
					return
				}
				allergies, hits, _, err := c.s.rxSafety(c.r.Context(), c.p.ClinicID, c.pat, items)
				if err != nil {
					out.Notices = append(out.Notices, "No se pudieron revisar las alertas de alergias e interacciones: revísalas tú.")
					return
				}
				for i := range out.Lines {
					n := strings.ToLower(out.Lines[i].Name)
					for _, a := range allergies {
						if strings.EqualFold(a.Medicine, out.Lines[i].Name) || strings.Contains(n, strings.ToLower(a.Medicine)) {
							out.Lines[i].Alerts = append(out.Lines[i].Alerts, "Alergia: "+a.Message)
						}
					}
					for _, h := range hits {
						if strings.EqualFold(h.Drug, out.Lines[i].Name) || strings.Contains(n, strings.ToLower(h.Drug)) {
							out.Lines[i].Alerts = append(out.Lines[i].Alerts, h.Message)
						}
					}
				}
				out.Notices = append(out.Notices, "Las alertas son una ayuda: la receta final la prescribe y firma el profesional.")
			},
		},
		{
			Key: "icd_suggest", Label: "Sugerir códigos CIE-10", Desc: "Códigos posibles a partir de la valoración; tú eliges.", Subject: "person", Giros: append([]string{"PSYCHOLOGY", "NUTRITION", "PHYSIOTHERAPY", "CHIROPRACTIC"}, prescribers...),
			Param: &aiParam{Label: "Valoración o diagnóstico", Required: true},
			Build: func(c *aiCtx, param string) aiPlan {
				return aiPlan{Instr: "Con la valoración escrita, propone en `lines` hasta 5 códigos CIE-10 posibles: `name` = el código exacto (por ejemplo J00 o E11.9), `detail` = el nombre del diagnóstico. Usa solo códigos de uso común en consulta externa. `sections` y `text` vacíos.", Data: "Valoración del profesional: " + param + "\n" + c.notesBlock(1, 300)}
			},
			Post: func(c *aiCtx, out *aiOut) {
				known := map[string]string{}
				for _, e := range icd10 {
					known[e.Code] = e.Name
				}
				kept := out.Lines[:0]
				for _, l := range out.Lines {
					code := strings.ToUpper(strings.TrimSpace(l.Name))
					if name, ok := known[code]; ok {
						l.Name, l.Detail = code, name
						kept = append(kept, l)
					}
				}
				out.Lines = kept
				if len(kept) == 0 {
					out.Notices = append(out.Notices, "Ningún código sugerido está en la lista de la clínica: búscalo a mano.")
				}
			},
		},

		// ---- dental ----
		{
			Key: "dental_budget", Label: "Presupuesto sugerido", Desc: "Propone tratamientos por pieza según el odontograma, con los precios de tu catálogo.", Giros: []string{"DENTAL"}, Subject: "person",
			Build: func(c *aiCtx, param string) aiPlan {
				og := c.charts("odontogram", 1)
				if len(og) == 0 {
					return aiProblem("Primero registra el odontograma del paciente.")
				}
				finds := odontogramBlock(og[0].Data)
				if finds == "" {
					return aiProblem("El odontograma no tiene hallazgos.")
				}
				rows, err := c.s.db.Query(c.r.Context(), `SELECT name FROM catalog_items WHERE clinic_id=$1 AND kind='service' AND active ORDER BY name LIMIT 150`, c.p.ClinicID)
				if err != nil {
					return aiProblem("No se pudo leer tu catálogo de servicios.")
				}
				defer rows.Close()
				var names []string
				for rows.Next() {
					var n string
					if rows.Scan(&n) == nil {
						names = append(names, n)
					}
				}
				if len(names) == 0 {
					return aiProblem("Agrega tus servicios al catálogo (con precio) para poder sugerir un presupuesto.")
				}
				return aiPlan{Instr: "Propone en `lines` los tratamientos que podrían considerarse para los hallazgos del odontograma. `name` = el nombre EXACTO de un servicio del catálogo (elige solo de esa lista; si ninguno encaja, no lo incluyas), `detail` = la pieza en notación FDI (o «general»), `qty` = cantidad. `summary`: prioridades en 2 oraciones. `sections` puede traer «Orden sugerido». Sin precios: los pone el sistema.",
					Data: "HALLAZGOS DEL ODONTOGRAMA:\n" + finds + "\nCATÁLOGO DE SERVICIOS:\n- " + strings.Join(names, "\n- "), Computed: finds}
			},
			Post: func(c *aiCtx, out *aiOut) {
				rows, err := c.s.db.Query(c.r.Context(), `SELECT id::text, name, price_cents, tax_rate::float8 FROM catalog_items WHERE clinic_id=$1 AND kind='service' AND active`, c.p.ClinicID)
				if err != nil {
					out.Lines = nil
					return
				}
				defer rows.Close()
				type it struct {
					id    string
					price int
					tax   float64
				}
				cat := map[string]it{}
				for rows.Next() {
					var id, n string
					var pr int
					var tx float64
					if rows.Scan(&id, &n, &pr, &tx) == nil {
						cat[strings.ToLower(strings.TrimSpace(n))] = it{id, pr, tx}
					}
				}
				kept := out.Lines[:0]
				for _, l := range out.Lines {
					if m, ok := cat[strings.ToLower(l.Name)]; ok {
						l.CatalogItemID, l.UnitPriceCents, l.TaxRate = m.id, m.price, m.tax
						kept = append(kept, l)
					} else {
						out.Notices = append(out.Notices, "«"+l.Name+"» no está en tu catálogo y se omitió.")
					}
				}
				out.Lines = kept
			},
		},
		{
			Key: "perio_note", Label: "Nota del periodontograma", Desc: "Resume el estado periodontal con cifras calculadas y sugiere el seguimiento.", Giros: []string{"DENTAL"}, Subject: "person",
			Build: func(c *aiCtx, param string) aiPlan {
				pg := c.charts("periodontogram", 2)
				if len(pg) == 0 {
					return aiProblem("Primero registra el periodontograma del paciente.")
				}
				cur := perioStats(pg[0].Data)
				data := "PERIODONTOGRAMA ACTUAL (" + pg[0].At.Format("2006-01-02") + "):\n" + cur
				if n := str(pg[0].Data["notes"]); n != "" {
					data += "Notas: " + truncateRunes(n, 300) + "\n"
				}
				if len(pg) > 1 {
					data += "\nANTERIOR (" + pg[1].At.Format("2006-01-02") + "):\n" + perioStats(pg[1].Data)
				}
				return aiPlan{Instr: "Redacta una nota breve del estado periodontal y compara con el registro anterior si existe. `summary`: 2 o 3 oraciones para el expediente. `sections`: «Hallazgos» y «Podría considerarse» (seguimiento, mantenimiento, reevaluación). No diagnostiques periodontitis ni estadifiques: eso lo hace el profesional. `text` y `lines` vacíos.", Data: data, Computed: data}
			},
		},
		{
			Key: "post_procedure", Label: "Indicaciones posteriores", Desc: "Hoja de cuidados después de un procedimiento dental.", Giros: []string{"DENTAL"}, Draft: true,
			Param: &aiParam{Label: "Procedimiento realizado", Required: true, Hint: "Ej. extracción de molar, endodoncia, resina, implante"},
			Build: func(c *aiCtx, param string) aiPlan {
				return aiPlan{Instr: "Redacta en `text` y en `sections` las indicaciones posteriores al procedimiento «" + param + "» para " + c.audience() + ": cuidados de las primeras 24 horas, alimentación, higiene, qué molestias son normales y cuándo contactar al consultorio. No incluyas medicamentos ni dosis (los indica el profesional). `lines` vacío.", Data: c.notesBlock(1, 300)}
			},
		},

		// ---- pediatrics ----
		{
			Key: "growth_reading", Label: "Lectura del crecimiento", Desc: "Interpreta peso, talla, IMC y perímetro cefálico con percentiles calculados.", Giros: []string{"PEDIATRICS"}, Subject: "person", MinNotes: 1,
			Build: func(c *aiCtx, param string) aiPlan {
				g := c.growthBlock()
				if g == "" {
					return aiProblem("Registra peso y talla en las consultas, y la fecha de nacimiento del paciente.")
				}
				return aiPlan{Instr: "Interpreta el crecimiento con las cifras y percentiles ya calculados. `summary`: 2 oraciones. `sections`: «Lo que muestran los datos», «Podría considerarse» (seguimiento o factores a explorar) y, en «Para los padres», 3 puntos en lenguaje sencillo. Si no hay percentiles, dilo y no los inventes. `text` y `lines` vacíos.", Data: g + "\n" + c.notesBlock(3, 300), Computed: g}
			},
		},

		// ---- nutrition ----
		{
			Key: "recall_reading", Label: "Lectura del recordatorio de 24 h", Desc: "Señala lo que destaca en lo que come el paciente.", Giros: []string{"NUTRITION"}, Subject: "person",
			Build: func(c *aiCtx, param string) aiPlan {
				rc := c.recallBlock()
				if rc == "" {
					return aiProblem("Primero registra el recordatorio de 24 horas.")
				}
				return aiPlan{Instr: "Lee el recordatorio alimentario y señala lo que destaca. `sections`: «Lo que destaca» (por ejemplo pocas verduras, mucha bebida azucarada, comidas omitidas, horarios), «Podría trabajarse» y «Preguntas para aclarar con el paciente». No calcules calorías ni macros exactos. `text` y `lines` vacíos.", Data: rc + "\n" + c.nutritionPlanBlock()}
			},
		},
		{
			Key: "nutrition_progress", Label: "Avance del paciente", Desc: "Compara peso y medidas entre consultas y con el plan vigente.", Giros: []string{"NUTRITION"}, Subject: "person", MinNotes: 2,
			Build: func(c *aiCtx, param string) aiPlan {
				tr := c.measureTrends()
				if tr == "" {
					return aiProblem("Aún no hay mediciones registradas para comparar.")
				}
				return aiPlan{Instr: "Resume el avance con las cifras calculadas. `summary`: 2 oraciones. `sections`: «Cambios» (cita las cifras), «Podría considerarse» (ajustes o seguimiento), «Para motivar al paciente» (2 frases). No juzgues al paciente. `text` y `lines` vacíos.", Data: "TENDENCIAS:\n" + tr + "\n" + c.nutritionPlanBlock() + "\n" + c.notesBlock(3, 300), Computed: tr}
			},
		},

		// ---- psychology (confidential: only the giro) ----
		{
			Key: "session_summary", Label: "Resumen de sesión", Desc: "Ordena la última nota en el formato clínico que uses.", Giros: []string{"PSYCHOLOGY"}, Subject: "person", MinNotes: 1, Draft: true,
			Param: &aiParam{Label: "Formato", Required: true, Options: []string{"SOAP", "DAP", "BIRP"}},
			Build: func(c *aiCtx, param string) aiPlan {
				f := map[string]string{"SOAP": "Subjetivo, Objetivo, Análisis, Plan", "DAP": "Datos, Evaluación, Plan", "BIRP": "Comportamiento, Intervención, Respuesta, Plan"}[param]
				return aiPlan{Instr: "Reordena la nota MÁS RECIENTE en formato " + param + " (" + f + "). En `sections` un encabezado por parte y en `text` el mismo contenido corrido para el expediente. Usa solo lo que el terapeuta escribió; no agregues diagnósticos, no valores riesgo y no interpretes más allá de la nota. `lines` vacío.", Data: c.notesBlock(2, 1200)}
			},
		},
		{
			Key: "goals_progress", Label: "Avance de objetivos y tareas", Desc: "Revisa los objetivos terapéuticos y las tareas entre sesiones.", Giros: []string{"PSYCHOLOGY"}, Subject: "person",
			Build: func(c *aiCtx, param string) aiPlan {
				tb := c.therapyBlock()
				if tb == "" {
					return aiProblem("Primero registra el plan terapéutico (objetivos y tareas).")
				}
				return aiPlan{Instr: "Revisa el avance del plan terapéutico con los conteos ya calculados. `sections`: «Avance», «Tareas vencidas o sin cumplir» y «Podría trabajarse en la próxima sesión». Son ideas para el terapeuta. `text` y `lines` vacíos.", Data: tb + "\n" + c.notesBlock(3, 400), Computed: tb}
			},
		},
		{
			Key: "scale_trend", Label: "Tendencia de las escalas", Desc: "Lee la evolución de PHQ-9, GAD-7 y PSS-10.", Giros: []string{"PSYCHOLOGY"}, Subject: "person",
			Build: func(c *aiCtx, param string) aiPlan {
				sc := c.scaleBlock()
				if sc == "" {
					return aiProblem("Aún no hay escalas aplicadas.")
				}
				return aiPlan{Instr: "Describe la evolución de los puntajes de las escalas (las cifras y las categorías ya vienen calculadas). `summary`: 2 oraciones. `sections`: «Tendencia» y «Podría explorarse en sesión». No diagnostiques ni valores riesgo suicida: eso es del terapeuta; recuérdalo en una línea. `text` y `lines` vacíos.", Data: sc, Computed: sc}
			},
		},
		{
			Key: "therapy_plan_draft", Label: "Borrador de objetivos y tareas", Desc: "Propone objetivos terapéuticos y tareas entre sesiones.", Giros: []string{"PSYCHOLOGY"}, Subject: "person", MinNotes: 2,
			Build: func(c *aiCtx, param string) aiPlan {
				return aiPlan{Instr: "Con las notas de sesión, propone un borrador. `sections`: «Objetivos posibles» (3 a 5, formulados de forma medible) y «Tareas entre sesiones» (3 a 5, concretas). Basa todo en lo que consta en las notas. `text` y `lines` vacíos.", Data: c.notesBlock(6, 600) + "\n" + c.therapyBlock()}
			},
		},

		// ---- dermatology ----
		{
			Key: "skin_routine", Label: "Rutina de cuidado de la piel", Desc: "Borrador de rutina de mañana y noche según la valoración.", Giros: []string{"DERMATOLOGY"}, Subject: "person", MinNotes: 1, Draft: true,
			Build: func(c *aiCtx, param string) aiPlan {
				return aiPlan{Instr: "Redacta una rutina de cuidado de la piel para " + c.audience() + " según la valoración y el plan. `sections`: «Mañana», «Noche», «Cada semana» y «Evita». Usa tipos de producto (limpiador suave, humectante, protector solar) y solo los activos que el profesional haya indicado. En `text` el mismo contenido para imprimir. `lines` vacío.", Data: c.notesBlock(2, 500)}
			},
		},

		// ---- gynecology ----
		{
			Key: "prenatal_summary", Label: "Resumen del control prenatal", Desc: "Semanas, altura uterina y puntos a vigilar, con cifras calculadas.", Giros: []string{"GYNECOLOGY"}, Subject: "person",
			Build: func(c *aiCtx, param string) aiPlan {
				blk, ok := c.prenatalBlock()
				if !ok {
					return aiProblem("Registra la fecha de última menstruación (o la fecha probable de parto) en el control prenatal.")
				}
				return aiPlan{Instr: "Resume el control prenatal con las cifras ya calculadas. `summary`: 2 oraciones. `sections`: «Estado actual», «A vigilar» y «Pendientes del control». `text` y `lines` vacíos.", Data: blk + "\n" + c.notesBlock(4, 400), Computed: blk}
			},
		},
		{
			Key: "prenatal_studies", Label: "Estudios según la semana", Desc: "Estudios y controles que suelen considerarse en esta etapa, contra los registrados.", Giros: []string{"GYNECOLOGY"}, Subject: "person",
			Build: func(c *aiCtx, param string) aiPlan {
				blk, ok := c.prenatalBlock()
				if !ok {
					return aiProblem("Registra la fecha de última menstruación (o la fecha probable de parto) en el control prenatal.")
				}
				studies := ""
				if cs := c.charts("prenatal", 1); len(cs) > 0 {
					studies = fmt.Sprint(cs[0].Data["studies"])
				}
				return aiPlan{Instr: "Con la semana de gestación calculada, lista como REFERENCIA GENERAL los estudios y controles que suelen considerarse en esta etapa y en la siguiente. `sections`: «Suelen considerarse ahora», «Próximas semanas» y «Ya constan en el registro» (compara con los estudios registrados). Aclara que el esquema final lo define el especialista según el caso. `text` y `lines` vacíos.", Data: blk + "\nESTUDIOS REGISTRADOS: " + studies, Computed: blk}
			},
		},

		// ---- rehabilitation (also see rehab_ai.go) ----
		{
			Key: "postop_rehab", Label: "Rehabilitación por etapas", Desc: "Propone las etapas de recuperación de una lesión o cirugía.", Giros: rehabGiros, Subject: "person",
			Param: &aiParam{Label: "Lesión o cirugía y fecha", Required: true, Hint: "Ej. reconstrucción de LCA rodilla derecha, 12 de agosto"},
			Build: func(c *aiCtx, param string) aiPlan {
				tr := c.measureTrends()
				return aiPlan{Instr: "Propone como referencia las etapas de rehabilitación para «" + param + "». Hoy es " + todayISO() + ": ubica en qué etapa podría estar según la fecha indicada. `sections`: una por etapa con «Objetivo», «Qué suele trabajarse» y «Criterios para avanzar». El especialista decide los tiempos reales y las restricciones del cirujano. `text` y `lines` vacíos.", Data: "Dolor y movilidad registrados:\n" + tr + "\n" + c.notesBlock(2, 300), Computed: tr}
			},
		},

		// ---- veterinary ----
		{
			Key: "vet_dose_check", Label: "Revisión de dosis por peso", Desc: "Compara tu dosis con los rangos de referencia por kilo.", Giros: []string{"VETERINARY"}, Subject: "animal",
			Param: &aiParam{Label: "Medicamento y dosis que usarás", Required: true, Hint: "Ej. Meloxicam 1.5 mg"},
			Build: func(c *aiCtx, param string) aiPlan {
				var weight *float64
				for _, e := range c.visibleNotes() {
					if w := fnum(e.Measures["weight_kg"]); w != nil && *w > 0 {
						weight = w
					}
				}
				if weight == nil {
					return aiProblem("Registra el peso del animal en una consulta para revisar la dosis.")
				}
				computed := fmt.Sprintf("Peso más reciente: %g kg.", *weight)
				if m := doseRe.FindStringSubmatch(param); m != nil {
					var mg float64
					fmt.Sscan(strings.Replace(m[1], ",", ".", 1), &mg)
					computed += fmt.Sprintf(" Dosis indicada: %g mg = %.2f mg/kg.", mg, mg / *weight)
				}
				return aiPlan{Instr: "El veterinario piensa usar: «" + param + "». Con el peso y los mg/kg ya calculados, indica como REFERENCIA el rango de dosis por kilo que suele citarse para ese fármaco en la especie, la vía y la frecuencia habituales, y si la dosis calculada parece estar dentro, por debajo o por encima. `sections`: «Comparación», «Precauciones» (especie, edad, medicación crónica, alergias) y «Verifica en tu formulario». Si no conoces el fármaco o la especie, dilo y no inventes un rango. `text` y `lines` vacíos.", Data: computed + "\n" + c.notesBlock(2, 300), Computed: computed}
			},
		},
		{
			Key: "vet_schedule", Label: "Calendario de vacunas y desparasitación", Desc: "Ordena lo vencido y lo próximo según lo aplicado.", Giros: []string{"VETERINARY"}, Subject: "animal",
			Build: func(c *aiCtx, param string) aiPlan {
				vb := c.vaccineBlock()
				return aiPlan{Instr: "Con lo aplicado y el esquema de referencia de la clínica, arma el calendario. Hoy es " + todayISO() + ". `sections`: «Vencidas o sin registro», «Próximas» y «Notas por edad o especie». Usa las fechas tal como constan. El esquema final lo define el veterinario. `text` y `lines` vacíos.", Data: vb, Computed: vb}
			},
		},
	}
}
