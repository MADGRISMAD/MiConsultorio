package api

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// Clinical report: who the clinic sees, who stopped coming and why people consult.
// Visible to administrators only (it carries guardian contact data for reactivation).

type rptMonth struct {
	Month     string `json:"month"` // YYYY-MM
	New       int    `json:"new"`
	Returning int    `json:"returning"`
}

type rptSignup struct {
	Month  string `json:"month"`
	Person int    `json:"person"`
	Animal int    `json:"animal"`
}

type rptContact struct {
	ID          string  `json:"id"`
	FileNumber  int     `json:"file_number"`
	Name        string  `json:"name"`
	Subject     string  `json:"subject"`
	LastVisit   *string `json:"last_visit"`
	DaysSince   *int    `json:"days_since"`
	ContactName string  `json:"contact_name"`
	Phone       string  `json:"contact_phone"`
	Email       string  `json:"contact_email"`
	RemindersOK bool    `json:"reminders_ok"`
}

type rptVaccine struct {
	rptContact
	Vaccine  string `json:"vaccine"`
	NextDue  string `json:"next_due"`
	Overdue  bool   `json:"overdue"`
	DaysLate int    `json:"days_late"`
}

type rptAgeRow struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Female  int    `json:"female"`
	Male    int    `json:"male"`
	Other   int    `json:"other"`
	Unknown int    `json:"unknown"`
	Total   int    `json:"total"`
}

type rptPatientsReport struct {
	ClinicKind string `json:"clinic_kind"`
	Visits     struct {
		Patients       int        `json:"patients"` // distinct patients with a consultation in the range
		Repeat         int        `json:"repeat"`   // with 2 or more consultations in the range
		ByMonth        []rptMonth `json:"by_month"`
		NewTotal       int        `json:"new_total"`
		ReturningTotal int        `json:"returning_total"`
	} `json:"visits"`
	Signups struct {
		Total   int         `json:"total"`
		Person  int         `json:"person"`
		Animal  int         `json:"animal"`
		ByMonth []rptSignup `json:"by_month"`
	} `json:"signups"`
	Inactive struct {
		Days  int          `json:"days"`
		Total int          `json:"total"`
		Page  int          `json:"page"`
		Limit int          `json:"limit"`
		Items []rptContact `json:"items"`
	} `json:"inactive"`
	Vaccines struct {
		Available bool         `json:"available"`
		Total     int          `json:"total"`
		Items     []rptVaccine `json:"items"`
	} `json:"vaccines"`
	Reasons []rptCount `json:"reasons"`
	People  struct {
		Total int         `json:"total"`
		ByAge []rptAgeRow `json:"by_age"`
	} `json:"people"`
	Animals struct {
		Total     int        `json:"total"`
		BySpecies []rptCount `json:"by_species"`
		BySex     []rptCount `json:"by_sex"`
	} `json:"animals"`
}

const rptConsultKinds = `('consulta','seguimiento','procedimiento')`

const rptContactCols = `p.id::text, p.file_number, trim(p.names || ' ' || p.last_names), p.subject,
	coalesce(nullif(p.guardian_name, ''), trim(p.names || ' ' || p.last_names)),
	coalesce(nullif(p.guardian_phone, ''), p.phone), coalesce(nullif(p.guardian_email, ''), p.email), p.reminders_ok`

func (s *Server) rptPatients(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	win, msg := rptParseWindow(r.URL.Query().Get)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	days, page, limit, msg := rptInactiveParams(r)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	rep, err := s.rptBuildPatients(r.Context(), p.ClinicID, win, days, page, limit)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"report": rep, "from": win.FromDay, "to": win.ToDay})
}

func rptInactiveParams(r *http.Request) (days, page, limit int, msg string) {
	q := r.URL.Query()
	days, page, limit = 180, 1, 25
	for _, f := range []struct {
		name     string
		dst      *int
		min, max int
	}{{"inactive_days", &days, 30, 1500}, {"page", &page, 1, 100000}, {"limit", &limit, 1, 100}} {
		if v := q.Get(f.name); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < f.min || n > f.max {
				return 0, 0, 0, "El parámetro " + f.name + " no es válido."
			}
			*f.dst = n
		}
	}
	return days, page, limit, ""
}

func (s *Server) rptBuildPatients(ctx context.Context, clinicID string, win rptWindow, days, page, limit int) (*rptPatientsReport, error) {
	rep := &rptPatientsReport{}
	rep.Visits.ByMonth, rep.Signups.ByMonth = []rptMonth{}, []rptSignup{}
	rep.Inactive.Items, rep.Vaccines.Items = []rptContact{}, []rptVaccine{}
	rep.Reasons, rep.People.ByAge = []rptCount{}, []rptAgeRow{}
	rep.Animals.BySpecies, rep.Animals.BySex = []rptCount{}, []rptCount{}
	rep.Vaccines.Available = true
	tz := win.TZ

	if err := s.db.QueryRow(ctx, `SELECT kind FROM clinics WHERE id = $1`, clinicID).Scan(&rep.ClinicKind); err != nil {
		return nil, err
	}

	// new vs returning: "new" = the month holds the patient's first consultation ever
	rows, err := s.db.Query(ctx, `
		WITH firsts AS (
			SELECT patient_id, min(occurred_at) AS f FROM encounters
			WHERE clinic_id = $1 AND addendum_of IS NULL AND kind IN `+rptConsultKinds+` GROUP BY patient_id),
		seen AS (
			SELECT DISTINCT patient_id, date_trunc('month', occurred_at AT TIME ZONE $4) AS m FROM encounters
			WHERE clinic_id = $1 AND occurred_at >= $2 AND occurred_at < $3 AND addendum_of IS NULL AND kind IN `+rptConsultKinds+`)
		SELECT to_char(seen.m, 'YYYY-MM'),
		       count(*) FILTER (WHERE date_trunc('month', f.f AT TIME ZONE $4) = seen.m),
		       count(*) FILTER (WHERE date_trunc('month', f.f AT TIME ZONE $4) <> seen.m)
		FROM seen JOIN firsts f USING (patient_id) GROUP BY 1 ORDER BY 1`, clinicID, win.From, win.To, tz)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var m rptMonth
		if err := rows.Scan(&m.Month, &m.New, &m.Returning); err != nil {
			rows.Close()
			return nil, err
		}
		rep.Visits.NewTotal += m.New
		rep.Visits.ReturningTotal += m.Returning
		rep.Visits.ByMonth = append(rep.Visits.ByMonth, m)
	}
	rows.Close()
	if err := s.db.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE n >= 2) FROM (
			SELECT count(*) AS n FROM encounters WHERE clinic_id = $1 AND occurred_at >= $2 AND occurred_at < $3
			  AND addendum_of IS NULL AND kind IN `+rptConsultKinds+` GROUP BY patient_id) t`,
		clinicID, win.From, win.To).Scan(&rep.Visits.Patients, &rep.Visits.Repeat); err != nil {
		return nil, err
	}

	// signups
	rows, err = s.db.Query(ctx, `
		SELECT to_char(date_trunc('month', created_at AT TIME ZONE $4), 'YYYY-MM'),
		       count(*) FILTER (WHERE subject = 'person'), count(*) FILTER (WHERE subject = 'animal')
		FROM patients WHERE clinic_id = $1 AND created_at >= $2 AND created_at < $3 GROUP BY 1 ORDER BY 1`, clinicID, win.From, win.To, tz)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x rptSignup
		if err := rows.Scan(&x.Month, &x.Person, &x.Animal); err != nil {
			rows.Close()
			return nil, err
		}
		rep.Signups.Person += x.Person
		rep.Signups.Animal += x.Animal
		rep.Signups.ByMonth = append(rep.Signups.ByMonth, x)
	}
	rows.Close()
	rep.Signups.Total = rep.Signups.Person + rep.Signups.Animal

	// inactive patients
	rep.Inactive.Days, rep.Inactive.Page, rep.Inactive.Limit = days, page, limit
	items, total, err := s.rptInactive(ctx, clinicID, days, limit, (page-1)*limit)
	if err != nil {
		return nil, err
	}
	rep.Inactive.Items, rep.Inactive.Total = items, total

	// vaccines due
	rows, err = s.db.Query(ctx, `
		SELECT `+rptContactCols+`, x.name, to_char(x.next_due, 'YYYY-MM-DD'), (CURRENT_DATE - x.next_due)::int, count(*) OVER ()
		FROM (SELECT DISTINCT ON (v.patient_id, lower(v.name)) v.patient_id, v.name, v.next_due
		      FROM vaccinations v WHERE v.clinic_id = $1 AND v.voided_at IS NULL
		      ORDER BY v.patient_id, lower(v.name), v.applied_on DESC, v.created_at DESC) x
		JOIN patients p ON p.id = x.patient_id AND p.clinic_id = $1 AND p.archived_at IS NULL
		WHERE x.next_due IS NOT NULL AND x.next_due <= CURRENT_DATE + 30
		ORDER BY x.next_due, p.file_number LIMIT 100`, clinicID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v rptVaccine
		if err := rows.Scan(&v.ID, &v.FileNumber, &v.Name, &v.Subject, &v.ContactName, &v.Phone, &v.Email, &v.RemindersOK,
			&v.Vaccine, &v.NextDue, &v.DaysLate, &rep.Vaccines.Total); err != nil {
			rows.Close()
			return nil, err
		}
		v.Overdue = v.DaysLate > 0
		rep.Vaccines.Items = append(rep.Vaccines.Items, v)
	}
	rows.Close()

	// top reasons (private notes are never read)
	rows, err = s.db.Query(ctx, `
		SELECT regexp_replace(translate(lower(btrim(reason)), 'áéíóúü', 'aeiouu'), '\s+', ' ', 'g') AS r, count(*)
		FROM encounters WHERE clinic_id = $1 AND occurred_at >= $2 AND occurred_at < $3 AND addendum_of IS NULL
		  AND NOT private AND kind IN `+rptConsultKinds+` AND btrim(reason) <> ''
		GROUP BY 1 ORDER BY 2 DESC, 1 LIMIT 15`, clinicID, win.From, win.To)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var c rptCount
		if err := rows.Scan(&c.Label, &c.Count); err != nil {
			rows.Close()
			return nil, err
		}
		c.Key = c.Label
		rep.Reasons = append(rep.Reasons, c)
	}
	rows.Close()

	// distribution of current (non archived) patients
	bands := []struct{ key, label string }{{"0-11", "0 a 11"}, {"12-17", "12 a 17"}, {"18-29", "18 a 29"}, {"30-44", "30 a 44"},
		{"45-59", "45 a 59"}, {"60-74", "60 a 74"}, {"75+", "75 o más"}, {"unknown", "Sin fecha"}}
	idx := map[string]*rptAgeRow{}
	for _, b := range bands {
		rep.People.ByAge = append(rep.People.ByAge, rptAgeRow{Key: b.key, Label: b.label})
	}
	for i := range rep.People.ByAge {
		idx[rep.People.ByAge[i].Key] = &rep.People.ByAge[i]
	}
	rows, err = s.db.Query(ctx, `
		SELECT CASE WHEN birth_date IS NULL THEN 'unknown'
		            WHEN date_part('year', age(birth_date)) < 12 THEN '0-11' WHEN date_part('year', age(birth_date)) < 18 THEN '12-17'
		            WHEN date_part('year', age(birth_date)) < 30 THEN '18-29' WHEN date_part('year', age(birth_date)) < 45 THEN '30-44'
		            WHEN date_part('year', age(birth_date)) < 60 THEN '45-59' WHEN date_part('year', age(birth_date)) < 75 THEN '60-74'
		            ELSE '75+' END, sex, count(*)
		FROM patients WHERE clinic_id = $1 AND archived_at IS NULL AND subject = 'person' GROUP BY 1, 2`, clinicID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var band, sex string
		var n int
		if err := rows.Scan(&band, &sex, &n); err != nil {
			rows.Close()
			return nil, err
		}
		row := idx[band]
		switch sex {
		case "Mujer":
			row.Female += n
		case "Hombre":
			row.Male += n
		case "Otro":
			row.Other += n
		default:
			row.Unknown += n
		}
		row.Total += n
		rep.People.Total += n
	}
	rows.Close()

	rows, err = s.db.Query(ctx, `
		SELECT coalesce(nullif(btrim(profile->>'species'), ''), 'Sin dato'), sex, count(*)
		FROM patients WHERE clinic_id = $1 AND archived_at IS NULL AND subject = 'animal' GROUP BY 1, 2`, clinicID)
	if err != nil {
		return nil, err
	}
	species, sexes := map[string]int{}, map[string]int{}
	for rows.Next() {
		var sp, sex string
		var n int
		if err := rows.Scan(&sp, &sex, &n); err != nil {
			rows.Close()
			return nil, err
		}
		species[sp] += n
		if sex == "" {
			sex = "Sin dato"
		}
		sexes[sex] += n
		rep.Animals.Total += n
	}
	rows.Close()
	rep.Animals.BySpecies = rptSortedCounts(species)
	rep.Animals.BySex = rptSortedCounts(sexes)
	return rep, nil
}

func rptSortedCounts(m map[string]int) []rptCount {
	out := make([]rptCount, 0, len(m))
	for k, n := range m {
		out = append(out, rptCount{Key: k, Label: k, Count: n})
	}
	for i := 1; i < len(out); i++ { // insertion sort: tiny lists
		for j := i; j > 0 && (out[j].Count > out[j-1].Count || (out[j].Count == out[j-1].Count && out[j].Label < out[j-1].Label)); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// rptInactive lists active patients with no visit in the last days days (never visited ones count from their sign-up).
func (s *Server) rptInactive(ctx context.Context, clinicID string, days, limit, offset int) ([]rptContact, int, error) {
	rows, err := s.db.Query(ctx, `
		SELECT `+rptContactCols+`, to_char(p.last_encounter_at AT TIME ZONE $5, 'YYYY-MM-DD'),
		       (CURRENT_DATE - (coalesce(p.last_encounter_at, p.created_at) AT TIME ZONE $5)::date)::int, count(*) OVER ()
		FROM patients p
		WHERE p.clinic_id = $1 AND p.archived_at IS NULL AND coalesce(p.last_encounter_at, p.created_at) < now() - make_interval(days => $2)
		ORDER BY coalesce(p.last_encounter_at, p.created_at), p.file_number LIMIT $3 OFFSET $4`, clinicID, days, limit, offset, rptTZ())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items, total := []rptContact{}, 0
	for rows.Next() {
		var c rptContact
		if err := rows.Scan(&c.ID, &c.FileNumber, &c.Name, &c.Subject, &c.ContactName, &c.Phone, &c.Email, &c.RemindersOK, &c.LastVisit, &c.DaysSince, &total); err != nil {
			return nil, 0, err
		}
		items = append(items, c)
	}
	return items, total, rows.Err()
}

func (s *Server) rptPatientsCSV(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	win, msg := rptParseWindow(r.URL.Query().Get)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	days, _, _, msg := rptInactiveParams(r)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	rep, err := s.rptBuildPatients(r.Context(), p.ClinicID, win, days, 1, 1)
	if err != nil {
		serverError(w, r, err)
		return
	}
	cw := rptCSVStart(w, "pacientes-"+win.FromDay+"-"+win.ToDay)
	n := strconv.Itoa
	_ = cw.Write([]string{"Reporte de pacientes", win.FromDay, win.ToDay})
	_ = cw.Write([]string{})
	_ = cw.Write([]string{"Mes", "Nuevos", "Recurrentes"})
	for _, m := range rep.Visits.ByMonth {
		_ = cw.Write([]string{m.Month, n(m.New), n(m.Returning)})
	}
	_ = cw.Write([]string{"Pacientes atendidos", n(rep.Visits.Patients)})
	_ = cw.Write([]string{"Con 2 o más consultas", n(rep.Visits.Repeat)})
	_ = cw.Write([]string{})
	_ = cw.Write([]string{"Altas por mes", "Personas", "Animales"})
	for _, m := range rep.Signups.ByMonth {
		_ = cw.Write([]string{m.Month, n(m.Person), n(m.Animal)})
	}
	_ = cw.Write([]string{})
	_ = cw.Write([]string{"Motivos de consulta más frecuentes", "Consultas"})
	for _, c := range rep.Reasons {
		_ = cw.Write([]string{csvSafe(c.Label), n(c.Count)})
	}
	_ = cw.Write([]string{})
	_ = cw.Write([]string{"Edad (personas)", "Mujeres", "Hombres", "Otro", "Sin dato", "Total"})
	for _, a := range rep.People.ByAge {
		_ = cw.Write([]string{a.Label, n(a.Female), n(a.Male), n(a.Other), n(a.Unknown), n(a.Total)})
	}
	_ = cw.Write([]string{})
	_ = cw.Write([]string{"Especie (animales)", "Pacientes"})
	for _, c := range rep.Animals.BySpecies {
		_ = cw.Write([]string{csvSafe(c.Label), n(c.Count)})
	}
	_ = cw.Write([]string{})
	_ = cw.Write([]string{"Pacientes sin visita en " + n(days) + " días", n(rep.Inactive.Total)})
	_ = cw.Write([]string{"Vacunas o desparasitaciones pendientes", n(rep.Vaccines.Total)})
	cw.Flush()
}

// rptInactiveCSV exports the whole retention list with contact data; each export is audited.
func (s *Server) rptInactiveCSV(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	days, _, _, msg := rptInactiveParams(r)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	items, total, err := s.rptInactive(r.Context(), p.ClinicID, days, 5000, 0)
	if err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "report_export", "Exportó la lista de pacientes sin visita ("+strconv.Itoa(total)+")", map[string]any{"inactive_days": days})
	cw := rptCSVStart(w, "pacientes-sin-visita-"+time.Now().Format("20060102"))
	_ = cw.Write([]string{"Expediente", "Paciente", "Tipo", "Última visita", "Días sin visita", "Contacto", "Teléfono", "Correo", "Acepta recordatorios"})
	for _, c := range items {
		last, since := "", ""
		if c.LastVisit != nil {
			last = *c.LastVisit
		}
		if c.DaysSince != nil {
			since = strconv.Itoa(*c.DaysSince)
		}
		subject := map[string]string{"person": "Persona", "animal": "Animal"}[c.Subject]
		ok := "No"
		if c.RemindersOK {
			ok = "Sí"
		}
		_ = cw.Write([]string{strconv.Itoa(c.FileNumber), csvSafe(c.Name), subject, last, since, csvSafe(c.ContactName), csvSafe(c.Phone), csvSafe(c.Email), ok})
	}
	cw.Flush()
}
