package api

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// mountArco: ARCO requests (staff side). Only the administrator handles them.
func (s *Server) mountArco(r chi.Router) {
	r.Route("/arco", func(r chi.Router) {
		r.Use(require(PermAdminUsers))
		r.Get("/", s.arcoList)
		r.Post("/", s.arcoCreateStaff)
		r.Get("/summary", s.arcoSummaryHandler)
		r.Get("/settings", s.arcoSettings)
		r.Get("/{id}", s.arcoGet)
		r.Patch("/{id}", s.arcoPatch)
		r.Post("/{id}/status", s.arcoSetStatus)
		r.Post("/{id}/notes", s.arcoNote)
		r.Post("/{id}/respond", s.arcoRespond)
		r.Post("/{id}/execute", s.arcoExecute)
		r.Get("/{id}/package", s.arcoPackage)
		r.Post("/{id}/archive-patient", s.arcoArchivePatient)
	})
}

type arcoEventView struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Actor     string    `json:"actor"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Server) arcoToday(ctx context.Context, clinicID string) time.Time {
	return arcoDay(time.Now(), clinicLocation(ctx, s.db, clinicID))
}

// arcoFind loads one request of the caller's clinic or answers 404.
func (s *Server) arcoFind(w http.ResponseWriter, r *http.Request, q rowsQuerier, suffix string) (arcoRequest, bool) {
	id := chi.URLParam(r, "id")
	p := principalFrom(r.Context())
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Solicitud no encontrada.")
		return arcoRequest{}, false
	}
	a, err := arcoScan(q.QueryRow(r.Context(), `SELECT `+arcoCols+arcoFrom+` WHERE r.clinic_id = $1 AND r.id = $2`+suffix, p.ClinicID, id))
	a.derive(s.arcoToday(r.Context(), p.ClinicID))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Solicitud no encontrada.")
		return a, false
	}
	if err != nil {
		serverError(w, r, err)
		return a, false
	}
	return a, true
}

func (s *Server) arcoLoad(ctx context.Context, clinicID, id string) (arcoRequest, error) {
	a, err := arcoScan(s.db.QueryRow(ctx, `SELECT `+arcoCols+arcoFrom+` WHERE r.clinic_id = $1 AND r.id = $2`, clinicID, id))
	if err == nil {
		a.derive(s.arcoToday(ctx, clinicID))
	}
	return a, err
}

func (s *Server) arcoList(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	ctx := r.Context()
	rows, err := s.db.Query(ctx, `SELECT `+arcoCols+arcoFrom+`WHERE r.clinic_id = $1 ORDER BY r.received_at DESC LIMIT 1000`, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	today := s.arcoToday(ctx, p.ClinicID)
	var all []arcoRequest
	for rows.Next() {
		a, err := arcoScan(rows)
		if err != nil {
			serverError(w, r, err)
			return
		}
		a.derive(today)
		all = append(all, a)
	}
	if err := rows.Err(); err != nil {
		serverError(w, r, err)
		return
	}
	// "vencida" is a derived state: the stored status keeps what the request really is.
	qs := r.URL.Query()
	status, kind, deadline, patient := qs.Get("status"), qs.Get("kind"), qs.Get("deadline"), qs.Get("patient")
	needle := strings.ToLower(strings.TrimSpace(qs.Get("q")))
	out := make([]arcoRequest, 0, len(all))
	for _, a := range all {
		switch {
		case kind != "" && a.Kind != kind:
			continue
		case patient != "" && (a.PatientID == nil || *a.PatientID != patient):
			continue
		case status == "abiertas" && !a.Open && !a.PendingExecute:
			continue
		case status == "vencida" && a.DeadlineState != "overdue":
			continue
		case status != "" && status != "abiertas" && status != "vencida" && a.Status != status:
			continue
		case deadline == "soon" && a.DeadlineState != "soon" && a.DeadlineState != "overdue":
			continue
		case needle != "" && !strings.Contains(strings.ToLower(a.Folio+" "+a.RequesterName+" "+a.RequesterEmail+" "+a.PatientName), needle):
			continue
		}
		out = append(out, a)
	}
	// Open ones first, the closest deadline on top.
	sort.SliceStable(out, func(i, j int) bool {
		ai, aj := out[i].DaysLeft, out[j].DaysLeft
		switch {
		case ai != nil && aj != nil:
			return *ai < *aj
		case ai != nil:
			return true
		case aj != nil:
			return false
		}
		return out[i].ReceivedAt.After(out[j].ReceivedAt)
	})
	writeJSON(w, http.StatusOK, map[string]any{"requests": out, "summary": arcoSummarize(all)})
}

func (s *Server) arcoSummaryHandler(w http.ResponseWriter, r *http.Request) {
	pending, err := arcoLoadPending(r.Context(), s.db, principalFrom(r.Context()).ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"summary": arcoSummarize(pending)})
}

// arcoSettings gives the public link of the clinic's ARCO form.
func (s *Server) arcoSettings(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	var code string
	if err := s.db.QueryRow(r.Context(), `SELECT arco_code FROM clinics WHERE id = $1`, p.ClinicID).Scan(&code); err != nil {
		serverError(w, r, err)
		return
	}
	l, err := loadLegal(r.Context(), s.db, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"slug": code, "path": "/" + code + "/arco", "url": s.appLink("/" + code + "/arco"),
		"privacy_contact_set": l.PrivacyContact != "" && l.PrivacyEmail != "",
		"mail_enabled":        s.mailEnabled(),
	})
}

func (s *Server) arcoGet(w http.ResponseWriter, r *http.Request) {
	a, ok := s.arcoFind(w, r, s.db, "")
	if !ok {
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT id, kind, actor_name, message, created_at FROM arco_events
		WHERE clinic_id = $1 AND request_id = $2 ORDER BY created_at, id`, principalFrom(r.Context()).ClinicID, a.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	events := []arcoEventView{}
	for rows.Next() {
		var e arcoEventView
		if err := rows.Scan(&e.ID, &e.Kind, &e.Actor, &e.Message, &e.CreatedAt); err != nil {
			serverError(w, r, err)
			return
		}
		events = append(events, e)
	}
	writeJSON(w, http.StatusOK, map[string]any{"request": a, "events": events})
}

// arcoPatientOK checks that the patient belongs to the clinic.
func arcoPatientOK(ctx context.Context, q rowsQuerier, clinicID, patientID string) bool {
	if !validUUID(patientID) {
		return false
	}
	var ok bool
	_ = q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM patients WHERE clinic_id = $1 AND id = $2)`, clinicID, patientID).Scan(&ok)
	return ok
}

// arcoReceivedDate reads an optional "YYYY-MM-DD" reception date (the day the request really arrived).
func arcoReceivedDate(raw string, loc *time.Location) (time.Time, string) {
	if raw == "" {
		return time.Time{}, ""
	}
	d, err := time.ParseInLocation("2006-01-02", raw, loc)
	now := time.Now()
	if err != nil || d.After(now) || d.Before(now.AddDate(-2, 0, 0)) {
		return time.Time{}, "La fecha de recepción no es válida (no puede ser futura ni de hace más de 2 años)."
	}
	// Midday keeps the date stable in any display zone.
	return time.Date(d.Year(), d.Month(), d.Day(), 12, 0, 0, 0, loc), ""
}

func (s *Server) arcoCreateStaff(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind           string `json:"kind"`
		RequesterName  string `json:"requester_name"`
		RequesterEmail string `json:"requester_email"`
		RequesterPhone string `json:"requester_phone"`
		Description    string `json:"description"`
		PatientID      string `json:"patient_id"`
		ReceivedOn     string `json:"received_on"`
	}
	if !decode(w, r, &req) {
		return
	}
	p := principalFrom(r.Context())
	ctx := r.Context()
	loc := clinicLocation(ctx, s.db, p.ClinicID)
	name, okName := arcoCleanText(arcoOneLine(req.RequesterName), 3, 120)
	desc, okDesc := arcoCleanText(req.Description, 5, 3000)
	email := strings.TrimSpace(req.RequesterEmail)
	phone := arcoOneLine(req.RequesterPhone)
	received, msg := arcoReceivedDate(req.ReceivedOn, loc)
	switch {
	case !arcoValidKind(req.Kind):
		msg = "Elige el tipo de solicitud."
	case !okName:
		msg = "Escribe el nombre completo de quien solicita."
	case !okDesc:
		msg = "Describe la solicitud (entre 5 y 3000 caracteres)."
	case email != "" && !validEmail(email):
		msg = "El correo no es válido."
	case len(phone) > 30:
		msg = "El teléfono es demasiado largo."
	case email == "" && phone == "":
		msg = "Registra un correo o un teléfono para comunicar la respuesta."
	case req.PatientID != "" && !arcoPatientOK(ctx, s.db, p.ClinicID, req.PatientID):
		msg = "Paciente no encontrado."
	}
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	var id, folio string
	err := inTx(ctx, s.db, func(tx pgx.Tx) error {
		var err error
		id, folio, _, err = arcoCreate(ctx, tx, p.ClinicID, arcoNew{Kind: req.Kind, Name: name, Email: email, Phone: phone, Description: desc,
			Via: "staff", HandledBy: p.actorName(), PatientID: req.PatientID, Received: received, Actor: p.actorName()}, loc)
		if err != nil {
			return err
		}
		audit(ctx, tx, p.ClinicID, p, "arco_created", "Registró la solicitud ARCO "+folio+" ("+arcoKindLabels[req.Kind]+")", map[string]any{"arco_id": id, "folio": folio})
		return nil
	})
	if err != nil {
		serverError(w, r, err)
		return
	}
	a, err := s.arcoLoad(ctx, p.ClinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"request": a})
}
