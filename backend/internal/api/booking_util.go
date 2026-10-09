package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata" // clinics' time zones must resolve even on minimal images

	"github.com/jackc/pgx/v5"
)

const defaultTimeZone = "America/Mexico_City"

// apptInfo is what the reminder and booking e-mails need to describe a visit.
type apptInfo struct {
	ClinicName, ClinicAddress, ClinicPhone string
	Professional, Service                  string
	PatientName                            string // first name only
	Start                                  time.Time
	Token                                  string
	Slug                                   string // booking slug when online booking is enabled
}

// clinicLocation returns the clinic's time zone (clinics.settings.timezone) or Mexico City.
func clinicLocation(ctx context.Context, q queryRower, clinicID string) *time.Location {
	var tz string
	_ = q.QueryRow(ctx, `SELECT coalesce(settings->>'timezone', '') FROM clinics WHERE id = $1`, clinicID).Scan(&tz)
	return locationOrDefault(tz)
}

func locationOrDefault(tz string) *time.Location {
	if tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
	}
	loc, err := time.LoadLocation(defaultTimeZone)
	if err != nil {
		return time.FixedZone("CST", -6*3600)
	}
	return loc
}

// localTime builds the instant of date ("YYYY-MM-DD") at clock ("HH:MM[:SS]") in loc.
func localTime(loc *time.Location, date, clock string) (time.Time, error) {
	if len(clock) > 5 {
		clock = clock[:5]
	}
	return time.ParseInLocation("2006-01-02 15:04", date+" "+clock, loc)
}

var (
	monthsES   = []string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}
	weekdaysES = []string{"domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"}
)

func longDateES(t time.Time) string {
	return fmt.Sprintf("%s %d de %s de %d", weekdaysES[t.Weekday()], t.Day(), monthsES[t.Month()-1], t.Year())
}

func clockES(t time.Time) string { return t.Format("15:04") }

func firstName(names string) string {
	f := strings.Fields(names)
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

// newToken is a 256-bit URL-safe random token.
func newToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

var tokenShape = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

func validToken(t string) bool { return tokenShape.MatchString(t) }

// normalizePhoneMX returns the E.164 form (+52XXXXXXXXXX) of a Mexican phone number, or "" when it
// does not look like one. It accepts spaces, dashes, parentheses, "+52", "52", "044"/"045" and the
// old "521" mobile prefix.
func normalizePhoneMX(raw string) string {
	var d strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			d.WriteRune(r)
		}
	}
	n := d.String()
	switch {
	case strings.HasPrefix(n, "521") && len(n) == 13:
		n = n[3:]
	case strings.HasPrefix(n, "52") && len(n) == 12:
		n = n[2:]
	case (strings.HasPrefix(n, "044") || strings.HasPrefix(n, "045")) && len(n) == 13:
		n = n[3:]
	case strings.HasPrefix(n, "01") && len(n) == 12:
		n = n[2:]
	}
	if len(n) != 10 || n[0] == '0' {
		return ""
	}
	return "+52" + n
}

func validEmail(e string) bool {
	if len(e) > 254 || strings.ContainsAny(e, " \r\n<>") {
		return false
	}
	a, err := mail.ParseAddress(e)
	return err == nil && a.Address == e && strings.Contains(e[strings.LastIndex(e, "@"):], ".")
}

// manageLink is the page where the patient confirms, cancels or reschedules.
func (s *Server) manageLink(token string, query string) string {
	l := s.appLink("/cita/" + url.PathEscape(token))
	if query != "" {
		l += "?" + query
	}
	return l
}

func (a apptInfo) details() string {
	var b strings.Builder
	b.WriteString(`<table role="presentation" cellpadding="0" cellspacing="0" style="width:100%;background:#f4f8fb;border-radius:12px;padding:16px;margin:8px 0">`)
	row := func(label, value string) {
		if value == "" {
			return
		}
		b.WriteString(`<tr><td style="padding:3px 12px 3px 0;font-size:13px;color:#7a8b9b;white-space:nowrap;vertical-align:top">` + esc(label) + `</td><td style="padding:3px 0;font-size:15px;color:#0b2540;font-weight:600">` + esc(value) + `</td></tr>`)
	}
	row("Fecha", longDateES(a.Start))
	row("Hora", clockES(a.Start))
	row("Consultorio", a.ClinicName)
	row("Atiende", a.Professional)
	row("Servicio", a.Service)
	row("Dirección", a.ClinicAddress)
	row("Teléfono", a.ClinicPhone)
	b.WriteString(`</table>`)
	return b.String()
}

func (a apptInfo) detailsText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Fecha: %s\nHora: %s\nConsultorio: %s\n", longDateES(a.Start), clockES(a.Start), a.ClinicName)
	if a.Professional != "" {
		b.WriteString("Atiende: " + a.Professional + "\n")
	}
	if a.Service != "" {
		b.WriteString("Servicio: " + a.Service + "\n")
	}
	if a.ClinicAddress != "" {
		b.WriteString("Dirección: " + a.ClinicAddress + "\n")
	}
	if a.ClinicPhone != "" {
		b.WriteString("Teléfono: " + a.ClinicPhone + "\n")
	}
	return b.String()
}

func smallLink(href, label string) string {
	return `<a href="` + esc(href) + `" style="color:#1673d1;text-decoration:underline">` + esc(label) + `</a>`
}

// actionRow renders the three buttons of a reminder.
func (s *Server) actionRow(token string) string {
	btn := func(href, label, bg, fg string) string {
		return `<a href="` + esc(href) + `" style="background:` + bg + `;color:` + fg + `;text-decoration:none;padding:11px 18px;border-radius:999px;font-weight:600;font-size:14px;display:inline-block;margin:0 8px 8px 0;border:1px solid #0b2540">` + esc(label) + `</a>`
	}
	return `<p style="margin:20px 0 6px">` +
		btn(s.manageLink(token, "accion=confirmar"), "Confirmar asistencia", "#0b2540", "#ffffff") +
		btn(s.manageLink(token, "accion=reagendar"), "Reagendar", "#ffffff", "#0b2540") +
		btn(s.manageLink(token, "accion=cancelar"), "Cancelar", "#ffffff", "#0b2540") + `</p>`
}

// attendRow asks the patient the one question of the reminder: will you come? Not answering changes nothing.
func (s *Server) attendRow(token string) string {
	btn := func(href, label, bg, fg string) string {
		return `<a href="` + esc(href) + `" style="background:` + bg + `;color:` + fg + `;text-decoration:none;padding:12px 22px;border-radius:999px;font-weight:600;font-size:15px;display:inline-block;margin:0 8px 8px 0;border:1px solid #0b2540">` + esc(label) + `</a>`
	}
	return `<p style="margin:20px 0 6px;font-size:16px;font-weight:600">¿Asistirás a tu cita?</p><p style="margin:0 0 6px">` +
		btn(s.manageLink(token, "accion=confirmar"), "Sí, asistiré", "#0b2540", "#ffffff") +
		btn(s.manageLink(token, "accion=cancelar"), "No podré asistir", "#ffffff", "#0b2540") + `</p>` +
		`<p style="margin:0;font-size:13px;color:#7a8b9b">Si no respondes, tu cita sigue agendada. ¿Prefieres otro día? ` + smallLink(s.manageLink(token, "accion=reagendar"), "Reagendar") + `.</p>`
}

// reminderMail builds the reminder e-mail. note is the clinic's own text from the agenda settings.
func (s *Server) reminderMail(a apptInfo, note string) (subject, text, html string) {
	hello := "Hola"
	if a.PatientName != "" {
		hello = "Hola " + a.PatientName
	}
	when := fmt.Sprintf("el %s a las %s", longDateES(a.Start), clockES(a.Start))
	subject = "Recordatorio de tu cita en " + a.ClinicName + " · " + a.Start.Format("02/01") + " " + clockES(a.Start)
	confirm, optout := s.manageLink(a.Token, "accion=confirmar"), s.manageLink(a.Token, "baja=1")
	text = hello + ",\n\nTe recordamos tu cita en " + a.ClinicName + " " + when + ".\n\n" + a.detailsText()
	if note != "" {
		text += "\n" + note + "\n"
	}
	text += "\n¿Asistirás a tu cita?\nSí, asistiré: " + confirm + "\nNo podré asistir: " + s.manageLink(a.Token, "accion=cancelar") + "\n\nSi no respondes, tu cita sigue agendada. ¿Prefieres otro día? Reagendar: " + s.manageLink(a.Token, "accion=reagendar") + "\n\nSi ya no quieres recibir recordatorios:\n" + optout + "\n"
	body := `<p style="margin:0 0 8px">` + esc(hello) + `, te recordamos tu cita en <strong>` + esc(a.ClinicName) + `</strong> ` + esc(when) + `.</p>` + a.details()
	if note != "" {
		body += `<p style="margin:12px 0">` + esc(note) + `</p>`
	}
	body += s.attendRow(a.Token) +
		`<p style="font-size:13px;color:#7a8b9b;margin:14px 0 0">Si ya no quieres recibir recordatorios, ` + smallLink(optout, "date de baja aquí") + `.</p>`
	return subject, text, layout("Recordatorio de tu cita", body)
}

// bookingMail is the confirmation sent right after booking online.
func (s *Server) bookingMail(a apptInfo, pendingClinic bool) (subject, text, html string) {
	hello := "Hola"
	if a.PatientName != "" {
		hello = "Hola " + a.PatientName
	}
	title, lead := "Tu cita quedó agendada", "tu cita quedó agendada."
	if pendingClinic {
		title, lead = "Recibimos tu solicitud de cita", "recibimos tu solicitud. El consultorio la revisará y te avisará si hay algún cambio."
	}
	manage := s.manageLink(a.Token, "")
	subject = title + " · " + a.ClinicName
	text = hello + ", " + lead + "\n\n" + a.detailsText() + "\nPuedes confirmar, reagendar o cancelar aquí:\n" + manage + "\n"
	body := `<p style="margin:0 0 8px">` + esc(hello) + `, ` + esc(lead) + `</p>` + a.details() + button(manage, "Administrar mi cita")
	return subject, text, layout(title, body)
}

// loadApptInfo gathers the data describing one appointment of a clinic. ok is false when it does not exist.
func (s *Server) loadApptInfo(ctx context.Context, q queryRower, clinicID, appointmentID string) (a apptInfo, ok bool, err error) {
	var date, start, tz string
	var enabled bool
	var slug *string
	err = q.QueryRow(ctx, `
		SELECT c.name, c.address, c.phone_number, coalesce(u.name, ''), coalesce(ci.name, ''), ap.names,
		       to_char(ap.date, 'YYYY-MM-DD'), to_char(ap.start_hour, 'HH24:MI'), coalesce(ap.confirm_token, ''),
		       coalesce(c.settings->>'timezone', ''), coalesce(s.booking_enabled, false), s.booking_slug
		FROM appointments ap
		JOIN clinics c ON c.id = ap.clinic_id
		LEFT JOIN users u ON u.id = ap.professional_id
		LEFT JOIN catalog_items ci ON ci.id = ap.service_id AND ci.clinic_id = ap.clinic_id
		LEFT JOIN agenda_settings s ON s.clinic_id = ap.clinic_id
		WHERE ap.clinic_id = $1 AND ap.id = $2`, clinicID, appointmentID).
		Scan(&a.ClinicName, &a.ClinicAddress, &a.ClinicPhone, &a.Professional, &a.Service, &a.PatientName, &date, &start, &a.Token, &tz, &enabled, &slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, false, nil
	}
	if err != nil {
		return a, false, err
	}
	a.PatientName = firstName(a.PatientName)
	if enabled && slug != nil {
		a.Slug = *slug
	}
	a.Start, err = localTime(locationOrDefault(tz), date, start)
	return a, err == nil, err
}
