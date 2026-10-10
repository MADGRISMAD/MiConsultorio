package api_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCalendarFeed(t *testing.T) {
	b := newBookingEnv(t, false)
	ctx := context.Background()
	doc := b.login("doc_a")
	var pid string
	if err := b.pool.QueryRow(ctx, `INSERT INTO patients (clinic_id, file_number, names) VALUES ($1, 1, 'Ana') RETURNING id`, b.clinicA).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	b.exec(`INSERT INTO appointments (clinic_id, curp, names, last_names, date, start_hour, end_hour, patient_id, professional_id, status)
		VALUES ($1, 'X', 'Ana', 'Pérez', current_date + 2, '09:00', '09:30', $2, $3, 'confirmed'), ($1, 'X', 'Cancelada', 'Z', current_date + 2, '10:00', '10:30', $2, $3, 'cancelled'),
		       ($1, 'X', 'Ajena', 'Q', current_date + 2, '11:00', '11:30', $2, NULL, 'confirmed')`, b.clinicA, pid, b.pro)

	st := doc.expect(200, "GET", "/api/me/calendar", nil)
	if st["enabled"] != false {
		t.Fatalf("starts off: %v", st)
	}
	st = doc.expect(200, "POST", "/api/me/calendar", map[string]any{"action": "enable", "show_names": false})
	path := st["path"].(string)
	if !strings.HasPrefix(path, "/api/public/calendar/") {
		t.Fatalf("path: %v", st)
	}
	get := func(p string) (int, string) {
		res, err := http.Get(b.srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(body)
	}
	code, ics := get(path)
	if code != 200 || strings.Count(ics, "BEGIN:VEVENT") != 1 || strings.Contains(ics, "Ana") || strings.Contains(ics, "Cancelada") || strings.Contains(ics, "Ajena") {
		t.Fatalf("only my live appointment, without the name: %d\n%s", code, ics)
	}
	// with names on
	doc.expect(200, "POST", "/api/me/calendar", map[string]any{"action": "enable", "show_names": true})
	if _, ics = get(path); !strings.Contains(ics, "Ana Pérez") {
		t.Fatalf("names on: %s", ics)
	}
	// replacing the address cuts off the old one
	st = doc.expect(200, "POST", "/api/me/calendar", map[string]any{"action": "rotate"})
	if code, _ = get(path); code != 404 {
		t.Fatalf("old address: %d", code)
	}
	if code, _ = get(st["path"].(string)); code != 200 {
		t.Fatalf("new address: %d", code)
	}
	doc.expect(200, "POST", "/api/me/calendar", map[string]any{"action": "disable"})
	if code, _ = get(st["path"].(string)); code != 404 {
		t.Fatalf("off: %d", code)
	}
	if code, _ = get("/api/public/calendar/zzzz.ics"); code != 404 {
		t.Fatalf("unknown: %d", code)
	}
}
