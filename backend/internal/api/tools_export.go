package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/madgrismad/miconsultorio/backend/internal/config"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

// ToolClient lets command-line tools (cmd/seedmock) call the real API in-process, signed in as an existing user.
// Every rule, validation and encryption of the HTTP handlers applies; nothing goes over the network and no e-mail is sent.
type ToolClient struct {
	h      http.Handler
	token  string
	UserID string
	// ClinicID is the clinic of the signed-in user.
	ClinicID string
}

// noMail is a sender that is never enabled: features that would send e-mail skip it.
type noMail struct{}

func (noMail) Enabled() bool                            { return false }
func (noMail) Send(context.Context, mail.Message) error { return nil }

// NewToolClient signs in the account with that e-mail (no password needed: the tool runs where the database is).
func NewToolClient(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, email string) (*ToolClient, error) {
	s := newServer(pool, cfg, noMail{})
	var id string
	if err := pool.QueryRow(ctx, `SELECT id FROM users WHERE lower(email) = lower($1) AND linked_owner_id IS NULL`, email).Scan(&id); err != nil {
		return nil, fmt.Errorf("no hay una cuenta con el correo %q", email)
	}
	p, err := loadPrincipal(ctx, pool, id)
	if err != nil {
		return nil, err
	}
	token, err := s.issueSession(p)
	if err != nil {
		return nil, err
	}
	return &ToolClient{h: s.router(), token: token, UserID: p.UserID, ClinicID: p.ClinicID}, nil
}

// Do sends one request. A body is sent as JSON; out (optional) receives the JSON answer. It returns the HTTP status,
// and an error carrying the API's message when the status is not 2xx.
func (c *ToolClient) Do(method, path string, body, out any) (int, error) {
	var rdr *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		rdr = bytes.NewReader(raw)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, "/api"+path, rdr)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: c.token})
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)
	if rec.Code >= 300 {
		var e struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &e)
		if e.Message == "" {
			e.Message = http.StatusText(rec.Code)
		}
		return rec.Code, &ToolError{Status: rec.Code, Code: e.Code, Message: e.Message}
	}
	if out != nil && rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			return rec.Code, err
		}
	}
	return rec.Code, nil
}

// ToolError is a non-2xx answer of the API.
type ToolError struct {
	Status  int
	Code    string
	Message string
}

func (e *ToolError) Error() string { return fmt.Sprintf("%d %s", e.Status, e.Message) }
