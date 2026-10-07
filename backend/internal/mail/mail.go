// Package mail sends e-mail over SMTP (STARTTLS on 587 or implicit TLS on 465).
package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// Message is one e-mail with a plain-text and an HTML body.
type Message struct {
	To      []string
	Subject string
	Text    string
	HTML    string
}

// Sender delivers messages. The API depends on this interface so tests can capture mail.
type Sender interface {
	Enabled() bool
	Send(ctx context.Context, m Message) error
}

// SMTP is the real sender.
type SMTP struct {
	Host   string
	Port   int
	User   string
	Pass   string
	From   string // "Name <address>" or just an address
	Secure bool   // implicit TLS (port 465); otherwise STARTTLS is used when the server offers it
}

// Enabled reports whether enough is configured to send.
func (s *SMTP) Enabled() bool { return s != nil && s.Host != "" && s.From != "" }

func clean(v string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(strings.TrimSpace(v))
}

func (s *SMTP) Send(ctx context.Context, m Message) error {
	if !s.Enabled() {
		return errors.New("mail: SMTP no configurado")
	}
	from, err := mail.ParseAddress(clean(s.From))
	if err != nil {
		return fmt.Errorf("mail: remitente inválido: %w", err)
	}
	var rcpts []string
	var toHdr []string
	for _, t := range m.To {
		a, err := mail.ParseAddress(clean(t))
		if err != nil {
			return fmt.Errorf("mail: destinatario inválido: %w", err)
		}
		rcpts = append(rcpts, a.Address)
		toHdr = append(toHdr, a.String())
	}
	if len(rcpts) == 0 {
		return errors.New("mail: sin destinatarios")
	}
	raw := build(from.String(), toHdr, m)

	port := s.Port
	if port == 0 {
		port = 587
		if s.Secure {
			port = 465
		}
	}
	addr := net.JoinHostPort(s.Host, fmt.Sprint(port))
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	deadline := time.Now().Add(30 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	var conn net.Conn
	if s.Secure {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("mail: conexión: %w", err)
	}
	_ = conn.SetDeadline(deadline)
	c, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("mail: saludo: %w", err)
	}
	defer c.Close()
	if !s.Secure {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}); err != nil {
				return fmt.Errorf("mail: starttls: %w", err)
			}
		} else if s.User != "" {
			return errors.New("mail: el servidor no ofrece STARTTLS; no se envían credenciales sin cifrar")
		}
	}
	if s.User != "" {
		if err := c.Auth(smtp.PlainAuth("", s.User, s.Pass, s.Host)); err != nil {
			return fmt.Errorf("mail: autenticación: %w", err)
		}
	}
	if err := c.Mail(from.Address); err != nil {
		return fmt.Errorf("mail: remitente: %w", err)
	}
	for _, r := range rcpts {
		if err := c.Rcpt(r); err != nil {
			return fmt.Errorf("mail: destinatario: %w", err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("mail: datos: %w", err)
	}
	if _, err := w.Write(raw); err != nil {
		return fmt.Errorf("mail: escritura: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mail: cierre: %w", err)
	}
	return c.Quit()
}

// build makes a multipart/alternative message with quoted-printable parts.
func build(from string, to []string, m Message) []byte {
	boundary := fmt.Sprintf("caresia-%d", time.Now().UnixNano())
	var b bytes.Buffer
	h := func(k, v string) { fmt.Fprintf(&b, "%s: %s\r\n", k, v) }
	h("From", from)
	h("To", strings.Join(to, ", "))
	h("Subject", mime.QEncoding.Encode("utf-8", clean(m.Subject)))
	h("Date", time.Now().Format(time.RFC1123Z))
	h("MIME-Version", "1.0")
	h("Content-Type", fmt.Sprintf(`multipart/alternative; boundary="%s"`, boundary))
	b.WriteString("\r\n")
	part := func(ctype, body string) {
		fmt.Fprintf(&b, "--%s\r\nContent-Type: %s; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n", boundary, ctype)
		w := quotedprintable.NewWriter(&b)
		_, _ = w.Write([]byte(body))
		_ = w.Close()
		b.WriteString("\r\n")
	}
	part("text/plain", m.Text)
	if m.HTML != "" {
		part("text/html", m.HTML)
	}
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return b.Bytes()
}
