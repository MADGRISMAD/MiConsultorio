package api

import (
	"fmt"
	"html"
	netmail "net/mail"
	"strings"

	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

// signupNotice tells the platform team that someone created an account, with the plan they asked to try (the "Solicitar demo"
// card they pressed) so it can be activated for them: every trial starts on Básico. It goes to DEMO_TO (or the address the
// platform mail is sent from) with copy to DEMO_CC, and answering it writes to the person.
func (s *Server) signupNotice(req registerRequest, requestedPlan, clinicID string) {
	to, cc := s.teamRecipients()
	if to == "" {
		return
	}
	asked := "Ninguno (Probar gratis)"
	if pl, ok := planByID(requestedPlan); ok {
		asked = pl.Name
	}
	link := s.appLink("/plataforma/negocios/" + clinicID)
	lines := [][2]string{
		{"Plan solicitado", asked}, {"Plan activado", "Básico (prueba de " + itoa(TrialDays) + " días)"},
		{"Nombre", req.Name}, {"Consultorio", req.ClinicName}, {"Correo", req.Email}, {"Teléfono", req.Phone}, {"Usuario", req.Username},
	}
	text := "Nuevo registro desde el sitio.\n\n"
	rows := ""
	for _, l := range lines {
		if l[1] == "" {
			continue
		}
		text += l[0] + ": " + l[1] + "\n"
		rows += fmt.Sprintf("<tr><td style=\"padding:4px 12px 4px 0;color:#5d6e7d\">%s</td><td style=\"padding:4px 0\">%s</td></tr>", l[0], html.EscapeString(l[1]))
	}
	text += "\nSi pidió otro plan, actívalo desde Plataforma > Negocios > Licencia:\n" + link + "\n"
	body := "<p><strong>Nuevo registro desde el sitio.</strong></p><table style=\"font:14px system-ui,sans-serif\">" + rows +
		"</table><p>Si pidió otro plan, actívalo desde <a href=\"" + html.EscapeString(link) + "\">Plataforma &gt; Negocios &gt; Licencia</a>.</p>"
	subject := "Nuevo registro · plan solicitado: " + asked
	if req.ClinicName != "" {
		subject += " · " + strings.TrimSpace(req.ClinicName)
	}
	m := mail.Message{To: []string{to}, Cc: cc, Subject: subject, Text: text, HTML: body}
	if addr, err := netmail.ParseAddress(req.Email); err == nil {
		m.ReplyTo = addr.String()
	}
	s.sendMail(m)
}
