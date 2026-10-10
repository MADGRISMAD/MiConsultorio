package api

import (
	"net/http"
	"strings"
)

// bookingCheck tells a signed-in team member why the public booking page of their clinic does not open,
// and where to fix it. Visitors never reach it: the public page itself keeps answering a plain miss.
func (s *Server) bookingCheck(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	slug := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("slug")))
	var saved string
	var enabled bool
	var status string
	var suspended bool
	err := s.db.QueryRow(r.Context(), `
		SELECT coalesce(a.booking_slug, ''), coalesce(a.booking_enabled, false), c.billing_status, c.branch_suspended_at IS NOT NULL
		FROM clinics c LEFT JOIN agenda_settings a ON a.clinic_id = c.id WHERE c.id = $1`, p.ClinicID).Scan(&saved, &enabled, &status, &suspended)
	if err != nil {
		serverError(w, r, err)
		return
	}
	var bookable, consulting int
	_ = s.db.QueryRow(r.Context(), `
		SELECT count(*) FILTER (WHERE ps.bookable AND ps.consults), count(*) FILTER (WHERE ps.consults)
		FROM professional_settings ps JOIN users u ON u.id = ps.user_id
		WHERE ps.clinic_id = $1 AND u.clinic_id = $1 AND NOT u.disabled`, p.ClinicID).Scan(&bookable, &consulting)

	out := func(problem, title, text, to, action string) {
		writeJSON(w, http.StatusOK, map[string]any{"problem": problem, "title": title, "text": text, "to": to, "action": action, "slug": saved})
	}
	switch {
	case status != "active" && status != "trialing" && status != "past_due":
		out("billing", "Tu suscripción no está activa", "Mientras la suscripción esté cancelada o vencida, tu página de citas no se muestra a los pacientes.", "/suscripcion", "Ir a Suscripción")
	case suspended:
		out("branch", "Esta sucursal está suspendida", "Una sucursal suspendida no muestra su página de citas. Revisa tu plan y tus sucursales.", "/suscripcion", "Ir a Suscripción")
	case saved == "":
		out("no_slug", "Aún no tienes un enlace de citas", "Elige el enlace que usarán tus pacientes y activa las reservas en línea.", "/ajustes?s=agenda", "Ir a Agenda y reservas")
	case !enabled:
		out("disabled", "Las reservas en línea están apagadas", "Activa «Reservas en línea» para que tus pacientes puedan abrir este enlace.", "/ajustes?s=agenda", "Activar en Agenda y reservas")
	case slug != "" && slug != saved:
		out("slug_mismatch", "El enlace no coincide con el de tu consultorio", "Tu enlace correcto es /reservar/"+saved+". Revisa que esté completo, sin espacios ni letras de más.", "/reservar/"+saved, "Abrir mi enlace correcto")
	case consulting == 0:
		out("no_consults", "Nadie atiende consultas en la agenda", "Marca «Atiendo consultas» en al menos un profesional.", "/ajustes?s=agenda", "Ir a Agenda y reservas")
	case bookable == 0:
		out("no_bookable", "Ningún profesional aparece en la reserva en línea", "Activa «Aparece en la reserva en línea» en al menos un profesional.", "/ajustes?s=agenda", "Ir a Agenda y reservas")
	default:
		out("", "", "", "", "")
	}
}
