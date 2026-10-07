package mail

import (
	"bufio"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/textproto"
	"strings"
	"testing"
	"time"
)

// fakeSMTP is a minimal server (no TLS, no auth) that records the DATA of one message.
func fakeSMTP(t *testing.T) (port int, got chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	got = make(chan string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		r := bufio.NewReader(c)
		say := func(s string) { c.Write([]byte(s + "\r\n")) }
		say("220 fake ESMTP")
		var rcpts []string
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			cmd := strings.ToUpper(strings.TrimSpace(line))
			switch {
			case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
				say("250 fake")
			case strings.HasPrefix(cmd, "MAIL FROM"):
				say("250 ok")
			case strings.HasPrefix(cmd, "RCPT TO"):
				rcpts = append(rcpts, strings.TrimSpace(line))
				say("250 ok")
			case cmd == "DATA":
				say("354 go")
				body, _ := io.ReadAll(textproto.NewReader(r).DotReader())
				got <- strings.Join(rcpts, "|") + "\n" + string(body)
				say("250 queued")
			case cmd == "QUIT":
				say("221 bye")
				return
			default:
				say("250 ok")
			}
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, got
}

func TestSendBuildsMultipartUTF8(t *testing.T) {
	port, got := fakeSMTP(t)
	s := &SMTP{Host: "127.0.0.1", Port: port, From: "Caresia <no-reply@caresia.test>"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.Send(ctx, Message{To: []string{"Paciente <p@x.mx>"}, Subject: "Restablece tu contraseña\r\nBcc: evil@x.mx", Text: "Hola ñandú ¿qué tal?", HTML: "<p>Hola ñandú</p>"})
	if err != nil {
		t.Fatal(err)
	}
	raw := <-got
	envelope, msg, _ := strings.Cut(raw, "\n")
	if !strings.Contains(envelope, "p@x.mx") {
		t.Fatalf("envelope: %q", envelope)
	}
	m, err := readMessage(msg)
	if err != nil {
		t.Fatal(err)
	}
	if m.header.Get("Bcc") != "" {
		t.Fatal("header injection through the subject must be impossible")
	}
	subj, _ := new(mime.WordDecoder).DecodeHeader(m.header.Get("Subject"))
	if !strings.HasPrefix(subj, "Restablece tu contraseña") {
		t.Fatalf("subject: %q", subj)
	}
	if len(m.parts) != 2 || !strings.Contains(m.parts[0], "ñandú ¿qué tal?") || !strings.Contains(m.parts[1], "<p>Hola ñandú</p>") {
		t.Fatalf("parts: %q", m.parts)
	}
}

func TestDisabledAndRefusesPlainAuth(t *testing.T) {
	if (&SMTP{}).Enabled() {
		t.Fatal("empty config must be disabled")
	}
	port, _ := fakeSMTP(t)
	s := &SMTP{Host: "127.0.0.1", Port: port, User: "u", Pass: "p", From: "a@b.mx"}
	if err := s.Send(context.Background(), Message{To: []string{"x@y.mx"}, Subject: "s", Text: "t"}); err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("credentials must never travel without TLS, got %v", err)
	}
}

type parsed struct {
	header textproto.MIMEHeader
	parts  []string
}

func readMessage(raw string) (parsed, error) {
	tp := textproto.NewReader(bufio.NewReader(strings.NewReader(raw)))
	h, err := tp.ReadMIMEHeader()
	if err != nil {
		return parsed{}, err
	}
	_, params, _ := mime.ParseMediaType(h.Get("Content-Type"))
	rest, _ := io.ReadAll(tp.R)
	mr := multipart.NewReader(strings.NewReader(string(rest)), params["boundary"])
	out := parsed{header: h}
	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}
		b, _ := io.ReadAll(quotedprintable.NewReader(p))
		out.parts = append(out.parts, string(b))
	}
	return out, nil
}
