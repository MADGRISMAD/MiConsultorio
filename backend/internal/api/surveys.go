package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

// Satisfaction survey. A while after a consultation is finished, the patient (who agreed to e-mails) gets one message
// with a link to rate the visit from 1 to 5 and leave a comment. From the rating the clinic chose, they are invited to
// leave a review on Google Maps when the clinic has registered its business there. One survey per appointment, and no
// more than one per patient every 30 days. The link is a random token and works once.

const (
	surveyAttempts = 3
	surveyMaxAge   = 72 * time.Hour // a visit older than this is no longer asked about
	surveyGap      = 30 * 24 * time.Hour
)

func (s *Server) surveyPass(ctx context.Context) {
	if !s.mailEnabled() {
		return
	}
	if err := s.sendSurveys(ctx, time.Now()); err != nil && ctx.Err() == nil {
		log.Printf("surveys: %v", err)
	}
}

type surveyCandidate struct {
	ApptID, PatientID, ProfessionalID, Email, ClinicName, Names, Pro string
	Subject                                                          string
}

func (s *Server) sendSurveys(ctx context.Context, now time.Time) error {
	rows, err := s.db.Query(ctx, `
		SELECT a.id::text, p.id::text, coalesce(a.professional_id::text, ''), c.id::text, c.name, p.subject, p.names, coalesce(u.name, ''),
		       CASE WHEN p.subject = 'animal' THEN coalesce(nullif(o.email, ''), nullif(p.guardian_email, ''), '')
		            ELSE coalesce(nullif(p.email, ''), nullif(p.guardian_email, ''), '') END
		FROM appointments a
		JOIN clinic_profile cp ON cp.clinic_id = a.clinic_id AND cp.survey_enabled
		JOIN clinics c ON c.id = a.clinic_id AND c.billing_status IN ('active', 'trialing', 'past_due') AND c.branch_suspended_at IS NULL
		JOIN patients p ON p.id = a.patient_id AND p.archived_at IS NULL
		LEFT JOIN owners o ON o.id = p.owner_id
		LEFT JOIN users u ON u.id = a.professional_id
		WHERE a.status = 'completed' AND a.finished_at IS NOT NULL
		  AND a.finished_at <= $1::timestamptz - make_interval(hours => cp.survey_delay_hours)
		  AND a.finished_at > $1::timestamptz - make_interval(secs => $2)
		  AND (p.reminders_ok OR (p.owner_id IS NOT NULL AND EXISTS (SELECT 1 FROM patients q WHERE q.owner_id = p.owner_id AND q.reminders_ok)))
		  AND NOT EXISTS (SELECT 1 FROM satisfaction_surveys x WHERE x.appointment_id = a.id)
		  AND NOT EXISTS (SELECT 1 FROM satisfaction_surveys x WHERE x.patient_id = p.id AND x.created_at > $1::timestamptz - make_interval(secs => $3))
		ORDER BY a.finished_at LIMIT 100`, now, surveyMaxAge.Seconds(), surveyGap.Seconds())
	if err != nil {
		return err
	}
	type row struct {
		surveyCandidate
		ClinicID string
	}
	var list []row
	for rows.Next() {
		var c row
		if err := rows.Scan(&c.ApptID, &c.PatientID, &c.ProfessionalID, &c.ClinicID, &c.ClinicName, &c.Subject, &c.Names, &c.Pro, &c.Email); err != nil {
			rows.Close()
			return err
		}
		if c.Email != "" {
			list = append(list, c)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, c := range list {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		s.sendSurvey(ctx, c.ClinicID, c.surveyCandidate)
	}
	return nil
}

func (s *Server) sendSurvey(ctx context.Context, clinicID string, c surveyCandidate) {
	tok, err := newToken()
	if err != nil {
		return
	}
	// claim the appointment first: another instance (or an earlier pass) never sends it twice
	var pro any
	if c.ProfessionalID != "" {
		pro = c.ProfessionalID
	}
	tag, err := s.db.Exec(ctx, `
		INSERT INTO satisfaction_surveys (clinic_id, appointment_id, patient_id, professional_id, token, attempts, claimed_at) VALUES ($1, $2, $3, $4, $5, 1, now())
		ON CONFLICT (appointment_id) DO UPDATE SET attempts = satisfaction_surveys.attempts + 1, claimed_at = now()
		WHERE satisfaction_surveys.sent_at IS NULL AND satisfaction_surveys.attempts < $6 AND satisfaction_surveys.claimed_at < now() - interval '10 minutes'`,
		clinicID, c.ApptID, c.PatientID, pro, tok, surveyAttempts)
	if err != nil || tag.RowsAffected() == 0 {
		return
	}
	var token string
	if err := s.db.QueryRow(ctx, `SELECT token FROM satisfaction_surveys WHERE appointment_id = $1`, c.ApptID).Scan(&token); err != nil {
		return
	}
	subject, text, html := s.surveyMail(c, token)
	sctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if err := s.mailer.Send(sctx, mail.Message{To: []string{c.Email}, Subject: subject, Text: text, HTML: html}); err != nil {
		log.Printf("survey mail (%s): %v", c.ApptID, err)
		return
	}
	_, _ = s.db.Exec(ctx, `UPDATE satisfaction_surveys SET sent_at = now() WHERE appointment_id = $1`, c.ApptID)
}

func (s *Server) surveyMail(c surveyCandidate, token string) (subject, text, html string) {
	link := s.appLink("/encuesta/" + token)
	who := firstName(c.Names)
	lead := "Hola " + who + ", gracias por visitarnos en " + c.ClinicName + "."
	ask := "¿Cómo fue tu atención? Cuéntanos en menos de un minuto; tu opinión nos ayuda a mejorar."
	if c.Subject == "animal" {
		lead = "Hola, gracias por traer a " + who + " a " + c.ClinicName + "."
		ask = "¿Cómo fue la atención? Cuéntanos en menos de un minuto; tu opinión nos ayuda a mejorar."
	}
	subject = "¿Cómo te atendimos? · " + c.ClinicName
	text = lead + "\n\n" + ask + "\n" + link + "\n"
	body := `<p style="margin:0 0 10px;font-size:16px">` + esc(lead) + `</p><p style="margin:0 0 6px;font-size:16px">` + esc(ask) + `</p>` + button(link, "Calificar mi visita")
	return subject, text, layout("¿Cómo te atendimos?", body)
}

// ---- public survey page ----

// surveyPublic holds the public (no session) handlers of the survey and the clinic page, with their own rate limiters.
type surveyPublic struct {
	*Server
	reads  *rateLimiter
	badTok *rateLimiter
	writes *rateLimiter
}

func (s *Server) mountPublicSurvey(r chi.Router) {
	p := &surveyPublic{Server: s, reads: newRateLimiter(300, 10*time.Minute), badTok: newRateLimiter(15, 15*time.Minute), writes: newRateLimiter(30, 10*time.Minute)}
	r.Get("/public/clinic/{slug}", p.profile)
	r.Get("/public/clinic/{slug}/media/{id}", p.mediaPublic)
	r.Get("/public/survey/{token}", p.view)
	r.Post("/public/survey/{token}", p.answer)
}

type surveyView struct {
	ID, ClinicID, ClinicName, Pro string
	Answered                      bool
	Rating                        int
}

func (s *surveyPublic) find(w http.ResponseWriter, r *http.Request) (surveyView, clinicProfile, bool) {
	var v surveyView
	var prof clinicProfile
	tok := chi.URLParam(r, "token")
	if !s.badTok.allow("tok|" + clientIP(r)) {
		tooMany(w)
		return v, prof, false
	}
	var rating *int16
	var answered *time.Time
	err := s.db.QueryRow(r.Context(), `
		SELECT x.id::text, c.id::text, c.name, coalesce(u.name, ''), x.answered_at, x.rating
		FROM satisfaction_surveys x JOIN clinics c ON c.id = x.clinic_id LEFT JOIN users u ON u.id = x.professional_id
		WHERE x.token = $1 AND x.sent_at IS NOT NULL`, tok).Scan(&v.ID, &v.ClinicID, &v.ClinicName, &v.Pro, &answered, &rating)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			s.badTok.fail("tok|" + clientIP(r))
			writeError(w, http.StatusNotFound, "Este enlace no es válido o ya venció.")
		} else {
			serverError(w, r, err)
		}
		return v, prof, false
	}
	v.Answered = answered != nil
	if rating != nil {
		v.Rating = int(*rating)
	}
	prof, err = s.loadProfile(r.Context(), v.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return v, prof, false
	}
	return v, prof, true
}

func (s *surveyPublic) view(w http.ResponseWriter, r *http.Request) {
	if !limit(w, s.reads, "read|"+clientIP(r)) {
		return
	}
	v, prof, ok := s.find(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"clinic_name": v.ClinicName, "professional": firstName(v.Pro), "answered": v.Answered, "rating": v.Rating, "review_url": inviteURL(prof, v.Rating)})
}

// inviteURL is the Google review link, only for ratings the clinic wants to send there.
func inviteURL(prof clinicProfile, rating int) string {
	if rating >= prof.MapsMinRating && rating > 0 {
		return prof.reviewURL()
	}
	return ""
}

func (s *surveyPublic) answer(w http.ResponseWriter, r *http.Request) {
	if !limit(w, s.writes, "write|"+clientIP(r)) {
		return
	}
	var in struct {
		Rating   int    `json:"rating"`
		Comment  string `json:"comment"`
		PublicOK bool   `json:"public_ok"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Comment = strings.TrimSpace(in.Comment)
	if in.Rating < 1 || in.Rating > 5 {
		writeError(w, http.StatusBadRequest, "Elige una calificación de 1 a 5 estrellas.")
		return
	}
	if utf8.RuneCountInString(in.Comment) > 1000 {
		writeError(w, http.StatusBadRequest, "El comentario es demasiado largo.")
		return
	}
	v, prof, ok := s.find(w, r)
	if !ok {
		return
	}
	if v.Answered {
		writeError(w, http.StatusConflict, "Ya recibimos tu opinión. ¡Gracias!")
		return
	}
	tag, err := s.db.Exec(r.Context(), `
		UPDATE satisfaction_surveys SET answered_at = now(), rating = $2, comment = $3, public_ok = $4 WHERE id = $1 AND answered_at IS NULL`,
		v.ID, in.Rating, in.Comment, in.PublicOK && in.Comment != "")
	if err != nil {
		serverError(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusConflict, "Ya recibimos tu opinión. ¡Gracias!")
		return
	}
	// a low rating tells the clinic right away (the in-app notification), a good one is invited to Google
	if in.Rating <= 2 {
		s.notifyLowRating(r.Context(), v.ClinicID, in.Rating)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "review_url": inviteURL(prof, in.Rating)})
}

// ---- results for the clinic ----

func (s *Server) surveySummary(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	st, err := s.surveyStatsFor(r.Context(), p.ClinicID, nil, nil)
	if err != nil {
		serverError(w, r, err)
		return
	}
	var sent, answered int
	_ = s.db.QueryRow(r.Context(), `SELECT count(*) FILTER (WHERE sent_at IS NOT NULL), count(*) FILTER (WHERE answered_at IS NOT NULL) FROM satisfaction_surveys WHERE clinic_id = $1`, p.ClinicID).Scan(&sent, &answered)
	rows, err := s.db.Query(r.Context(), `
		SELECT x.rating, x.comment, x.public_ok, x.answered_at, coalesce(u.name, '')
		FROM satisfaction_surveys x LEFT JOIN users u ON u.id = x.professional_id
		WHERE x.clinic_id = $1 AND x.answered_at IS NOT NULL ORDER BY x.answered_at DESC LIMIT 30`, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	recent := []map[string]any{}
	for rows.Next() {
		var rating int
		var comment, pro string
		var pub bool
		var at time.Time
		if err := rows.Scan(&rating, &comment, &pub, &at, &pro); err != nil {
			serverError(w, r, err)
			return
		}
		recent = append(recent, map[string]any{"rating": rating, "comment": comment, "public": pub, "date": at, "professional": pro})
	}
	writeJSON(w, http.StatusOK, map[string]any{"stats": st, "sent": sent, "answered": answered, "recent": recent})
}

func (s *Server) notifyLowRating(ctx context.Context, clinicID string, rating int) {
	s.notify(ctx, s.db, clinicID, ntfNotice{Perm: PermAdminUsers, Kind: "survey_low", Title: "Un paciente calificó su visita con " + itoa(rating) + " de 5",
		Body: "Revisa su comentario en Ajustes › Perfil y encuesta.", Link: "/ajustes?s=perfil"})
}

func (s *Server) mountProfile(r chi.Router) {
	admin := require(PermAdminUsers)
	r.With(admin).Get("/clinic/profile", s.getProfile)
	r.With(admin).Put("/clinic/profile", s.updateClinicProfile)
	r.With(admin).Get("/clinic/surveys", s.surveySummary)
	r.With(admin).Get("/clinic/media", s.mediaOverview)
	r.With(admin).Post("/clinic/media", s.mediaUpload)
	r.With(admin).Get("/clinic/media/{id}", s.mediaOwn)
	r.With(admin).Delete("/clinic/media/{id}", s.mediaDelete)
	r.With(admin).Put("/clinic/profile/professionals/{id}", s.setProHidden)
}
