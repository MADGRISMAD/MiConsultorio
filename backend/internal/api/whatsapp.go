package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// waError is a failed call to the WhatsApp Cloud API. Permanent errors (a bad number, a rejected
// template) are not worth retrying.
type waError struct {
	Status    int
	Detail    string
	Permanent bool
}

func (e *waError) Error() string {
	if e.Status == 0 {
		return e.Detail
	}
	return fmt.Sprintf("HTTP %d: %s", e.Status, e.Detail)
}

func (s *Server) whatsAppEnabled() bool {
	return s.cfg.WhatsAppToken != "" && s.cfg.WhatsAppPhoneID != ""
}

var waHTTP = &http.Client{Timeout: 20 * time.Second}

// sendWhatsAppReminder sends the approved template with four body parameters:
// patient name, date, time and clinic. phone is E.164 (+52...).
func (s *Server) sendWhatsAppReminder(ctx context.Context, phone string, a apptInfo) error {
	name := a.PatientName
	if name == "" {
		name = "paciente"
	}
	param := func(v string) map[string]string { return map[string]string{"type": "text", "text": v} }
	payload := map[string]any{
		"messaging_product": "whatsapp",
		"to":                strings.TrimPrefix(phone, "+"),
		"type":              "template",
		"template": map[string]any{
			"name":     s.cfg.WhatsAppTemplate,
			"language": map[string]string{"code": s.cfg.WhatsAppLang},
			"components": []map[string]any{{
				"type":       "body",
				"parameters": []map[string]string{param(name), param(longDateES(a.Start)), param(clockES(a.Start)), param(a.ClinicName)},
			}},
		},
	}
	raw, _ := json.Marshal(payload)
	base := s.cfg.WhatsAppAPIBase
	if base == "" {
		base = "https://graph.facebook.com/v20.0"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/"+s.cfg.WhatsAppPhoneID+"/messages", bytes.NewReader(raw))
	if err != nil {
		return &waError{Detail: err.Error(), Permanent: true}
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.WhatsAppToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := waHTTP.Do(req)
	if err != nil {
		return &waError{Detail: err.Error()}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	if resp.StatusCode/100 == 2 {
		return nil
	}
	var parsed struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	detail := strings.TrimSpace(string(body))
	if json.Unmarshal(body, &parsed) == nil && parsed.Error.Message != "" {
		detail = parsed.Error.Message
	}
	retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
	return &waError{Status: resp.StatusCode, Detail: truncate(detail, 200), Permanent: !retryable}
}
