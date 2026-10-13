package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

// What a per-patient AI task can read. Everything is loaded on demand, once, and the numbers (trends, percentiles,
// weeks of pregnancy, counts) are computed here, never by the model. The patient's name and contact data are never read.

type aiCtx struct {
	s        *Server
	r        *http.Request
	p        *Principal
	id       string
	pat      patient
	kinds    []string
	notes    []encounter // visible, non-private, oldest first
	notesOK  bool
	chronicS string
	chartMem map[string][]aiChart
}

type aiChart struct {
	At   time.Time
	Data map[string]any
}

func str(v any) string { s, _ := v.(string); return strings.TrimSpace(s) }

func (c *aiCtx) isAnimal() bool { return c.pat.Subject == "animal" }

func (c *aiCtx) hasKind(k ...string) bool {
	for _, x := range c.kinds {
		for _, y := range k {
			if x == y {
				return true
			}
		}
	}
	return false
}

// visibleNotes returns the visible, non-private consultations, oldest first.
func (c *aiCtx) visibleNotes() []encounter {
	if c.notesOK {
		return c.notes
	}
	c.notesOK = true
	encs, err := c.s.visibleEncounters(c.r.Context(), c.p, c.id)
	if err != nil {
		return nil
	}
	for _, e := range encs {
		if !e.Hidden && !e.Private {
			c.notes = append(c.notes, e)
		}
	}
	sort.SliceStable(c.notes, func(i, j int) bool { return c.notes[i].OccurredAt.Before(c.notes[j].OccurredAt) })
	return c.notes
}

// lastNotes returns the n most recent notes, oldest first.
func (c *aiCtx) lastNotes(n int) []encounter {
	all := c.visibleNotes()
	if len(all) > n {
		return all[len(all)-n:]
	}
	return all
}

func (c *aiCtx) notesBlock(n, field int) string {
	var b strings.Builder
	for _, e := range c.lastNotes(n) {
		fmt.Fprintf(&b, "[%s · %s] Motivo: %s | Subjetivo: %s | Exploración: %s | Valoración: %s", e.OccurredAt.Format("2006-01-02"), e.Kind,
			truncateRunes(e.Reason, 200), truncateRunes(e.Subjective, field), truncateRunes(e.Exam, field), truncateRunes(e.Assessment, field))
		if len(e.DiagnosisCodes) > 0 {
			b.WriteString(" | CIE-10: " + strings.Join(e.DiagnosisCodes, ", "))
		}
		fmt.Fprintf(&b, " | Plan: %s", truncateRunes(e.Plan, field))
		if len(e.Measures) > 0 {
			ms, _ := json.Marshal(e.Measures)
			b.WriteString(" | Mediciones: " + string(ms))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// header is the fixed part of the patient's context: age, sex, allergies and chronic medication (no name).
func (c *aiCtx) header() string {
	var b strings.Builder
	if c.isAnimal() {
		b.WriteString("Paciente: animal")
		if sp, _ := c.pat.Profile["species"].(string); sp != "" {
			b.WriteString(" (" + sp + ")")
		}
	} else {
		b.WriteString("Paciente: persona")
	}
	if c.pat.Age != nil {
		fmt.Fprintf(&b, ", %d años", *c.pat.Age)
	}
	if c.pat.Sex != "" {
		fmt.Fprintf(&b, ", sexo %s", c.pat.Sex)
	}
	b.WriteString("\n")
	if al := patientAllergies(c.pat.Profile); len(al) > 0 {
		b.WriteString("Alergias registradas: " + strings.Join(al, ", ") + "\n")
	}
	if c.chronicS == "" {
		c.chronicS = "-"
		if list, err := c.s.chronicOf(c.r.Context(), c.s.db, c.p.ClinicID, c.id); err == nil {
			var meds []string
			for _, m := range list {
				if m.Active {
					meds = append(meds, strings.TrimSpace(m.Name+" "+m.Dose+" "+m.Frequency))
				}
			}
			if len(meds) > 0 {
				c.chronicS = strings.Join(meds, "; ")
			}
		}
	}
	if c.chronicS != "-" {
		b.WriteString("Medicación crónica activa: " + c.chronicS + "\n")
	}
	return b.String()
}

// charts returns the latest n charts of a kind (newest first).
func (c *aiCtx) charts(kind string, n int) []aiChart {
	if c.chartMem == nil {
		c.chartMem = map[string][]aiChart{}
	}
	if v, ok := c.chartMem[kind]; ok && len(v) >= n {
		return v[:n]
	}
	rows, err := c.s.db.Query(c.r.Context(), `SELECT data, created_at FROM patient_charts WHERE clinic_id=$1 AND patient_id=$2 AND kind=$3 ORDER BY created_at DESC LIMIT $4`, c.p.ClinicID, c.id, kind, max(n, 12))
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []aiChart
	for rows.Next() {
		var raw []byte
		var at time.Time
		if rows.Scan(&raw, &at) != nil {
			continue
		}
		var m map[string]any
		if json.Unmarshal(raw, &m) == nil && m != nil {
			out = append(out, aiChart{At: at, Data: m})
		}
	}
	c.chartMem[kind] = out
	if len(out) > n {
		return out[:n]
	}
	return out
}

// ---- computed numbers ----

func fnum(v any) *float64 {
	switch x := v.(type) {
	case float64:
		return &x
	case json.Number:
		if f, err := x.Float64(); err == nil {
			return &f
		}
	}
	return nil
}

// measureTrends: for each numeric measure of the schema, first and last value with dates and the change.
func (c *aiCtx) measureTrends() string {
	labels := map[string]string{}
	for _, f := range allMeasureFields(c.pat.Subject, c.kinds) {
		if f.Type == "number" {
			u := ""
			if f.Unit != "" {
				u = " " + f.Unit
			}
			labels[f.Key] = f.Label + u
		}
	}
	type pt struct {
		d string
		v float64
	}
	series := map[string][]pt{}
	for _, e := range c.visibleNotes() {
		for k := range labels {
			if v := fnum(e.Measures[k]); v != nil {
				series[k] = append(series[k], pt{e.OccurredAt.Format("2006-01-02"), *v})
			}
		}
	}
	keys := make([]string, 0, len(series))
	for k := range series {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		s := series[k]
		if len(s) == 1 {
			fmt.Fprintf(&b, "- %s: una sola medición, %g (%s).\n", labels[k], s[0].v, s[0].d)
			continue
		}
		f, l := s[0], s[len(s)-1]
		fmt.Fprintf(&b, "- %s: de %g (%s) a %g (%s); cambio %+g en %d mediciones.\n", labels[k], f.v, f.d, l.v, l.d, math.Round((l.v-f.v)*100)/100, len(s))
	}
	return b.String()
}

// labTrends: for each analyte with numeric results, first and last values, the latest flag and reference range.
func (c *aiCtx) labTrends() string {
	res, err := c.s.labLoadResults(c.r.Context(), c.s.db, c.p.ClinicID, `r.patient_id = $2 AND NOT EXISTS (SELECT 1 FROM lab_results s WHERE s.supersedes_id = r.id)`, c.id)
	if err != nil || len(res) == 0 {
		return ""
	}
	type acc struct {
		unit string
		vals []labResult
	}
	by := map[string]*acc{}
	var order []string
	for _, x := range res {
		if x.ValueNum == nil {
			continue
		}
		a := by[x.Analyte]
		if a == nil {
			a = &acc{unit: x.Unit}
			by[x.Analyte] = a
			order = append(order, x.Analyte)
		}
		a.vals = append(a.vals, x)
	}
	var b strings.Builder
	for i, name := range order {
		if i >= 30 {
			break
		}
		a := by[name]
		l := a.vals[len(a.vals)-1]
		ref := ""
		if l.RefLow != nil || l.RefHigh != nil {
			lo, hi := "", ""
			if l.RefLow != nil {
				lo = fmt.Sprintf("%g", *l.RefLow)
			}
			if l.RefHigh != nil {
				hi = fmt.Sprintf("%g", *l.RefHigh)
			}
			ref = fmt.Sprintf(" (referencia %s–%s)", lo, hi)
		}
		flag := ""
		if l.Flag != "" && l.Flag != "normal" {
			flag = " [" + l.Flag + "]"
		}
		if len(a.vals) == 1 {
			fmt.Fprintf(&b, "- %s: %g %s (%s)%s%s\n", name, *l.ValueNum, a.unit, l.ResultedAt.Format("2006-01-02"), ref, flag)
			continue
		}
		f := a.vals[0]
		fmt.Fprintf(&b, "- %s: de %g (%s) a %g %s (%s); cambio %+g en %d estudios%s%s\n", name, *f.ValueNum, f.ResultedAt.Format("2006-01-02"), *l.ValueNum, a.unit,
			l.ResultedAt.Format("2006-01-02"), math.Round((*l.ValueNum-*f.ValueNum)*100)/100, len(a.vals), ref, flag)
	}
	return b.String()
}

// growthBlock: weight, height, BMI and head circumference with the percentile of the latest measurement when the clinic
// has reference tables; the numbers always come from the system.
func (c *aiCtx) growthBlock() string {
	if c.isAnimal() || c.pat.BirthDate == nil {
		return ""
	}
	birth, err := time.Parse("2006-01-02", *c.pat.BirthDate)
	if err != nil {
		return ""
	}
	meas := []growthMeasurement{}
	for _, e := range c.visibleNotes() {
		g := growthMeasurement{Date: e.OccurredAt.Format("2006-01-02"), WeightKg: growthNumber(e.Measures["weight_kg"], 1500), HeightCm: growthNumber(e.Measures["height_cm"], 300), HeadCm: growthNumber(e.Measures["head_circumference"], 100)}
		if g.WeightKg == nil && g.HeightCm == nil && g.HeadCm == nil {
			continue
		}
		if g.WeightKg != nil && g.HeightCm != nil {
			b := growthRound(*g.WeightKg/math.Pow(*g.HeightCm/100, 2), 2)
			g.BMI = &b
		}
		g.AgeMonths = growthAgeMonths(&birth, e.OccurredAt)
		meas = append(meas, g)
	}
	if len(meas) == 0 {
		return ""
	}
	var b strings.Builder
	if am := growthAgeMonths(&birth, time.Now()); am != nil {
		fmt.Fprintf(&b, "Edad actual: %.1f meses.\n", *am)
	}
	refSex := map[string]string{"Hombre": "M", "Mujer": "F"}[c.pat.Sex]
	resp := map[string]any{}
	if refSex != "" {
		_ = c.s.growthAttachReferences(c.r, c.p.ClinicID, refSex, "", meas, resp)
	}
	names := map[string]string{"weight_for_age": "Peso para la edad", "length_height_for_age": "Talla para la edad", "bmi_for_age": "IMC para la edad", "head_circumference_for_age": "Perímetro cefálico para la edad"}
	if ind, ok := resp["indicators"].(map[string]growthIndicatorOut); ok {
		fmt.Fprintf(&b, "Referencia usada: %v.\n", resp["standard"])
		keys := make([]string, 0, len(ind))
		for k := range ind {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			pts := ind[k].Points
			if len(pts) == 0 {
				continue
			}
			l := pts[len(pts)-1]
			label := names[k]
			if label == "" {
				label = k
			}
			if l.Percentile != nil && l.Z != nil {
				line := fmt.Sprintf("- %s: %g %s el %s, percentil %.0f (z %.2f)", label, l.Value, ind[k].Unit, l.Date, *l.Percentile, *l.Z)
				if len(pts) > 1 {
					if f := pts[0]; f.Percentile != nil {
						line += fmt.Sprintf("; al inicio percentil %.0f (%s)", *f.Percentile, f.Date)
					}
				}
				b.WriteString(line + ".\n")
			} else {
				fmt.Fprintf(&b, "- %s: %g %s el %s (sin percentil: falta referencia para esa edad).\n", label, l.Value, ind[k].Unit, l.Date)
			}
		}
	} else {
		b.WriteString("(Sin tablas de referencia cargadas: no hay percentiles; solo se tienen las cifras.)\n")
		l := meas[len(meas)-1]
		if l.WeightKg != nil {
			fmt.Fprintf(&b, "- Peso: %g kg (%s)\n", *l.WeightKg, l.Date)
		}
		if l.HeightCm != nil {
			fmt.Fprintf(&b, "- Talla: %g cm (%s)\n", *l.HeightCm, l.Date)
		}
		if l.HeadCm != nil {
			fmt.Fprintf(&b, "- Perímetro cefálico: %g cm (%s)\n", *l.HeadCm, l.Date)
		}
	}
	return b.String()
}

// scaleBlock: the PHQ-9 / GAD-7 / PSS-10 results in order, per scale.
func (c *aiCtx) scaleBlock() string {
	cs := c.charts("scale", 40)
	if len(cs) == 0 {
		return ""
	}
	by := map[string][]string{}
	var order []string
	for i := len(cs) - 1; i >= 0; i-- {
		d := cs[i].Data
		name := strings.ToUpper(str(d["scale"]))
		if by[name] == nil {
			order = append(order, name)
		}
		sc := fnum(d["score"])
		if sc == nil {
			continue
		}
		by[name] = append(by[name], fmt.Sprintf("%s: %g (%s)", cs[i].At.Format("2006-01-02"), *sc, str(d["severity"])))
	}
	var b strings.Builder
	for _, n := range order {
		fmt.Fprintf(&b, "- %s: %s\n", n, strings.Join(by[n], " → "))
	}
	return b.String()
}

// therapyBlock: goals and tasks of the latest therapy plan, with counts.
func (c *aiCtx) therapyBlock() string {
	cs := c.charts("therapy_plan", 1)
	if len(cs) == 0 {
		return ""
	}
	d := cs[0].Data
	var b strings.Builder
	today := time.Now().Format("2006-01-02")
	counts := map[string]int{}
	goals, _ := d["goals"].([]any)
	for _, g := range goals {
		m, _ := g.(map[string]any)
		st := str(m["status"])
		counts[st]++
		fmt.Fprintf(&b, "- Objetivo [%s]: %s\n", map[string]string{"active": "en curso", "done": "logrado", "paused": "en pausa"}[st], truncateRunes(str(m["text"]), 200))
	}
	tasks, _ := d["tasks"].([]any)
	done, open, late := 0, 0, 0
	for _, t := range tasks {
		m, _ := t.(map[string]any)
		isDone, _ := m["done"].(bool)
		due := str(m["due"])
		switch {
		case isDone:
			done++
		case due != "" && due < today:
			late++
			open++
		default:
			open++
		}
		state := "pendiente"
		if isDone {
			state = "hecha"
		} else if due != "" && due < today {
			state = "vencida"
		}
		fmt.Fprintf(&b, "- Tarea [%s]%s: %s\n", state, map[bool]string{true: " (para " + due + ")", false: ""}[due != ""], truncateRunes(str(m["text"]), 200))
	}
	fmt.Fprintf(&b, "Resumen calculado: objetivos en curso %d, logrados %d, en pausa %d; tareas hechas %d, pendientes %d (vencidas %d).\n", counts["active"], counts["done"], counts["paused"], done, open, late)
	return b.String()
}

// pregnancyWeeks computes weeks of gestation from the FUM or the FPP of the latest prenatal chart.
func pregnancyWeeks(d map[string]any, at time.Time) (weeks float64, ok bool) {
	if f, err := time.Parse("2006-01-02", str(d["fum"])); err == nil {
		return at.Sub(f).Hours() / 24 / 7, true
	}
	if f, err := time.Parse("2006-01-02", str(d["fpp"])); err == nil {
		return 40 - f.Sub(at).Hours()/24/7, true
	}
	return 0, false
}

func (c *aiCtx) prenatalBlock() (string, bool) {
	cs := c.charts("prenatal", 1)
	if len(cs) == 0 {
		return "", false
	}
	d := cs[0].Data
	w, ok := pregnancyWeeks(d, time.Now())
	if !ok || w < 0 || w > 45 {
		return "", false
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Semanas de gestación hoy (calculadas): %d+%d.\n", int(w), int((w-math.Floor(w))*7))
	tri := "primer"
	if w >= 28 {
		tri = "tercer"
	} else if w >= 14 {
		tri = "segundo"
	}
	fmt.Fprintf(&b, "Trimestre: %s.\n", tri)
	if v := str(d["fpp"]); v != "" {
		b.WriteString("FPP: " + v + "\n")
	}
	if v := str(d["risk"]); v != "" {
		b.WriteString("Riesgo registrado: " + v + "\n")
	}
	if v := str(d["blood_type"]); v != "" {
		b.WriteString("Grupo sanguíneo: " + v + "\n")
	}
	if v := str(d["gestas"]); v != "" {
		b.WriteString("Gestas: " + v + "\n")
	}
	// fundal height of each visit against the weeks of that day (between weeks 20 and 36 it should be close to the weeks)
	for _, e := range c.visibleNotes() {
		fh := fnum(e.Measures["fundal_height"])
		if fh == nil {
			continue
		}
		wk, ok := pregnancyWeeks(d, e.OccurredAt)
		if !ok {
			continue
		}
		line := fmt.Sprintf("- %s: altura uterina %g cm, semana %.0f", e.OccurredAt.Format("2006-01-02"), *fh, wk)
		if wk >= 20 && wk <= 36 {
			if diff := *fh - wk; math.Abs(diff) > 2 {
				line += fmt.Sprintf(" (difiere %+.0f cm de lo esperado)", diff)
			} else {
				line += " (dentro de lo esperado ±2 cm)"
			}
		}
		b.WriteString(line + "\n")
	}
	return b.String(), true
}

// vaccineBlock: for each vaccine/deworming suggested for the species, the last application and when it is due.
func (c *aiCtx) vaccineBlock() string {
	rows, err := c.s.db.Query(c.r.Context(), `SELECT name, to_char(applied_on,'YYYY-MM-DD'), coalesce(to_char(next_due,'YYYY-MM-DD'),'') FROM vaccinations
		WHERE clinic_id=$1 AND patient_id=$2 AND voided_at IS NULL ORDER BY applied_on DESC LIMIT 300`, c.p.ClinicID, c.id)
	if err != nil {
		return ""
	}
	defer rows.Close()
	type app struct{ name, on, due string }
	var apps []app
	for rows.Next() {
		var a app
		if rows.Scan(&a.name, &a.on, &a.due) == nil {
			apps = append(apps, a)
		}
	}
	species, _ := c.pat.Profile["species"].(string)
	today := time.Now().Format("2006-01-02")
	var b strings.Builder
	for _, a := range apps {
		fmt.Fprintf(&b, "- Aplicada: %s el %s", a.name, a.on)
		if a.due != "" {
			state := "próxima"
			if a.due < today {
				state = "VENCIDA desde"
			}
			fmt.Fprintf(&b, " · %s %s", state, a.due)
		}
		b.WriteString("\n")
	}
	if sg := c.s.vaccineSuggestionsFor(c.pat.Subject, species); len(sg) > 0 {
		b.WriteString("Esquema de referencia de la clínica (nombre · intervalo en días):\n")
		for _, x := range sg {
			fmt.Fprintf(&b, "- %s · %d días\n", x.Name, x.IntervalDays)
		}
	}
	return b.String()
}

func odontogramBlock(d map[string]any) string {
	teeth, _ := d["teeth"].(map[string]any)
	keys := make([]string, 0, len(teeth))
	for k := range teeth {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		t, _ := teeth[k].(map[string]any)
		var parts []string
		if s := str(t["state"]); s != "" {
			parts = append(parts, s)
		}
		surf, _ := t["surfaces"].(map[string]any)
		sk := make([]string, 0, len(surf))
		for x := range surf {
			sk = append(sk, x)
		}
		sort.Strings(sk)
		for _, x := range sk {
			parts = append(parts, fmt.Sprintf("%s en %s", str(surf[x]), x))
		}
		if n := str(t["note"]); n != "" {
			parts = append(parts, "nota: "+truncateRunes(n, 100))
		}
		if len(parts) > 0 {
			fmt.Fprintf(&b, "- Pieza %s: %s\n", k, strings.Join(parts, "; "))
		}
	}
	return b.String()
}

// perioStats counts the sites by probing depth, bleeding, mobility and furcation.
func perioStats(d map[string]any) string {
	teeth, _ := d["teeth"].(map[string]any)
	present, sites, pd4, pd6, bop, bopSites, mob, furc := 0, 0, 0, 0, 0, 0, 0, 0
	type worst struct {
		tooth string
		pd    float64
	}
	var ws []worst
	for k, v := range teeth {
		t, _ := v.(map[string]any)
		if abs, _ := t["absent"].(bool); abs {
			continue
		}
		present++
		mx := 0.0
		pds, _ := t["pd"].([]any)
		for _, x := range pds {
			if f := fnum(x); f != nil {
				sites++
				if *f >= 4 {
					pd4++
				}
				if *f >= 6 {
					pd6++
				}
				mx = math.Max(mx, *f)
			}
		}
		bops, _ := t["bop"].([]any)
		for _, x := range bops {
			bopSites++
			if bl, _ := x.(bool); bl {
				bop++
			}
		}
		if m := fnum(t["mob"]); m != nil && *m > 0 {
			mob++
		}
		if m := fnum(t["furc"]); m != nil && *m > 0 {
			furc++
		}
		ws = append(ws, worst{k, mx})
	}
	sort.Slice(ws, func(i, j int) bool { return ws[i].pd > ws[j].pd || (ws[i].pd == ws[j].pd && ws[i].tooth < ws[j].tooth) })
	var b strings.Builder
	fmt.Fprintf(&b, "Piezas presentes: %d. Sitios sondeados: %d; con profundidad ≥4 mm: %d; ≥6 mm: %d.\n", present, sites, pd4, pd6)
	if bopSites > 0 {
		fmt.Fprintf(&b, "Sangrado al sondaje: %d de %d sitios (%.0f %%).\n", bop, bopSites, float64(bop)*100/float64(bopSites))
	}
	fmt.Fprintf(&b, "Piezas con movilidad: %d. Piezas con furca afectada: %d.\n", mob, furc)
	if p := fnum(d["plaque_pct"]); p != nil {
		fmt.Fprintf(&b, "Índice de placa registrado: %g %%.\n", *p)
	}
	if p := fnum(d["bleeding_pct"]); p != nil {
		fmt.Fprintf(&b, "Índice de sangrado registrado: %g %%.\n", *p)
	}
	if len(ws) > 0 && ws[0].pd >= 4 {
		var top []string
		for i, w := range ws {
			if i >= 6 || w.pd < 4 {
				break
			}
			top = append(top, fmt.Sprintf("%s (%g mm)", w.tooth, w.pd))
		}
		b.WriteString("Piezas con mayor profundidad: " + strings.Join(top, ", ") + ".\n")
	}
	return b.String()
}

func (c *aiCtx) recallBlock() string {
	cs := c.charts("food_recall", 3)
	var b strings.Builder
	for i := len(cs) - 1; i >= 0; i-- {
		d := cs[i].Data
		fmt.Fprintf(&b, "Recordatorio (%s, %s):\n", str(d["date"]), map[string]string{"24h": "24 horas", "dia_tipico": "día típico"}[str(d["kind"])])
		meals, _ := d["meals"].([]any)
		for _, m := range meals {
			mm, _ := m.(map[string]any)
			if it := str(mm["items"]); it != "" {
				fmt.Fprintf(&b, "  %s %s: %s\n", str(mm["name"]), str(mm["time"]), truncateRunes(it, 300))
			}
		}
		if w := str(d["water"]); w != "" {
			fmt.Fprintf(&b, "  Agua: %s\n", w)
		}
		if n := str(d["notes"]); n != "" {
			fmt.Fprintf(&b, "  Notas: %s\n", truncateRunes(n, 200))
		}
	}
	return b.String()
}

func (c *aiCtx) nutritionPlanBlock() string {
	cs := c.charts("nutrition_plan", 2)
	var b strings.Builder
	for i := len(cs) - 1; i >= 0; i-- {
		d := cs[i].Data
		fmt.Fprintf(&b, "Plan del %s: objetivo «%s», %v kcal diarias (carbohidratos %v %%, proteínas %v %%, grasas %v %%).\n", cs[i].At.Format("2006-01-02"), truncateRunes(str(d["goal"]), 120), d["kcal"], d["carb_pct"], d["protein_pct"], d["fat_pct"])
	}
	return b.String()
}
