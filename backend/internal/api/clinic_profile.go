package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/madgrismad/miconsultorio/backend/internal/db"
)

// The clinic's public page (/<slug>, the same slug as online booking) and the settings of the satisfaction survey.
// The page shows what the clinic chose to publish, the professionals offered online and, if wanted, the rating that
// patients gave in the survey with the comments they allowed to show. The link to leave a review on Google Maps only
// exists when the clinic gave its Place ID.

var placeIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{10,200}$`)

type clinicProfile struct {
	Enabled          bool   `json:"enabled"`
	Tagline          string `json:"tagline"`
	About            string `json:"about"`
	HoursText        string `json:"hours_text"`
	WhatsApp         string `json:"whatsapp"`
	ContactEmail     string `json:"contact_email"`
	Website          string `json:"website"`
	MapsURL          string `json:"maps_url"`
	GooglePlaceID    string `json:"google_place_id"`
	ShowReviews      bool   `json:"show_reviews"`
	SurveyEnabled    bool   `json:"survey_enabled"`
	SurveyDelayHours int    `json:"survey_delay_hours"`
	MapsMinRating    int    `json:"maps_min_rating"`
	// Directorio
	// DirectoryHidden: por defecto todos salen en el directorio; quien no quiera, lo oculta.
	DirectoryHidden bool     `json:"directory_hidden"`
	City            string   `json:"city"`
	State           string   `json:"state"`
	Neighborhood    string   `json:"neighborhood"`
	Insurances      []string `json:"insurances"`
	Languages       []string `json:"languages"`
	PaymentMethods  []string `json:"payment_methods"`
	// AcceptsInsurance: la lista de aseguradoras solo cuenta (y se muestra) cuando acepta seguros.
	AcceptsInsurance bool `json:"accepts_insurance"`
	// Highlights: por qué elegir el consultorio (lista corta, hasta 6).
	Highlights []string  `json:"highlights"`
	FAQs       []faqItem `json:"faqs"`
	Facebook   string    `json:"facebook_url"`
	Instagram  string    `json:"instagram_url"`
	// ServiceDetails: descripción y «destacado» de los servicios (nil = no cambiar).
	ServiceDetails []serviceDetail `json:"service_details,omitempty"`
	// PublicServices son los servicios del catálogo que se muestran con su precio (nil = no cambiar).
	PublicServices []string `json:"public_services,omitempty"`
}

type faqItem struct {
	Q string `json:"q"`
	A string `json:"a"`
}

type serviceDetail struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Featured    bool   `json:"featured"`
}

// socialURL turns what the clinic typed (an @user, a user or a full link) into the address of its profile on the network,
// and refuses links that go anywhere else.
func socialURL(network, raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", true
	}
	hosts := map[string][]string{"instagram": {"instagram.com"}, "facebook": {"facebook.com", "fb.com"}}[network]
	if strings.HasPrefix(strings.ToLower(raw), "http") {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || len(raw) > 200 || strings.ContainsAny(raw, " \t\r\n<>\"") {
			return "", false
		}
		h := strings.ToLower(u.Hostname())
		for _, ok := range hosts {
			if h == ok || strings.HasSuffix(h, "."+ok) {
				return raw, true
			}
		}
		return "", false
	}
	handle := strings.TrimPrefix(raw, "@")
	if !regexp.MustCompile(`^[A-Za-z0-9._-]{1,60}$`).MatchString(handle) {
		return "", false
	}
	return "https://www." + hosts[0] + "/" + handle, true
}

func (c clinicProfile) reviewURL() string {
	if c.GooglePlaceID == "" {
		return ""
	}
	return "https://search.google.com/local/writereview?placeid=" + url.QueryEscape(c.GooglePlaceID)
}

func defaultProfile() clinicProfile {
	return clinicProfile{ShowReviews: true, SurveyDelayHours: 3, MapsMinRating: 4, Insurances: []string{}, Languages: []string{"Español"}, PaymentMethods: []string{}, Highlights: []string{}, FAQs: []faqItem{}}
}

const profileCols = `enabled, tagline, about, hours_text, whatsapp, contact_email, website, maps_url, google_place_id, show_reviews, survey_enabled, survey_delay_hours, maps_min_rating,
	directory_hidden, city, state, neighborhood, insurances, languages, payment_methods,
	accepts_insurance, highlights, faqs, facebook_url, instagram_url`

func (s *Server) loadProfile(ctx context.Context, clinicID string) (clinicProfile, error) {
	c := defaultProfile()
	var mr int16
	var delay int32
	var faqRaw []byte
	err := s.db.QueryRow(ctx, `SELECT `+profileCols+` FROM clinic_profile WHERE clinic_id = $1`, clinicID).
		Scan(&c.Enabled, &c.Tagline, &c.About, &c.HoursText, &c.WhatsApp, &c.ContactEmail, &c.Website, &c.MapsURL, &c.GooglePlaceID, &c.ShowReviews, &c.SurveyEnabled, &delay, &mr,
			&c.DirectoryHidden, &c.City, &c.State, &c.Neighborhood, &c.Insurances, &c.Languages, &c.PaymentMethods,
			&c.AcceptsInsurance, &c.Highlights, &faqRaw, &c.Facebook, &c.Instagram)
	if errors.Is(err, pgx.ErrNoRows) {
		return defaultProfile(), nil
	}
	c.SurveyDelayHours, c.MapsMinRating = int(delay), int(mr)
	if len(faqRaw) > 0 {
		_ = json.Unmarshal(faqRaw, &c.FAQs)
	}
	if c.FAQs == nil {
		c.FAQs = []faqItem{}
	}
	if c.Highlights == nil {
		c.Highlights = []string{}
	}
	return c, err
}

func (s *Server) bookingSlugOf(ctx context.Context, clinicID string) string {
	var slug string
	_ = s.db.QueryRow(ctx, `SELECT coalesce(booking_slug, '') FROM agenda_settings WHERE clinic_id = $1`, clinicID).Scan(&slug)
	return slug
}

func (s *Server) getProfile(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	c, err := s.loadProfile(r.Context(), p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	slug := s.bookingSlugOf(r.Context(), p.ClinicID)
	services := []map[string]any{}
	rows, err := s.db.Query(r.Context(), `SELECT id::text, name, price_cents, public, package_sessions, public_description, featured FROM catalog_items WHERE clinic_id = $1 AND kind = 'service' AND active ORDER BY package_sessions IS NOT NULL, name LIMIT 200`, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	for rows.Next() {
		var id, name string
		var price int
		var pub bool
		var sessions *int
		var desc string
		var feat bool
		if err := rows.Scan(&id, &name, &price, &pub, &sessions, &desc, &feat); err != nil {
			rows.Close()
			serverError(w, r, err)
			return
		}
		services = append(services, map[string]any{"id": id, "name": name, "price_cents": price, "public": pub, "package_sessions": sessions, "description": desc, "featured": feat})
	}
	rows.Close()
	score, checklist, err := s.profileChecklist(r.Context(), p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profile": c, "slug": slug, "public_url": publicProfilePath(slug), "review_url": c.reviewURL(),
		"services": services, "states": mxStates, "payment_methods": paymentMethods, "completeness": map[string]any{"score": score, "items": checklist}})
}

func publicProfilePath(slug string) string {
	if slug == "" {
		return ""
	}
	return "/" + slug
}

func httpsURL(raw string, max int) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", true
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || len(raw) > max || strings.ContainsAny(raw, " \t\r\n<>\"") {
		return "", false
	}
	return raw, true
}

func (s *Server) updateClinicProfile(w http.ResponseWriter, r *http.Request) {
	var in clinicProfile
	if !decode(w, r, &in) {
		return
	}
	p := principalFrom(r.Context())
	for _, f := range []struct {
		v   *string
		max int
	}{{&in.Tagline, 120}, {&in.About, 1500}, {&in.HoursText, 400}, {&in.WhatsApp, 20}, {&in.ContactEmail, 120}, {&in.GooglePlaceID, 200}} {
		*f.v = strings.TrimSpace(*f.v)
		if utf8.RuneCountInString(*f.v) > f.max {
			writeError(w, http.StatusBadRequest, "Uno de los textos es demasiado largo.")
			return
		}
	}
	var ok bool
	if in.Website, ok = httpsURL(in.Website, 200); !ok {
		writeError(w, http.StatusBadRequest, "El sitio web debe ser un enlace que empiece con https://.")
		return
	}
	if in.MapsURL, ok = httpsURL(in.MapsURL, 500); !ok {
		writeError(w, http.StatusBadRequest, "El enlace de Google Maps debe empezar con https://.")
		return
	}
	if in.ContactEmail != "" && !db.ValidEmail(in.ContactEmail) {
		writeError(w, http.StatusBadRequest, "El correo de contacto no es válido.")
		return
	}
	if in.GooglePlaceID != "" && !placeIDRe.MatchString(in.GooglePlaceID) {
		writeError(w, http.StatusBadRequest, "El Place ID de Google no es válido.")
		return
	}
	if in.SurveyDelayHours < 1 || in.SurveyDelayHours > 72 || in.MapsMinRating < 1 || in.MapsMinRating > 5 {
		writeError(w, http.StatusBadRequest, "Revisa el tiempo de envío y la calificación mínima.")
		return
	}
	in.City, in.Neighborhood = strings.TrimSpace(in.City), strings.TrimSpace(in.Neighborhood)
	if utf8.RuneCountInString(in.City) > 80 || utf8.RuneCountInString(in.Neighborhood) > 80 {
		writeError(w, http.StatusBadRequest, "La ciudad o la colonia son demasiado largas.")
		return
	}
	if in.State != "" && !slices.Contains(mxStates, in.State) {
		writeError(w, http.StatusBadRequest, "Elige un estado de la lista.")
		return
	}
	var msg string
	if in.Insurances, msg = profileList(in.Insurances, 20, 60, nil); msg != "" {
		writeError(w, http.StatusBadRequest, "Aseguradoras: "+msg)
		return
	}
	if in.Languages, msg = profileList(in.Languages, 10, 30, nil); msg != "" {
		writeError(w, http.StatusBadRequest, "Idiomas: "+msg)
		return
	}
	if in.PaymentMethods, msg = profileList(in.PaymentMethods, len(paymentMethods), 30, paymentMethods); msg != "" {
		writeError(w, http.StatusBadRequest, "Formas de pago: "+msg)
		return
	}
	if !in.AcceptsInsurance {
		in.Insurances = []string{} // sin seguros no hay lista (tampoco en el directorio)
	}
	if in.Highlights, msg = profileList(in.Highlights, 6, 100, nil); msg != "" {
		writeError(w, http.StatusBadRequest, "Motivos para elegirte: "+msg)
		return
	}
	faqs := []faqItem{}
	for _, f := range in.FAQs {
		f.Q, f.A = strings.TrimSpace(f.Q), strings.TrimSpace(f.A)
		if f.Q == "" && f.A == "" {
			continue
		}
		if f.Q == "" || f.A == "" || utf8.RuneCountInString(f.Q) > 150 || utf8.RuneCountInString(f.A) > 800 {
			writeError(w, http.StatusBadRequest, "Cada pregunta frecuente necesita pregunta y respuesta (pregunta hasta 150 caracteres, respuesta hasta 800).")
			return
		}
		faqs = append(faqs, f)
	}
	if len(faqs) > 12 {
		writeError(w, http.StatusBadRequest, "Máximo 12 preguntas frecuentes.")
		return
	}
	in.FAQs = faqs
	if in.Facebook, ok = socialURL("facebook", in.Facebook); !ok {
		writeError(w, http.StatusBadRequest, "El Facebook debe ser tu usuario o un enlace de facebook.com.")
		return
	}
	if in.Instagram, ok = socialURL("instagram", in.Instagram); !ok {
		writeError(w, http.StatusBadRequest, "El Instagram debe ser tu usuario o un enlace de instagram.com.")
		return
	}
	featured := 0
	for i := range in.ServiceDetails {
		d := &in.ServiceDetails[i]
		d.Description = strings.TrimSpace(d.Description)
		if !validUUID(d.ID) || utf8.RuneCountInString(d.Description) > 200 {
			writeError(w, http.StatusBadRequest, "La descripción de un servicio puede tener hasta 200 caracteres.")
			return
		}
		if d.Featured {
			featured++
		}
	}
	if featured > 4 {
		writeError(w, http.StatusBadRequest, "Puedes destacar hasta 4 servicios.")
		return
	}
	if (in.Enabled) && s.bookingSlugOf(r.Context(), p.ClinicID) == "" {
		writeError(w, http.StatusBadRequest, "Primero define el enlace de tu página en Ajustes › Agenda (enlace de citas en línea).")
		return
	}
	faqJSON, _ := json.Marshal(in.FAQs)
	_, err := s.db.Exec(r.Context(), `
		INSERT INTO clinic_profile (clinic_id, enabled, tagline, about, hours_text, whatsapp, contact_email, website, maps_url, google_place_id, show_reviews, survey_enabled, survey_delay_hours, maps_min_rating,
			directory_hidden, city, state, neighborhood, insurances, languages, payment_methods,
			accepts_insurance, highlights, faqs, facebook_url, instagram_url, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $14, $7, $8, $9, $10, $11, $12, $13, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24::jsonb, $25, $26, now())
		ON CONFLICT (clinic_id) DO UPDATE SET enabled = $2, tagline = $3, about = $4, hours_text = $5, whatsapp = $6, contact_email = $14, website = $7, maps_url = $8,
			google_place_id = $9, show_reviews = $10, survey_enabled = $11, survey_delay_hours = $12, maps_min_rating = $13,
			directory_hidden = $15, city = $16, state = $17, neighborhood = $18, insurances = $19, languages = $20, payment_methods = $21,
			accepts_insurance = $22, highlights = $23, faqs = $24::jsonb, facebook_url = $25, instagram_url = $26, updated_at = now()`,
		p.ClinicID, in.Enabled, in.Tagline, in.About, in.HoursText, in.WhatsApp, in.Website, in.MapsURL, in.GooglePlaceID, in.ShowReviews, in.SurveyEnabled, in.SurveyDelayHours, in.MapsMinRating, in.ContactEmail,
		in.DirectoryHidden, in.City, in.State, in.Neighborhood, in.Insurances, in.Languages, in.PaymentMethods,
		in.AcceptsInsurance, in.Highlights, string(faqJSON), in.Facebook, in.Instagram)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if in.PublicServices != nil {
		ids := make([]string, 0, len(in.PublicServices))
		for _, id := range in.PublicServices {
			if validUUID(id) {
				ids = append(ids, id)
			}
		}
		if _, err := s.db.Exec(r.Context(), `UPDATE catalog_items SET public = (id::text = ANY($2)) WHERE clinic_id = $1 AND kind = 'service'`, p.ClinicID, ids); err != nil {
			serverError(w, r, err)
			return
		}
	}
	for _, d := range in.ServiceDetails {
		if _, err := s.db.Exec(r.Context(), `UPDATE catalog_items SET public_description = $3, featured = $4 WHERE clinic_id = $1 AND id = $2 AND kind = 'service'`, p.ClinicID, d.ID, d.Description, d.Featured); err != nil {
			serverError(w, r, err)
			return
		}
	}
	audit(r.Context(), s.db, p.ClinicID, p, "clinic_profile", "Actualizó el perfil público y la encuesta de satisfacción", nil)
	s.getProfile(w, r)
}

// ---- satisfaction numbers ----

type surveyStats struct {
	Count   int     `json:"count"`
	Average float64 `json:"average"`
	Dist    [5]int  `json:"distribution"` // index 0 = one star
}

func (s *Server) surveyStatsFor(ctx context.Context, clinicID string, since *time.Time, until *time.Time) (surveyStats, error) {
	var st surveyStats
	rows, err := s.db.Query(ctx, `
		SELECT rating, count(*) FROM satisfaction_surveys
		WHERE clinic_id = $1 AND answered_at IS NOT NULL AND ($2::timestamptz IS NULL OR answered_at >= $2) AND ($3::timestamptz IS NULL OR answered_at < $3)
		GROUP BY rating`, clinicID, since, until)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	sum := 0
	for rows.Next() {
		var rating, n int
		if err := rows.Scan(&rating, &n); err != nil {
			return st, err
		}
		if rating >= 1 && rating <= 5 {
			st.Dist[rating-1] = n
			st.Count += n
			sum += rating * n
		}
	}
	if st.Count > 0 {
		st.Average = float64(int(float64(sum)/float64(st.Count)*10+0.5)) / 10
	}
	return st, rows.Err()
}

// ---- public page ----

func (s *surveyPublic) profile(w http.ResponseWriter, r *http.Request) {
	if !limit(w, s.reads, "profile|"+clientIP(r)) {
		return
	}
	ctx := r.Context()
	slug := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "slug")))
	notFound := func() { writeError(w, http.StatusNotFound, "Esta página no está disponible.") }
	if !slugShape.MatchString(slug) {
		notFound()
		return
	}
	var id, name, kind, address, phone string
	var bookingOn bool
	err := s.db.QueryRow(ctx, `
		SELECT c.id::text, c.name, c.kind, c.address, c.phone_number, a.booking_enabled
		FROM agenda_settings a JOIN clinics c ON c.id = a.clinic_id
		JOIN clinic_profile cp ON cp.clinic_id = c.id AND cp.enabled
		WHERE lower(a.booking_slug) = $1 AND c.billing_status IN ('active', 'trialing', 'past_due') AND c.branch_suspended_at IS NULL`, slug).
		Scan(&id, &name, &kind, &address, &phone, &bookingOn)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound()
		} else {
			serverError(w, r, err)
		}
		return
	}
	prof, err := s.loadProfile(ctx, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	kinds, err := s.clinicKindsFor(ctx, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	areas := []string{}
	for _, k := range kinds {
		if l := areaLabels[k]; l != "" {
			areas = append(areas, l)
		}
	}
	// the team shown: people who work there now, see patients, are not hidden and work in a giro the clinic still has
	pros := []map[string]any{}
	rows, err := s.db.Query(ctx, `
		SELECT u.name, u.specialty_title, u.areas, (SELECT m.id::text FROM clinic_media m WHERE m.user_id = u.id AND m.slot = 'pro'),
		       u.cedula, u.cedula_specialty, u.public_bio
		FROM users u LEFT JOIN professional_settings ps ON ps.user_id = u.id
		WHERE u.clinic_id = $1 AND u.role IN ('admin', 'doctor') AND u.linked_owner_id IS NULL AND NOT u.disabled AND NOT u.public_hidden
		  AND coalesce(ps.consults, true) AND (cardinality(u.areas) = 0 OR u.areas && $2::text[])
		ORDER BY u.name LIMIT 30`, id, kinds)
	if err != nil {
		serverError(w, r, err)
		return
	}
	for rows.Next() {
		var n, t, ced, cedSpec, bio string
		var photo *string
		var areas []string
		if err := rows.Scan(&n, &t, &areas, &photo, &ced, &cedSpec, &bio); err != nil {
			rows.Close()
			serverError(w, r, err)
			return
		}
		pr := map[string]any{"name": n, "title": publicTitle(t, areas, kinds), "photo_url": "", "cedula": ced, "cedula_specialty": cedSpec, "bio": bio}
		if photo != nil {
			pr["photo_url"] = mediaURL(slug, *photo)
		}
		pros = append(pros, pr)
	}
	rows.Close()
	profileID, coverID, gallery, err := s.publicMedia(ctx, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	galleryURLs := make([]string, 0, len(gallery))
	for _, g := range gallery {
		galleryURLs = append(galleryURLs, mediaURL(slug, g))
	}
	profileURL, coverURL := "", ""
	if profileID != nil {
		profileURL = mediaURL(slug, *profileID)
	}
	if coverID != nil {
		coverURL = mediaURL(slug, *coverID)
	}
	out := map[string]any{
		"name": name, "address": address, "phone": phone, "areas": areas, "professionals": pros,
		"tagline": prof.Tagline, "about": prof.About, "hours_text": prof.HoursText, "whatsapp": prof.WhatsApp, "website": prof.Website,
		"maps_url": prof.MapsURL, "review_url": prof.reviewURL(), "booking_url": "", "slug": slug,
		"email": prof.ContactEmail, "kinds": kinds, "profile_url": profileURL, "cover_url": coverURL, "gallery": galleryURLs,
		"city": prof.City, "state": prof.State, "neighborhood": prof.Neighborhood, "listed": !prof.DirectoryHidden,
		"insurances": prof.Insurances, "languages": prof.Languages, "payment_methods": prof.PaymentMethods,
		"accepts_insurance": prof.AcceptsInsurance, "highlights": prof.Highlights, "faqs": prof.FAQs, "facebook_url": prof.Facebook, "instagram_url": prof.Instagram,
	}
	// servicios publicados con su precio
	svcs := []map[string]any{}
	sr, err := s.db.Query(ctx, `SELECT id::text, name, price_cents, duration_minutes, modality, package_sessions, public_description, featured FROM catalog_items WHERE clinic_id = $1 AND kind = 'service' AND public AND active ORDER BY package_sessions IS NOT NULL, featured DESC, price_cents, name LIMIT 40`, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	for sr.Next() {
		var sid, n, modality string
		var price int
		var dur, sessions *int
		var desc string
		var feat bool
		if err := sr.Scan(&sid, &n, &price, &dur, &modality, &sessions, &desc, &feat); err != nil {
			sr.Close()
			serverError(w, r, err)
			return
		}
		svcs = append(svcs, map[string]any{"id": sid, "name": n, "price_cents": price, "duration_minutes": dur, "modality": modality, "package_sessions": sessions, "description": desc, "featured": feat})
	}
	sr.Close()
	out["services"] = svcs
	if bookingOn {
		out["booking_url"] = "/" + slug + "/reservar"
	}
	if prof.ShowReviews {
		st, err := s.surveyStatsFor(ctx, id, nil, nil)
		if err != nil {
			serverError(w, r, err)
			return
		}
		reviews := []map[string]any{}
		if st.Count > 0 {
			rr, err := s.db.Query(ctx, `
				SELECT rating, comment, answered_at, reply FROM satisfaction_surveys
				WHERE clinic_id = $1 AND answered_at IS NOT NULL AND public_ok AND comment <> '' ORDER BY answered_at DESC LIMIT 20`, id)
			if err != nil {
				serverError(w, r, err)
				return
			}
			for rr.Next() {
				var rating int
				var c, reply string
				var at time.Time
				if err := rr.Scan(&rating, &c, &at, &reply); err != nil {
					rr.Close()
					serverError(w, r, err)
					return
				}
				reviews = append(reviews, map[string]any{"rating": rating, "comment": c, "date": at.Format("2006-01-02"), "reply": reply, "verified": true})
			}
			rr.Close()
		}
		out["rating"] = st
		out["reviews"] = reviews
	}
	writeJSON(w, http.StatusOK, out)
}

func mediaURL(slug, id string) string { return "/api/public/clinic/" + slug + "/media/" + id }

// profileList recorta, quita repetidos y vacíos, y valida una lista de textos del perfil. allowed (si no es nil) limita los valores.
func profileList(in []string, maxItems, maxLen int, allowed []string) ([]string, string) {
	out := []string{}
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || slices.Contains(out, v) {
			continue
		}
		if utf8.RuneCountInString(v) > maxLen {
			return nil, "uno de los valores es demasiado largo."
		}
		if allowed != nil && !slices.Contains(allowed, v) {
			return nil, "hay un valor que no es de la lista."
		}
		out = append(out, v)
	}
	if len(out) > maxItems {
		return nil, "son demasiados."
	}
	return out, ""
}
