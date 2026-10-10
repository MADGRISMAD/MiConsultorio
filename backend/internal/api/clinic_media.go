package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Photos of the clinic's public page. Images are stored in the database (a handful per clinic, each resized by the
// browser before it is sent) and served by id. The administrator sees them while editing, even before publishing;
// the public sees them only when the page is on.

const (
	mediaMaxBytes   = 1400 << 10 // after decoding
	mediaMaxGallery = 5
)

var mediaTypes = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}

type mediaItem struct {
	ID string `json:"id"`
}

type mediaPro struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Title  string  `json:"title"`
	Photo  *string `json:"photo"` // media id
	Hidden bool    `json:"hidden"`
	Active bool    `json:"active"` // shows on the page right now (not disabled, sees patients, works in a giro of the clinic)
	Bio    string  `json:"bio"`
}

// mediaOverview is what the editor shows.
func (s *Server) mediaOverview(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	ctx := r.Context()
	out := map[string]any{"profile": nil, "cover": nil, "gallery": []string{}, "max_gallery": mediaMaxGallery}
	rows, err := s.db.Query(ctx, `SELECT id::text, slot FROM clinic_media WHERE clinic_id = $1 AND slot <> 'pro' ORDER BY created_at`, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	gallery := []string{}
	for rows.Next() {
		var id, slot string
		if err := rows.Scan(&id, &slot); err != nil {
			rows.Close()
			serverError(w, r, err)
			return
		}
		switch slot {
		case "profile", "cover":
			out[slot] = id
		default:
			gallery = append(gallery, id)
		}
	}
	rows.Close()
	out["gallery"] = gallery
	kinds, err := s.clinicKindsFor(ctx, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	prows, err := s.db.Query(ctx, `
		SELECT u.id::text, u.name, u.specialty_title, (SELECT m.id::text FROM clinic_media m WHERE m.user_id = u.id AND m.slot = 'pro'), u.public_hidden, u.public_bio,
		       (NOT u.disabled AND coalesce(ps.consults, true) AND (cardinality(u.areas) = 0 OR u.areas && $2::text[]))
		FROM users u LEFT JOIN professional_settings ps ON ps.user_id = u.id
		WHERE u.clinic_id = $1 AND u.role IN ('admin', 'doctor') AND u.linked_owner_id IS NULL AND NOT u.disabled
		ORDER BY u.name`, p.ClinicID, kinds)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer prows.Close()
	pros := []mediaPro{}
	for prows.Next() {
		var x mediaPro
		if err := prows.Scan(&x.ID, &x.Name, &x.Title, &x.Photo, &x.Hidden, &x.Bio, &x.Active); err != nil {
			serverError(w, r, err)
			return
		}
		pros = append(pros, x)
	}
	out["professionals"] = pros
	writeJSON(w, http.StatusOK, out)
}

type mediaIn struct {
	Slot   string `json:"slot"`
	UserID string `json:"user_id"`
	Image  string `json:"image"` // data URL
}

func (s *Server) mediaUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	var in mediaIn
	if !decodeLoose(w, r, &in) {
		return
	}
	p := principalFrom(r.Context())
	ctx := r.Context()
	head, b64, ok := strings.Cut(in.Image, ",")
	if !ok || !strings.HasPrefix(head, "data:") || !strings.HasSuffix(head, ";base64") {
		writeError(w, http.StatusBadRequest, "La imagen no es válida.")
		return
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(data) == 0 {
		writeError(w, http.StatusBadRequest, "La imagen no es válida.")
		return
	}
	if len(data) > mediaMaxBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "La imagen pesa demasiado. Elige una más pequeña.")
		return
	}
	mime := http.DetectContentType(data) // the real type, not what the data URL claims
	if !mediaTypes[mime] {
		writeError(w, http.StatusBadRequest, "Sube una imagen JPG, PNG o WebP.")
		return
	}
	var user any
	switch in.Slot {
	case "profile", "cover", "gallery":
		if in.UserID != "" {
			writeError(w, http.StatusBadRequest, "La imagen no es válida.")
			return
		}
	case "pro":
		var one int
		if !validUUID(in.UserID) || s.db.QueryRow(ctx, `SELECT 1 FROM users WHERE id = $1 AND clinic_id = $2 AND role IN ('admin', 'doctor')`, in.UserID, p.ClinicID).Scan(&one) != nil {
			writeError(w, http.StatusBadRequest, "Elige a la persona del equipo.")
			return
		}
		user = in.UserID
	default:
		writeError(w, http.StatusBadRequest, "La imagen no es válida.")
		return
	}
	var id string
	err = inTx(ctx, s.db, func(tx pgx.Tx) error {
		if err := lockClinic(ctx, tx, p.ClinicID); err != nil {
			return err
		}
		switch in.Slot {
		case "gallery":
			var n int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM clinic_media WHERE clinic_id = $1 AND slot = 'gallery'`, p.ClinicID).Scan(&n); err != nil {
				return err
			}
			if n >= mediaMaxGallery {
				return fail(http.StatusConflict, "La galería permite hasta 5 fotos. Elimina una para agregar otra.")
			}
		case "pro":
			if _, err := tx.Exec(ctx, `DELETE FROM clinic_media WHERE clinic_id = $1 AND slot = 'pro' AND user_id = $2`, p.ClinicID, in.UserID); err != nil {
				return err
			}
		default:
			if _, err := tx.Exec(ctx, `DELETE FROM clinic_media WHERE clinic_id = $1 AND slot = $2`, p.ClinicID, in.Slot); err != nil {
				return err
			}
		}
		return tx.QueryRow(ctx, `INSERT INTO clinic_media (clinic_id, slot, user_id, mime, data) VALUES ($1, $2, $3, $4, $5) RETURNING id::text`,
			p.ClinicID, in.Slot, user, mime, data).Scan(&id)
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	audit(ctx, s.db, p.ClinicID, p, "clinic_media", "Cambió una foto de la página pública ("+in.Slot+")", nil)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (s *Server) mediaDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Foto no encontrada.")
		return
	}
	p := principalFrom(r.Context())
	tag, err := s.db.Exec(r.Context(), `DELETE FROM clinic_media WHERE id = $1 AND clinic_id = $2`, id, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "Foto no encontrada.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// mediaOwn serves a photo to the clinic's own team (the editor's preview).
func (s *Server) mediaOwn(w http.ResponseWriter, r *http.Request) {
	s.serveMedia(w, r, principalFrom(r.Context()).ClinicID, chi.URLParam(r, "id"), false)
}

// mediaPublic serves a photo of a clinic whose page is on.
func (s *surveyPublic) mediaPublic(w http.ResponseWriter, r *http.Request) {
	if !limit(w, s.reads, "media|"+clientIP(r)) {
		return
	}
	slug := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "slug")))
	var clinicID string
	err := s.db.QueryRow(r.Context(), `
		SELECT c.id::text FROM agenda_settings a JOIN clinics c ON c.id = a.clinic_id JOIN clinic_profile cp ON cp.clinic_id = c.id AND cp.enabled
		WHERE lower(a.booking_slug) = $1 AND c.billing_status IN ('active', 'trialing', 'past_due') AND c.branch_suspended_at IS NULL`, slug).Scan(&clinicID)
	if err != nil {
		writeError(w, http.StatusNotFound, "No encontramos la foto.")
		return
	}
	s.serveMedia(w, r, clinicID, chi.URLParam(r, "id"), true)
}

func (s *Server) serveMedia(w http.ResponseWriter, r *http.Request, clinicID, id string, public bool) {
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "No encontramos la foto.")
		return
	}
	var mime string
	var data []byte
	// a specialist who is hidden or no longer works there does not keep a public photo
	q := `SELECT m.mime, m.data FROM clinic_media m LEFT JOIN users u ON u.id = m.user_id WHERE m.id = $1 AND m.clinic_id = $2`
	if public {
		q += ` AND (m.user_id IS NULL OR (NOT u.disabled AND NOT u.public_hidden))`
	}
	if err := s.db.QueryRow(r.Context(), q, id, clinicID).Scan(&mime, &data); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "No encontramos la foto.")
		} else {
			serverError(w, r, err)
		}
		return
	}
	etag := `"` + id + `"` // a photo is replaced by a new row, so its id never points to other bytes
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", map[bool]string{true: "public, max-age=300", false: "private, max-age=300"}[public])
	_, _ = w.Write(data)
}

// setProHidden is PUT /clinic/profile/professionals/{id}: show or hide a specialist on the public page.
func (s *Server) setProHidden(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Persona no encontrada.")
		return
	}
	var in struct {
		Hidden bool    `json:"hidden"`
		Bio    *string `json:"bio"` // formación y experiencia para el perfil público (nil = no cambiar)
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Bio != nil {
		*in.Bio = strings.TrimSpace(*in.Bio)
		if utf8.RuneCountInString(*in.Bio) > 600 {
			writeError(w, http.StatusBadRequest, "La semblanza es demasiado larga (máximo 600 caracteres).")
			return
		}
	}
	p := principalFrom(r.Context())
	tag, err := s.db.Exec(r.Context(), `UPDATE users SET public_hidden = $3, public_bio = coalesce($4, public_bio) WHERE id = $1 AND clinic_id = $2 AND role IN ('admin', 'doctor')`, id, p.ClinicID, in.Hidden, in.Bio)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "Persona no encontrada.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// publicMedia are the photo ids a public page links to.
func (s *Server) publicMedia(ctx context.Context, clinicID string) (profile, cover *string, gallery []string, err error) {
	rows, err := s.db.Query(ctx, `SELECT id::text, slot FROM clinic_media WHERE clinic_id = $1 AND slot <> 'pro' ORDER BY created_at`, clinicID)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()
	gallery = []string{}
	for rows.Next() {
		var id, slot string
		if err := rows.Scan(&id, &slot); err != nil {
			return nil, nil, nil, err
		}
		id2 := id
		switch slot {
		case "profile":
			profile = &id2
		case "cover":
			cover = &id2
		default:
			gallery = append(gallery, id)
		}
	}
	return profile, cover, gallery, rows.Err()
}

// decodeLoose is decode for a body whose size limit the caller already set (an image as a data URL).
func decodeLoose(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "La imagen pesa demasiado. Elige una más pequeña.")
		} else {
			writeError(w, http.StatusBadRequest, "Solicitud inválida.")
		}
		return false
	}
	return true
}
