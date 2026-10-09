package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// Business types ("giro"). The first one chosen is the clinic's main kind; a clinic with
// several specialties lists the others in `specialties`.
var clinicKindList = []string{
	"GENERAL_MEDICAL", "DENTAL", "PEDIATRICS", "INTERNAL_MEDICINE", "PHYSIOTHERAPY", "NUTRITION",
	"PSYCHOLOGY", "DERMATOLOGY", "GYNECOLOGY", "ORTHOPEDICS", "VETERINARY", "CHIROPRACTIC",
}

var clinicKinds = func() map[string]bool {
	m := map[string]bool{}
	for _, k := range clinicKindList {
		m[k] = true
	}
	return m
}()

// ---------------------------------------------------------------------------
// Scheduling settings
// ---------------------------------------------------------------------------

type DayHours struct {
	Open  bool   `json:"open"`
	Start string `json:"start"` // HH:MM
	End   string `json:"end"`
}

// Settings are the clinic's scheduling preferences, edited in the setup wizard.
type Settings struct {
	Hours              map[string]DayHours `json:"hours"` // keys: mon tue wed thu fri sat sun
	AppointmentMinutes int                 `json:"appointment_minutes"`
}

var weekdays = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

func defaultSettings() Settings {
	s := Settings{Hours: map[string]DayHours{}, AppointmentMinutes: 30}
	for i, d := range weekdays {
		s.Hours[d] = DayHours{Open: i < 5, Start: "09:00", End: "18:00"} // Monday to Friday
	}
	return s
}

// normalized fills whatever is missing with the defaults, so clients always get a full object.
func (s Settings) normalized() Settings {
	def := defaultSettings()
	if s.AppointmentMinutes == 0 {
		s.AppointmentMinutes = def.AppointmentMinutes
	}
	if s.Hours == nil {
		s.Hours = map[string]DayHours{}
	}
	for _, d := range weekdays {
		if _, ok := s.Hours[d]; !ok {
			s.Hours[d] = def.Hours[d]
		}
	}
	return s
}

func (s Settings) validate() string {
	if s.AppointmentMinutes < 5 || s.AppointmentMinutes > 240 || s.AppointmentMinutes%5 != 0 {
		return "La duración de las citas debe estar entre 5 y 240 minutos, en pasos de 5."
	}
	for day := range s.Hours {
		if !hasPermission(weekdays, day) {
			return "Día inválido en el horario."
		}
	}
	for _, d := range weekdays {
		h, ok := s.Hours[d]
		if !ok || !h.Open {
			continue
		}
		start, e1 := time.Parse("15:04", h.Start)
		end, e2 := time.Parse("15:04", h.End)
		if e1 != nil || e2 != nil {
			return "Las horas del horario no son válidas."
		}
		if !end.After(start) {
			return "En cada día abierto, la hora de cierre debe ser posterior a la de apertura."
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Clinic profile
// ---------------------------------------------------------------------------

type clinic struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	Specialties    []string `json:"specialties"`
	Name           string   `json:"name"`
	PhoneNumber    string   `json:"phone_number"`
	Address        string   `json:"address"`
	ImageURL       string   `json:"image_url"`
	Settings       Settings `json:"settings"`
	SetupCompleted bool     `json:"setup_completed"`
}

func loadClinic(ctx context.Context, q queryRower, id string) (clinic, error) {
	var c clinic
	var raw []byte
	err := q.QueryRow(ctx, `
		SELECT id, kind, specialties, name, phone_number, address, image_url, settings, setup_completed_at IS NOT NULL
		FROM clinics WHERE id = $1`, id).
		Scan(&c.ID, &c.Kind, &c.Specialties, &c.Name, &c.PhoneNumber, &c.Address, &c.ImageURL, &raw, &c.SetupCompleted)
	if err != nil {
		return c, err
	}
	_ = json.Unmarshal(raw, &c.Settings)
	c.Settings = c.Settings.normalized()
	if c.Specialties == nil {
		c.Specialties = []string{}
	}
	return c, nil
}

func (s *Server) clinicInfo(w http.ResponseWriter, r *http.Request) {
	c, err := loadClinic(r.Context(), s.db, principalFrom(r.Context()).ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"clinic": c})
}

type clinicUpdate struct {
	Name        *string   `json:"name"`
	PhoneNumber *string   `json:"phone_number"`
	Address     *string   `json:"address"`
	Kind        *string   `json:"kind"`
	Specialties *[]string `json:"specialties"`
	Settings    *Settings `json:"settings"`
	ImageURL    *string   `json:"image_url"` // the clinic's logo or photo as a small data URL; empty goes back to the Caresia logo
}

// updateOwnClinic lets the clinic's administrator edit its profile and scheduling settings.
// Every field is optional so the wizard can save one step at a time.
func (s *Server) updateOwnClinic(w http.ResponseWriter, r *http.Request) {
	var req clinicUpdate
	if !decode(w, r, &req) {
		return
	}
	p := principalFrom(r.Context())
	var result clinic
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		if err := lockClinic(r.Context(), tx, p.ClinicID); err != nil {
			return err
		}
		c, err := loadClinic(r.Context(), tx, p.ClinicID)
		if err != nil {
			return err
		}
		if req.Name != nil {
			n := strings.TrimSpace(*req.Name)
			if l := utf8.RuneCountInString(n); l < 2 || l > 120 {
				return fail(http.StatusBadRequest, "El nombre del consultorio debe tener entre 2 y 120 caracteres.")
			}
			c.Name = n
		}
		if req.PhoneNumber != nil {
			if utf8.RuneCountInString(*req.PhoneNumber) > 30 {
				return fail(http.StatusBadRequest, "El teléfono es demasiado largo.")
			}
			c.PhoneNumber = strings.TrimSpace(*req.PhoneNumber)
		}
		if req.Address != nil {
			if utf8.RuneCountInString(*req.Address) > 250 {
				return fail(http.StatusBadRequest, "La dirección es demasiado larga.")
			}
			c.Address = strings.TrimSpace(*req.Address)
		}
		if req.ImageURL != nil {
			if msg := validLogo(*req.ImageURL); msg != "" {
				return fail(http.StatusBadRequest, msg)
			}
			c.ImageURL = *req.ImageURL
		}
		if req.Kind != nil {
			if !clinicKinds[*req.Kind] {
				return fail(http.StatusBadRequest, "Elige un giro válido.")
			}
			c.Kind = *req.Kind
		}
		if req.Specialties != nil {
			seen := map[string]bool{}
			c.Specialties = []string{}
			for _, k := range *req.Specialties {
				if !clinicKinds[k] {
					return fail(http.StatusBadRequest, "Especialidad inválida.")
				}
				if !seen[k] {
					seen[k] = true
					c.Specialties = append(c.Specialties, k)
				}
			}
		}
		// the main kind is never repeated among the extra specialties
		kept := c.Specialties[:0]
		for _, k := range c.Specialties {
			if k != c.Kind {
				kept = append(kept, k)
			}
		}
		c.Specialties = kept
		if req.Settings != nil {
			ns := req.Settings.normalized()
			if msg := ns.validate(); msg != "" {
				return fail(http.StatusBadRequest, msg)
			}
			c.Settings = ns
		}

		raw, err := json.Marshal(c.Settings)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `
			UPDATE clinics SET name=$2, phone_number=$3, address=$4, kind=$5, specialties=$6, settings=$7, image_url=$8, updated_at=now()
			WHERE id = $1`, p.ClinicID, c.Name, c.PhoneNumber, c.Address, c.Kind, c.Specialties, raw, c.ImageURL); err != nil {
			return err
		}
		audit(r.Context(), tx, p.ClinicID, p, "clinic_settings", "Actualizó la configuración del consultorio", nil)
		result, err = loadClinic(r.Context(), tx, p.ClinicID)
		return err
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"clinic": result})
}

// completeSetup marks the first-run wizard as done (finished or skipped).
func (s *Server) completeSetup(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	tag, err := s.db.Exec(r.Context(), `UPDATE clinics SET setup_completed_at = now() WHERE id = $1 AND setup_completed_at IS NULL`, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if tag.RowsAffected() > 0 {
		audit(r.Context(), s.db, p.ClinicID, p, "setup_completed", "Terminó la configuración inicial", nil)
	}
	fresh, err := loadPrincipal(r.Context(), s.db, p.UserID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, sessionResponse{Session: sessionOf(fresh)})
}

const maxLogoChars = 300_000

// validLogo accepts an empty string (the default Caresia logo) or a small PNG, JPEG or WebP as a base64 data URL.
func validLogo(v string) string {
	if v == "" {
		return ""
	}
	var rest string
	for _, pre := range []string{"data:image/png;base64,", "data:image/jpeg;base64,", "data:image/webp;base64,"} {
		if strings.HasPrefix(v, pre) {
			rest = v[len(pre):]
		}
	}
	if rest == "" || len(v) > maxLogoChars {
		return "La imagen debe ser PNG, JPG o WebP y pesar menos de 200 KB."
	}
	if _, err := base64.StdEncoding.DecodeString(rest); err != nil {
		return "La imagen no es válida."
	}
	return ""
}
