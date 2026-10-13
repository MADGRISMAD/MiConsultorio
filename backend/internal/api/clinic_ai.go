package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// AI helpers of the clinic (not of one patient): the summary of a period, the patients who stopped coming, a reply to a
// satisfaction survey and the texts of the public page. The model never receives patient names; the numbers are computed
// here. Each use costs one magic use and comes back as a draft or suggestion.

func (s *Server) mountClinicAI(r chi.Router) {
	admin := require(PermAdminUsers)
	r.With(admin).Post("/ai/clinic/period-summary", s.aiPeriodSummary)
	r.With(require(PermAdminAppointments, PermAdminUsers)).Post("/ai/clinic/at-risk", s.aiAtRisk)
	r.With(admin).Post("/ai/clinic/review-reply", s.aiReviewReply)
	r.With(admin).Post("/ai/clinic/public-page", s.aiPublicPage)
}

func (s *Server) clinicGiros(r *http.Request, p *Principal) string {
	kinds, err := s.clinicKindsFor(r.Context(), p.ClinicID)
	if err != nil {
		return ""
	}
	var names []string
	for _, k := range kinds {
		if l := areaLabels[k]; l != "" {
			names = append(names, l)
		}
	}
	return strings.Join(names, ", ")
}

// ---- period summary ----

func pctChange(cur, prev float64) string {
	if prev == 0 {
		return "sin base de comparación"
	}
	return fmt.Sprintf("%+.0f %% frente al periodo anterior", (cur-prev)*100/prev)
}

func (s *Server) aiPeriodSummary(w http.ResponseWriter, r *http.Request) {
	if !s.aiReady(w) {
		return
	}
	var in struct {
		Range string `json:"range"`
	}
	if !decode(w, r, &in) {
		return
	}
	days := map[string]int{"hoy": 1, "semana": 7, "mes": 30}[in.Range]
	if days == 0 {
		writeError(w, http.StatusBadRequest, "Elige el periodo: hoy, semana o mes.")
		return
	}
	p := principalFrom(r.Context())
	today := time.Now().Format("2006-01-02")
	from := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	win, msg := rptParseWindow(func(k string) string { return map[string]string{"from": from, "to": today}[k] })
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	cobros := false
	if p.Billing != nil {
		if plan, ok := planByID(p.Billing.Plan); ok {
			cobros = plan.Cobros
		}
	}
	cur, _, err := s.indicatorPeriod(r.Context(), p.ClinicID, win, cobros)
	if err != nil {
		serverError(w, r, err)
		return
	}
	pf := win.From.AddDate(0, 0, -win.Days)
	prevWin := rptWindow{From: pf, To: win.From, FromDay: pf.Format("2006-01-02"), ToDay: win.From.AddDate(0, 0, -1).Format("2006-01-02"), Days: win.Days, TZ: win.TZ}
	prev, _, err := s.indicatorPeriod(r.Context(), p.ClinicID, prevWin, cobros)
	if err != nil {
		serverError(w, r, err)
		return
	}
	var upcoming, atRisk int
	_ = s.db.QueryRow(r.Context(), `SELECT count(*) FROM appointments WHERE clinic_id=$1 AND date > $2::date AND date <= ($2::date + 7) AND status IN ('scheduled','confirmed')`, p.ClinicID, today).Scan(&upcoming)
	_ = s.db.QueryRow(r.Context(), atRiskCount, p.ClinicID, today).Scan(&atRisk)

	var b strings.Builder
	fmt.Fprintf(&b, "Periodo: %s a %s (%d día%s). Se compara con los %d días anteriores.\n", win.FromDay, win.ToDay, win.Days, map[bool]string{true: "s", false: ""}[win.Days > 1], win.Days)
	line := func(label string, c, p float64, unit string) {
		fmt.Fprintf(&b, "- %s: %g%s (antes %g%s; %s)\n", label, c, unit, p, unit, pctChange(c, p))
	}
	line("Consultas registradas", float64(cur.Encounters), float64(prev.Encounters), "")
	line("Citas", float64(cur.Appointments), float64(prev.Appointments), "")
	line("Citas completadas", float64(cur.Completed), float64(prev.Completed), "")
	line("Pacientes nuevos", float64(cur.NewPatients), float64(prev.NewPatients), "")
	line("Pacientes que regresan", float64(cur.ReturningPatient), float64(prev.ReturningPatient), "")
	fmt.Fprintf(&b, "- Cancelaciones: %.0f %% (antes %.0f %%)\n- Inasistencias: %.0f %% (antes %.0f %%)\n", cur.CancellationRate, prev.CancellationRate, cur.NoShowRate, prev.NoShowRate)
	if cur.OccupancyRate != nil {
		fmt.Fprintf(&b, "- Ocupación de la agenda: %.0f %%\n", *cur.OccupancyRate)
	}
	if cobros {
		line("Ingresos (pesos)", float64(cur.RevenueCents)/100, float64(prev.RevenueCents)/100, "")
		fmt.Fprintf(&b, "- Ventas: %d; ticket promedio %.0f pesos\n", cur.Sales, float64(cur.AvgTicketCents)/100)
	}
	if cur.SatisfactionN > 0 {
		fmt.Fprintf(&b, "- Satisfacción: %.1f de 5 en %d respuestas\n", cur.SatisfactionAvg, cur.SatisfactionN)
	}
	fmt.Fprintf(&b, "- Citas programadas en los próximos 7 días: %d\n- Pacientes que dejaron de venir (sin cita futura): %d\n", upcoming, atRisk)
	data := b.String()
	instr := "Resume cómo va el consultorio en el periodo con las cifras ya calculadas. `summary`: 2 o 3 oraciones con lo más importante. `sections`: «Lo que va bien», «Lo que vale la pena revisar» y «Podría considerarse» (acciones concretas, p. ej. recordar citas si suben las inasistencias). Cita las cifras tal cual; si una cifra no tiene comparación, dilo. `text` y `lines` vacíos."
	s.finishAI(w, r, p, "", "period_summary", "Resumen del periodo", false, aiPrompt(instr, "Consultorio de: "+s.clinicGiros(r, p)+"\n", data), data, nil)
}

// ---- patients who stopped coming ----

const atRiskWhere = `p.clinic_id = $1 AND p.archived_at IS NULL AND p.last_encounter_at IS NOT NULL
	AND p.last_encounter_at < $2::date - 90 AND p.last_encounter_at > $2::date - 730
	AND NOT EXISTS (SELECT 1 FROM appointments a WHERE a.clinic_id = p.clinic_id AND a.patient_id = p.id AND a.date >= $2::date AND a.status IN ('scheduled', 'confirmed'))`

const atRiskCount = `SELECT count(*) FROM patients p WHERE ` + atRiskWhere

type atRiskPatient struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	LastVisit   string `json:"last_visit"`
	Days        int    `json:"days"`
	RemindersOK bool   `json:"reminders_ok"`
}

func (s *Server) aiAtRisk(w http.ResponseWriter, r *http.Request) {
	if !s.aiReady(w) {
		return
	}
	p := principalFrom(r.Context())
	today := time.Now().Format("2006-01-02")
	rows, err := s.db.Query(r.Context(), `SELECT p.id::text, trim(p.names || ' ' || p.last_names), p.last_encounter_at, p.reminders_ok FROM patients p WHERE `+atRiskWhere+` ORDER BY p.last_encounter_at DESC LIMIT 60`, p.ClinicID, today)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	list := []atRiskPatient{}
	seg := [3]int{}
	for rows.Next() {
		var x atRiskPatient
		var at time.Time
		if err := rows.Scan(&x.ID, &x.Name, &at, &x.RemindersOK); err != nil {
			serverError(w, r, err)
			return
		}
		x.LastVisit, x.Days = at.Format("2006-01-02"), int(time.Since(at).Hours()/24)
		switch {
		case x.Days < 180:
			seg[0]++
		case x.Days < 365:
			seg[1]++
		default:
			seg[2]++
		}
		list = append(list, x)
	}
	if rows.Err() != nil {
		serverError(w, r, rows.Err())
		return
	}
	if len(list) == 0 {
		writeError(w, http.StatusBadRequest, "No hay pacientes que hayan dejado de venir (más de 3 meses sin consulta y sin cita próxima).")
		return
	}
	data := fmt.Sprintf("Pacientes sin consulta en los últimos 3 meses y sin cita próxima: %d (se listan hasta 60).\n- De 3 a 6 meses sin venir: %d\n- De 6 a 12 meses: %d\n- Más de 12 meses: %d\n", len(list), seg[0], seg[1], seg[2])
	instr := "Propone cómo reconectar con estos pacientes. `summary`: 2 oraciones. `lines`: 2 mensajes listos para enviar (`name` = el tono, p. ej. «Recordatorio amable» y «Retomar el seguimiento»; `detail` = el mensaje de máximo 280 caracteres, que empiece con «Hola [nombre],», invite a agendar sin presionar, no mencione diagnósticos ni datos clínicos y se firme «El equipo del consultorio»). `sections`: «Cómo priorizar» con 3 ideas. `text` vacío."
	prompt := aiPrompt(instr, "Consultorio de: "+s.clinicGiros(r, p)+"\n", data)
	s.finishAI(w, r, p, "", "at_risk", "Pacientes que dejaron de venir", true, prompt, data, func(o *aiOut) { o.Extra = list })
}

// ---- reply to a survey ----

func (s *Server) aiReviewReply(w http.ResponseWriter, r *http.Request) {
	if !s.aiReady(w) {
		return
	}
	var in struct {
		ID string `json:"id"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !validUUID(in.ID) {
		writeError(w, http.StatusNotFound, "Respuesta no encontrada.")
		return
	}
	p := principalFrom(r.Context())
	var rating int
	var comment string
	err := s.db.QueryRow(r.Context(), `SELECT rating, comment FROM satisfaction_surveys WHERE clinic_id=$1 AND id=$2 AND answered_at IS NOT NULL`, p.ClinicID, in.ID).Scan(&rating, &comment)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Respuesta no encontrada.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	data := fmt.Sprintf("Calificación: %d de 5.\n", rating)
	if c := strings.TrimSpace(comment); c != "" {
		data += "Comentario del paciente: " + truncateRunes(c, 800) + "\n"
	} else {
		data += "El paciente no dejó comentario.\n"
	}
	instr := "Redacta en `text` la respuesta pública del consultorio a esta calificación, de máximo 450 caracteres. Agradece con calidez; si la calificación es de 3 o menos, ofrece una disculpa sincera y invita a comunicarse con el consultorio para resolverlo. No menciones ni confirmes que la persona es paciente, no incluyas datos clínicos ni nombres, no prometas nada que no puedas cumplir y no discutas. Firma «El equipo del consultorio». `sections`, `lines` y `summary` vacíos."
	s.finishAI(w, r, p, "", "review_reply", "Respuesta a una calificación", true, aiPrompt(instr, "", data), "", nil)
}

// ---- texts of the public page ----

func (s *Server) aiPublicPage(w http.ResponseWriter, r *http.Request) {
	if !s.aiReady(w) {
		return
	}
	p := principalFrom(r.Context())
	var name, city, state string
	_ = s.db.QueryRow(r.Context(), `SELECT c.name, coalesce(cp.city,''), coalesce(cp.state,'') FROM clinics c LEFT JOIN clinic_profile cp ON cp.clinic_id = c.id WHERE c.id=$1`, p.ClinicID).Scan(&name, &city, &state)
	rows, err := s.db.Query(r.Context(), `SELECT name FROM catalog_items WHERE clinic_id=$1 AND kind='service' AND active ORDER BY featured DESC, name LIMIT 20`, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	var services []string
	for rows.Next() {
		var n string
		if rows.Scan(&n) == nil {
			services = append(services, n)
		}
	}
	data := "Nombre del consultorio: " + name + "\nÁreas: " + s.clinicGiros(r, p) + "\n"
	if city != "" {
		data += "Ciudad: " + city + ", " + state + "\n"
	}
	if len(services) > 0 {
		data += "Servicios: " + strings.Join(services, ", ") + "\n"
	}
	instr := "Redacta los textos de la página pública del consultorio para pacientes. `title`: un lema de máximo 90 caracteres. `text`: «Sobre el consultorio», 2 párrafos cortos (máximo 700 caracteres en total), cálidos y claros. `sections`: una sola llamada «Motivos para elegirnos» con 4 a 6 motivos de máximo 80 caracteres. `lines`: 5 preguntas frecuentes (`name` = pregunta, `detail` = respuesta de máximo 250 caracteres; usa respuestas generales como cómo agendar, qué llevar a la primera cita o cómo cancelar, sin precios ni horarios concretos). " +
		"PROHIBIDO inventar: años de experiencia, premios, certificaciones, equipo médico, precios, horarios, garantías o promesas de resultados, y nada de «el mejor» ni superlativos. Usa solo lo que consta en los datos. `summary` vacío."
	s.finishAI(w, r, p, "", "public_page", "Textos de la página pública", true, aiPrompt(instr, "", data), "", nil)
}
