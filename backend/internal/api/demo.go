package api

import (
	"fmt"
	"html"
	"net/http"
	netmail "net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

// demoRequest es lo que escribe quien pide una demostración en el sitio público.
type demoRequest struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Clinic    string `json:"clinic"`
	Specialty string `json:"specialty"`
	Message   string `json:"message"`
	// Website es una trampa para robots: queda oculto en el formulario y una persona nunca lo llena.
	Website string `json:"website"`
}

// mountPublicDemo registra POST /public/demo: avisa por correo a DEMO_TO (o a la dirección con la que se envía el
// correo de la plataforma) con copia a DEMO_CC, y responde al visitante con "Reply-To" para contestarle directo.
func (s *Server) mountPublicDemo(r chi.Router) {
	// pocos envíos por dirección IP y por hora: frena el abuso del formulario (una instancia por servidor)
	lim := newRateLimiter(8, time.Hour)
	r.Post("/public/demo", func(w http.ResponseWriter, r *http.Request) { s.demoRequest(w, r, lim) })
}

func demoClip(v string, max int) string {
	v = strings.TrimSpace(v)
	if utf8.RuneCountInString(v) > max {
		v = string([]rune(v)[:max])
	}
	return v
}

func (s *Server) demoRequest(w http.ResponseWriter, r *http.Request, lim *rateLimiter) {
	var in demoRequest
	if !decode(w, r, &in) {
		return
	}
	// un robot llenó la trampa: se responde como si todo hubiera salido bien y no se envía nada
	if strings.TrimSpace(in.Website) != "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !limit(w, lim, "demo|"+clientIP(r)) {
		return
	}
	in.Name, in.Clinic, in.Specialty = demoClip(in.Name, 120), demoClip(in.Clinic, 160), demoClip(in.Specialty, 80)
	in.Phone, in.Message = demoClip(in.Phone, 40), demoClip(in.Message, 1500)
	addr, err := netmail.ParseAddress(strings.TrimSpace(in.Email))
	if in.Name == "" || err != nil || len(addr.Address) > 200 {
		writeError(w, http.StatusBadRequest, "Escribe tu nombre y un correo válido para poder contestarte.")
		return
	}
	to := s.cfg.DemoTo
	if to == "" {
		if from, err := netmail.ParseAddress(s.cfg.MailFrom); err == nil {
			to = from.Address
		}
	}
	if !s.mailEnabled() || to == "" {
		writeError(w, http.StatusServiceUnavailable, "No pudimos recibir tu solicitud ahora. Escríbenos a "+contactFallback+".")
		return
	}
	line := func(label, v string) string {
		if v == "" {
			return ""
		}
		return label + ": " + v + "\n"
	}
	text := "Nueva solicitud de demo desde el sitio.\n\n" +
		line("Nombre", in.Name) + line("Correo", addr.Address) + line("Teléfono", in.Phone) + line("Consultorio", in.Clinic) +
		line("Especialidad", in.Specialty) + line("Mensaje", in.Message) +
		"\nPara contestar, responde a este correo: va directo a " + addr.Address + ".\n"
	row := func(label, v string) string {
		if v == "" {
			return ""
		}
		return fmt.Sprintf("<tr><td style=\"padding:4px 12px 4px 0;color:#5d6e7d\">%s</td><td style=\"padding:4px 0\">%s</td></tr>", label, html.EscapeString(v))
	}
	body := "<p><strong>Nueva solicitud de demo desde el sitio.</strong></p><table style=\"font:14px system-ui,sans-serif\">" +
		row("Nombre", in.Name) + row("Correo", addr.Address) + row("Teléfono", in.Phone) + row("Consultorio", in.Clinic) +
		row("Especialidad", in.Specialty) + row("Mensaje", in.Message) + "</table><p>Para contestar, responde a este correo: va directo al visitante.</p>"
	subject := "Solicitud de demo: " + in.Name
	if in.Clinic != "" {
		subject += " · " + in.Clinic
	}
	s.sendMail(mail.Message{To: []string{to}, Cc: s.cfg.DemoCC, ReplyTo: addr.String(), Subject: subject, Text: text, HTML: body})
	w.WriteHeader(http.StatusNoContent)
}

const contactFallback = "caresia-info@caresia.mx"
