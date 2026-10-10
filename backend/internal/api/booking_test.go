package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/madgrismad/miconsultorio/backend/internal/api"
	"github.com/madgrismad/miconsultorio/backend/internal/config"
)

// fakeWhatsApp records the Cloud API calls; status is what it answers.
type fakeWhatsApp struct {
	mu     sync.Mutex
	status int
	bodies []map[string]any
	auth   []string
}

func (f *fakeWhatsApp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	f.mu.Lock()
	f.bodies = append(f.bodies, body)
	f.auth = append(f.auth, r.URL.Path+"|"+r.Header.Get("Authorization"))
	st := f.status
	f.mu.Unlock()
	w.WriteHeader(st)
	_, _ = w.Write([]byte(`{"error":{"message":"boom"}}`))
}

func (f *fakeWhatsApp) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.bodies)
}

type bookingEnv struct {
	*env
	cfg   *config.Config
	wa    *fakeWhatsApp
	pro   string
	date  string // a weekday a few days ahead
	slugA string
}

func newBookingEnv(t *testing.T, withWA bool) *bookingEnv {
	t.Helper()
	b := &bookingEnv{wa: &fakeWhatsApp{status: 200}, slugA: "clinica-a"}
	waSrv := httptest.NewServer(b.wa)
	t.Cleanup(waSrv.Close)
	b.env = setupWith(t, func(c *config.Config) {
		b.cfg = c
		if withWA {
			c.WhatsAppToken, c.WhatsAppPhoneID, c.WhatsAppAPIBase = "tok-wa", "12345", waSrv.URL
			c.WhatsAppTemplate, c.WhatsAppLang = "recordatorio_cita", "es_MX"
		}
	})
	b.exec(`INSERT INTO agenda_settings (clinic_id, booking_enabled, booking_slug, booking_message, remind_whatsapp) VALUES ($1, true, $2, 'Bienvenido', $3)`,
		b.clinicA, b.slugA, withWA)
	b.exec(`INSERT INTO agenda_settings (clinic_id, booking_enabled, booking_slug) VALUES ($1, false, 'clinica-b')`, b.clinicB)
	b.pro = b.userID("doc_a")
	b.exec(`INSERT INTO professional_settings (user_id, clinic_id, bookable, slot_minutes) VALUES ($1, $2, true, 30)`, b.pro, b.clinicA)
	b.exec(`INSERT INTO professional_settings (user_id, clinic_id, bookable) VALUES ($1, $2, true)`, b.userID("doc_b"), b.clinicB)
	b.exec(`INSERT INTO catalog_items (clinic_id, kind, name, price_cents) VALUES ($1, 'service', 'Consulta general', 50000), ($1, 'product', 'Jarabe', 100)`, b.clinicA)
	loc, _ := time.LoadLocation("America/Mexico_City")
	d := time.Now().In(loc).AddDate(0, 0, 4)
	for d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
		d = d.AddDate(0, 0, 1)
	}
	b.date = d.Format("2006-01-02")
	return b
}

func (b *bookingEnv) book(c *client, start, email, phone string) (int, map[string]any) {
	return c.do("POST", "/api/public/booking/"+b.slugA+"/appointments", map[string]any{
		"professional_id": b.pro, "date": b.date, "start": start, "names": "Ana", "last_names": "López",
		"email": email, "phone": phone, "reason": "Dolor de muela", "accept_privacy": true, "accept_reminders": true,
	})
}

func (b *bookingEnv) token(m map[string]any) string { return sub(m, "appointment")["token"].(string) }

func TestPublicBookingInfoAndIsolation(t *testing.T) {
	b := newBookingEnv(t, false)
	anon := b.anon()
	info := sub(anon.expect(200, "GET", "/api/public/booking/"+b.slugA, nil), "booking")
	if sub(info, "clinic")["name"] != "Clinica a" || info["message"] != "Bienvenido" {
		t.Fatalf("info: %v", info)
	}
	pros, svcs := info["professionals"].([]any), info["services"].([]any)
	if len(pros) != 1 || pros[0].(map[string]any)["id"] != b.pro {
		t.Fatalf("professionals must be only clinic A's bookable ones: %v", pros)
	}
	// only services (the clinic's own plus the default "Consulta" every clinic starts with), never products
	names := map[string]bool{}
	for _, sv := range svcs {
		names[sv.(map[string]any)["name"].(string)] = true
		if _, has := sv.(map[string]any)["price_cents"]; has {
			t.Fatal("prices must stay hidden unless the clinic publishes them")
		}
	}
	if len(svcs) != 2 || !names["Consulta general"] || !names["Consulta"] {
		t.Fatalf("only services: %v", svcs)
	}
	b.exec(`UPDATE agenda_settings SET booking_show_prices = true WHERE clinic_id = $1`, b.clinicA)
	svcs = sub(anon.expect(200, "GET", "/api/public/booking/"+b.slugA, nil), "booking")["services"].([]any)
	if svcs[0].(map[string]any)["price_cents"] != float64(50000) {
		t.Fatalf("prices: %v", svcs)
	}
	anon.expect(404, "GET", "/api/public/booking/clinica-b", nil) // disabled
	anon.expect(404, "GET", "/api/public/booking/no-existe", nil)
	anon.expect(404, "GET", "/api/public/booking/..%2Fx", nil)
	// clinic B's professional can't be booked through A's page
	code, _ := anon.do("POST", "/api/public/booking/"+b.slugA+"/appointments", map[string]any{
		"professional_id": b.userID("doc_b"), "date": b.date, "start": "10:00", "names": "Ana", "last_names": "L",
		"email": "a@b.mx", "accept_privacy": true,
	})
	if code != 400 {
		t.Fatalf("foreign professional: %d", code)
	}
}

func TestBookingAvailabilityAndDoubleBooking(t *testing.T) {
	b := newBookingEnv(t, false)
	anon := b.anon()
	path := fmt.Sprintf("/api/public/booking/%s/availability?professional=%s&date=%s", b.slugA, b.pro, b.date)
	slots := anon.expect(200, "GET", path, nil)["slots"].([]any)
	if len(slots) != 18 { // 09:00-18:00 in 30 minutes
		t.Fatalf("expected 18 slots, got %d", len(slots))
	}
	// a block takes 09:00-10:00 away
	b.exec(`INSERT INTO time_blocks (clinic_id, professional_id, date_from, date_to, start_hour, end_hour) VALUES ($1, $2, $3, $3, '09:00', '10:00')`, b.clinicA, b.pro, b.date)
	slots = anon.expect(200, "GET", path, nil)["slots"].([]any)
	if len(slots) != 16 || slots[0].(map[string]any)["start"] != "10:00" {
		t.Fatalf("block not respected: %v", slots)
	}
	// past dates and dates beyond the horizon offer nothing
	past := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	if got := anon.expect(200, "GET", fmt.Sprintf("/api/public/booking/%s/availability?professional=%s&date=%s", b.slugA, b.pro, past), nil)["slots"].([]any); len(got) != 0 {
		t.Fatal("past date must be empty")
	}
	far := time.Now().AddDate(0, 0, 90).Format("2006-01-02")
	if got := anon.expect(200, "GET", fmt.Sprintf("/api/public/booking/%s/availability?professional=%s&date=%s", b.slugA, b.pro, far), nil)["slots"].([]any); len(got) != 0 {
		t.Fatal("beyond the horizon must be empty")
	}
	anon.expect(400, "GET", fmt.Sprintf("/api/public/booking/%s/availability?professional=x&date=%s", b.slugA, b.date), nil)

	// blocked and out-of-hours slots can't be booked
	if code, _ := b.book(anon, "09:30", "x@x.mx", ""); code != 409 {
		t.Fatalf("blocked slot: %d", code)
	}
	if code, _ := b.book(anon, "19:00", "x@x.mx", ""); code != 409 {
		t.Fatalf("out of hours: %d", code)
	}

	out := anon.expect(201, "POST", "/api/public/booking/"+b.slugA+"/appointments", map[string]any{
		"professional_id": b.pro, "date": b.date, "start": "10:00", "names": "Ana", "last_names": "López",
		"email": "ana@correo.mx", "phone": "55 1234 5678", "reason": "Dolor", "accept_privacy": true, "accept_reminders": true,
	})
	if sub(out, "appointment")["end"] != "10:30" {
		t.Fatalf("out: %v", out)
	}
	var src, status, phone string
	var consent bool
	if err := b.pool.QueryRow(context.Background(), `SELECT source, status, phone, reminders_consent FROM appointments WHERE confirm_token = $1`, b.token(out)).Scan(&src, &status, &phone, &consent); err != nil {
		t.Fatal(err)
	}
	if src != "online" || status != "scheduled" || phone != "+525512345678" || !consent {
		t.Fatalf("row: %s %s %s %v", src, status, phone, consent)
	}
	m := b.mail.wait(t, 1)
	if m.To[0] != "ana@correo.mx" || !strings.Contains(m.Text, "/cita/"+b.token(out)) {
		t.Fatalf("confirmation mail: %+v", m)
	}
	// the visit goes to the patient's calendar: an .ics attached and a Google Calendar link
	if len(m.Attachments) != 1 || m.Attachments[0].Name != "cita.ics" || !strings.Contains(m.HTML, "calendar.google.com/calendar/render") {
		t.Fatalf("calendar invitation: %+v", m.Attachments)
	}
	ics := string(m.Attachments[0].Data)
	for _, want := range []string{"BEGIN:VEVENT", "UID:cita-", "DTSTART:", "DTEND:", "SUMMARY:Cita en Clinica a", "STATUS:CONFIRMED", "BEGIN:VALARM"} {
		if !strings.Contains(ics, want) {
			t.Fatalf("ics lacks %q:\n%s", want, ics)
		}
	}
	var logged int
	_ = b.pool.QueryRow(context.Background(), `SELECT count(*) FROM activity_log WHERE type = 'appointment_booked_online' AND clinic_id = $1`, b.clinicA).Scan(&logged)
	if logged != 1 {
		t.Fatalf("audit rows: %d", logged)
	}
	if code, _ := b.book(anon, "10:00", "otro@x.mx", ""); code != 409 {
		t.Fatalf("taken slot: %d", code)
	}
	slots = anon.expect(200, "GET", path, nil)["slots"].([]any)
	if len(slots) != 15 {
		t.Fatalf("taken slot must disappear: %d", len(slots))
	}
}

func TestBookingValidation(t *testing.T) {
	b := newBookingEnv(t, false)
	anon := b.anon()
	post := func(mod func(m map[string]any)) int {
		m := map[string]any{"professional_id": b.pro, "date": b.date, "start": "11:00", "names": "Ana", "last_names": "López",
			"email": "ana@correo.mx", "accept_privacy": true}
		mod(m)
		code, _ := anon.do("POST", "/api/public/booking/"+b.slugA+"/appointments", m)
		return code
	}
	if post(func(m map[string]any) { m["accept_privacy"] = false }) != 400 {
		t.Fatal("privacy notice is required")
	}
	if post(func(m map[string]any) { m["email"], m["phone"] = "", "" }) != 400 {
		t.Fatal("a contact is required")
	}
	if post(func(m map[string]any) { m["email"] = "no-es-correo" }) != 400 {
		t.Fatal("email shape")
	}
	if post(func(m map[string]any) { m["phone"] = "123" }) != 400 {
		t.Fatal("phone shape")
	}
	if post(func(m map[string]any) { m["service_id"] = b.userID("doc_a") }) != 400 {
		t.Fatal("service must be a clinic service")
	}
	if post(func(m map[string]any) { m["unknown"] = 1 }) != 400 {
		t.Fatal("strict body")
	}
	// the honeypot answers like success but stores nothing
	if post(func(m map[string]any) { m["website"] = "http://spam" }) != 201 {
		t.Fatal("honeypot status")
	}
	var n int
	_ = b.pool.QueryRow(context.Background(), `SELECT count(*) FROM appointments`).Scan(&n)
	if n != 0 {
		t.Fatalf("nothing should be stored, got %d", n)
	}
}

func TestBookingRateLimitPerContact(t *testing.T) {
	b := newBookingEnv(t, false)
	anon := b.anon()
	for i, st := range []string{"10:00", "10:30", "11:00"} {
		b.exec(`UPDATE appointments SET status = 'cancelled'`) // one open visit per giro
		if code, out := b.book(anon, st, "mismo@x.mx", ""); code != 201 {
			t.Fatalf("booking %d: %d %v", i, code, out)
		}
	}
	if code, _ := b.book(anon, "11:30", "mismo@x.mx", ""); code != 429 {
		t.Fatalf("per-contact limit: %d", code)
	}
}

func TestConcurrentBookingsOfTheSameSlot(t *testing.T) {
	b := newBookingEnv(t, false)
	var wg sync.WaitGroup
	var mu sync.Mutex
	codes := map[int]int{}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			code, _ := b.book(b.anon(), "12:00", fmt.Sprintf("p%d@x.mx", i), "")
			mu.Lock()
			codes[code]++
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	if codes[201] != 1 || codes[409] != 7 {
		t.Fatalf("exactly one booking must win: %v", codes)
	}
	var n int
	_ = b.pool.QueryRow(context.Background(), `SELECT count(*) FROM appointments WHERE start_hour = '12:00'`).Scan(&n)
	if n != 1 {
		t.Fatalf("rows: %d", n)
	}
}

func TestAppointmentTokenLinks(t *testing.T) {
	b := newBookingEnv(t, false)
	anon := b.anon()
	_, out := b.book(anon, "10:00", "ana@correo.mx", "")
	tok := b.token(out)
	base := "/api/public/appointments/" + tok

	got := sub(anon.expect(200, "GET", base, nil), "appointment")
	if got["status"] != "scheduled" || got["can_confirm"] != true || got["can_cancel"] != true || got["patient"] != "Ana" || got["date"] != b.date {
		t.Fatalf("view: %v", got)
	}
	if _, leaks := got["email"]; leaks {
		t.Fatal("must not expose contact data")
	}
	anon.expect(404, "GET", "/api/public/appointments/"+strings.Repeat("a", 43), nil)
	anon.expect(404, "GET", "/api/public/appointments/corto", nil)
	anon.expect(404, "POST", "/api/public/appointments/"+strings.Repeat("a", 43)+"/cancel", nil)

	if sub(anon.expect(200, "POST", base+"/confirm", nil), "appointment")["status"] != "confirmed" {
		t.Fatal("confirm")
	}
	anon.expect(200, "POST", base+"/confirm", nil) // idempotent

	// too late to cancel: the visit is within the notice window
	b.exec(`UPDATE agenda_settings SET cancel_min_hours = 24 * 30 WHERE clinic_id = $1`, b.clinicA)
	if code, out := anon.do("POST", base+"/cancel", map[string]any{"reason": "x"}); code != 409 || out["code"] != "NOT_ALLOWED" {
		t.Fatalf("late cancel: %d %v", code, out)
	}
	b.exec(`UPDATE agenda_settings SET cancel_min_hours = 2 WHERE clinic_id = $1`, b.clinicA)
	if sub(anon.expect(200, "POST", base+"/cancel", map[string]any{"reason": "Me surgió algo"}), "appointment")["status"] != "cancelled" {
		t.Fatal("cancel")
	}
	var reason string
	_ = b.pool.QueryRow(context.Background(), `SELECT cancel_reason FROM appointments WHERE confirm_token = $1`, tok).Scan(&reason)
	if !strings.Contains(reason, "Me surgió algo") {
		t.Fatalf("reason: %q", reason)
	}
	anon.expect(409, "POST", base+"/confirm", nil) // a cancelled visit can't be confirmed
	// the slot is free again
	if code, _ := b.book(anon, "10:00", "otra@x.mx", ""); code != 201 {
		t.Fatalf("slot after cancel: %d", code)
	}
}

func TestTokenProbingIsLimited(t *testing.T) {
	b := newBookingEnv(t, false)
	anon := b.anon()
	limited := false
	for i := 0; i < 40 && !limited; i++ {
		code, _ := anon.do("GET", "/api/public/appointments/"+fmt.Sprintf("%043d", i), nil)
		limited = code == 429
	}
	if !limited {
		t.Fatal("guessing tokens must be rate limited")
	}
}

func (b *bookingEnv) reminderRows(tok string) (rows []string) {
	q, err := b.pool.Query(context.Background(), `SELECT r.channel || ':' || r.hours_before || ':' || r.status || ':' || r.attempts
		FROM appointment_reminders r JOIN appointments a ON a.id = r.appointment_id WHERE a.confirm_token = $1 ORDER BY r.channel, r.hours_before DESC`, tok)
	if err != nil {
		b.t.Fatal(err)
	}
	defer q.Close()
	for q.Next() {
		var s string
		_ = q.Scan(&s)
		rows = append(rows, s)
	}
	return rows
}

func (b *bookingEnv) makeDue(tok string) {
	b.exec(`UPDATE appointment_reminders SET send_at = now() - interval '1 minute' WHERE appointment_id = (SELECT id FROM appointments WHERE confirm_token = $1) AND status = 'pending'`, tok)
}

func (b *bookingEnv) tick() int {
	b.t.Helper()
	n, err := api.RunRemindersOnce(context.Background(), b.pool, b.cfg, b.mail)
	if err != nil {
		b.t.Fatal(err)
	}
	return n
}

func TestRemindersScheduleAndSend(t *testing.T) {
	b := newBookingEnv(t, true)
	anon := b.anon()
	_, out := b.book(anon, "10:00", "ana@correo.mx", "5512345678")
	tok := b.token(out)
	rows := b.reminderRows(tok)
	if strings.Join(rows, ",") != "email:24:pending:0,email:2:pending:0,whatsapp:24:pending:0,whatsapp:2:pending:0" {
		t.Fatalf("scheduled: %v", rows)
	}
	b.mail.wait(t, 1) // booking confirmation
	if b.tick() != 0 {
		t.Fatal("nothing is due yet")
	}
	b.makeDue(tok)
	if n := b.tick(); n != 4 {
		t.Fatalf("processed %d", n)
	}
	for _, r := range b.reminderRows(tok) {
		if !strings.Contains(r, ":sent:1") {
			t.Fatalf("not sent: %v", b.reminderRows(tok))
		}
	}
	m := b.mail.wait(t, 3)
	if m.To[0] != "ana@correo.mx" || !strings.Contains(m.Text, "/cita/"+tok+"?accion=confirmar") || !strings.Contains(m.Text, "baja=1") || !strings.Contains(m.HTML, "No podré asistir") || !strings.Contains(m.HTML, "Si no respondes, tu cita sigue agendada") || !strings.Contains(m.Text, "accion=cancelar") {
		t.Fatalf("reminder mail: %+v", m)
	}
	if len(m.Attachments) != 1 || !strings.Contains(string(m.Attachments[0].Data), "SEQUENCE:") {
		t.Fatalf("the reminder carries the same calendar event: %+v", m.Attachments)
	}
	if b.wa.count() != 2 {
		t.Fatalf("whatsapp calls: %d", b.wa.count())
	}
	call := b.wa.bodies[0]
	if call["to"] != "525512345678" || call["type"] != "template" || b.wa.auth[0] != "/12345/messages|Bearer tok-wa" {
		t.Fatalf("whatsapp call: %v %v", call, b.wa.auth[0])
	}
	params := call["template"].(map[string]any)["components"].([]any)[0].(map[string]any)["parameters"].([]any)
	if len(params) != 4 || params[0].(map[string]any)["text"] != "Ana" || params[2].(map[string]any)["text"] != "10:00" || params[3].(map[string]any)["text"] != "Clinica a" {
		t.Fatalf("template params: %v", params)
	}
	if b.tick() != 0 {
		t.Fatal("sent reminders must not repeat")
	}
	// the patient never answered: the appointment is still booked (nothing cancels it on its own)
	var status string
	if err := b.pool.QueryRow(t.Context(), `SELECT status FROM appointments WHERE confirm_token = $1`, tok).Scan(&status); err != nil || status != "scheduled" {
		t.Fatalf("an unanswered reminder must leave the appointment booked: %q %v", status, err)
	}
}

func TestReminderRetriesAndFailure(t *testing.T) {
	b := newBookingEnv(t, true)
	b.exec(`UPDATE agenda_settings SET remind_email = false WHERE clinic_id = $1`, b.clinicA)
	_, out := b.book(b.anon(), "10:00", "", "5512345678")
	tok := b.token(out)
	b.wa.mu.Lock()
	b.wa.status = 500
	b.wa.mu.Unlock()
	b.makeDue(tok)
	b.tick()
	rows := strings.Join(b.reminderRows(tok), ",")
	if rows != "whatsapp:24:pending:1,whatsapp:2:pending:1" {
		t.Fatalf("after first failure: %s", rows)
	}
	var sendAt time.Time
	_ = b.pool.QueryRow(context.Background(), `SELECT min(send_at) FROM appointment_reminders`).Scan(&sendAt)
	if sendAt.Before(time.Now().Add(3 * time.Minute)) {
		t.Fatalf("backoff not applied: %v", sendAt)
	}
	b.tick() // not due: backoff
	if b.wa.count() != 2 {
		t.Fatalf("retried too early: %d", b.wa.count())
	}
	b.makeDue(tok)
	b.tick()
	b.makeDue(tok)
	b.tick()
	rows = strings.Join(b.reminderRows(tok), ",")
	if rows != "whatsapp:24:failed:3,whatsapp:2:failed:3" {
		t.Fatalf("after 3 attempts: %s", rows)
	}
	var msg string
	_ = b.pool.QueryRow(context.Background(), `SELECT error FROM appointment_reminders LIMIT 1`).Scan(&msg)
	if !strings.Contains(msg, "boom") {
		t.Fatalf("error not recorded: %q", msg)
	}

	// a 4xx is permanent: no retries
	b.exec(`UPDATE appointment_reminders SET status = 'pending', attempts = 0, error = ''`)
	b.wa.mu.Lock()
	b.wa.status = 400
	b.wa.mu.Unlock()
	b.makeDue(tok)
	b.tick()
	if rows = strings.Join(b.reminderRows(tok), ","); rows != "whatsapp:24:failed:1,whatsapp:2:failed:1" {
		t.Fatalf("permanent failure: %s", rows)
	}
}

func TestReminderChannelNotConfiguredIsSkipped(t *testing.T) {
	b := newBookingEnv(t, false)
	b.exec(`UPDATE agenda_settings SET remind_whatsapp = true WHERE clinic_id = $1`, b.clinicA)
	_, out := b.book(b.anon(), "10:00", "", "5512345678")
	tok := b.token(out)
	b.makeDue(tok)
	b.tick()
	var status, msg string
	_ = b.pool.QueryRow(context.Background(), `SELECT status, error FROM appointment_reminders LIMIT 1`).Scan(&status, &msg)
	if status != "skipped" || !strings.Contains(msg, "WhatsApp no está configurado") {
		t.Fatalf("%s %q", status, msg)
	}
}

func TestRemindersRespectConsentAndStatus(t *testing.T) {
	b := newBookingEnv(t, false)
	anon := b.anon()
	// no consent -> nothing queued
	code, out := anon.do("POST", "/api/public/booking/"+b.slugA+"/appointments", map[string]any{
		"professional_id": b.pro, "date": b.date, "start": "09:00", "names": "Ana", "last_names": "López",
		"email": "sin@x.mx", "accept_privacy": true, "accept_reminders": false,
	})
	if code != 201 || len(b.reminderRows(b.token(out))) != 0 {
		t.Fatalf("no consent: %d %v", code, b.reminderRows(b.token(out)))
	}

	_, out = b.book(anon, "10:00", "ana@correo.mx", "")
	tok := b.token(out)
	if len(b.reminderRows(tok)) != 2 {
		t.Fatalf("rows: %v", b.reminderRows(tok))
	}
	b.mail.wait(t, 2)
	// confirming keeps them; cancelling drops them
	anon.expect(200, "POST", "/api/public/appointments/"+tok+"/confirm", nil)
	if len(b.reminderRows(tok)) != 2 {
		t.Fatal("confirm keeps reminders")
	}
	// a visit cancelled by staff (SQL) is not reminded even if a row is still pending
	b.makeDue(tok)
	b.exec(`UPDATE appointments SET status = 'cancelled' WHERE confirm_token = $1`, tok)
	before := b.mail.count()
	b.tick()
	if b.mail.count() != before {
		t.Fatal("reminder sent for a cancelled visit")
	}
	if rows := strings.Join(b.reminderRows(tok), ","); rows != "email:24:skipped:0,email:2:skipped:0" {
		t.Fatalf("rows: %s", rows)
	}

	// opt-out: no consent anywhere, pending reminders deleted, other upcoming visits of the contact too
	_, o1 := b.book(anon, "13:00", "baja@x.mx", "")
	t1 := b.token(o1)
	b.exec(`UPDATE appointments SET professional_id = NULL WHERE confirm_token = $1`, t1) // no giro: does not count as open in the same one
	_, o2 := b.book(anon, "13:30", "baja@x.mx", "")
	b.exec(`UPDATE appointments SET professional_id = $2 WHERE confirm_token = $1`, t1, b.pro)
	t2 := b.token(o2)
	anon.expect(200, "POST", "/api/public/appointments/"+t1+"/optout", nil)
	if len(b.reminderRows(t1)) != 0 || len(b.reminderRows(t2)) != 0 {
		t.Fatalf("opt-out must drop reminders: %v %v", b.reminderRows(t1), b.reminderRows(t2))
	}
	if sub(anon.expect(200, "GET", "/api/public/appointments/"+t2, nil), "appointment")["reminders"] != false {
		t.Fatal("reminders flag")
	}
}

func TestOptoutTurnsOffPatientConsent(t *testing.T) {
	b := newBookingEnv(t, false)
	var pid string
	if err := b.pool.QueryRow(context.Background(), `INSERT INTO patients (clinic_id, file_number, names, email, reminders_ok) VALUES ($1, 1, 'Luis', 'luis@x.mx', true) RETURNING id`, b.clinicA).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	tok := strings.Repeat("t", 43)
	b.exec(`INSERT INTO appointments (clinic_id, patient_id, curp, names, last_names, date, start_hour, end_hour, professional_id, confirm_token)
	        VALUES ($1, $2, '', 'Luis', '', $3, '10:00', '10:30', $4, $5)`, b.clinicA, pid, b.date, b.pro, tok)
	anon := b.anon()
	anon.expect(200, "POST", "/api/public/appointments/"+tok+"/optout", nil)
	var ok bool
	_ = b.pool.QueryRow(context.Background(), `SELECT reminders_ok FROM patients WHERE id = $1`, pid).Scan(&ok)
	if ok {
		t.Fatal("patient consent must be off")
	}
}

func TestRemindersUseThePatientsConsentAndContact(t *testing.T) {
	b := newBookingEnv(t, false)
	var pid string
	if err := b.pool.QueryRow(context.Background(), `INSERT INTO patients (clinic_id, file_number, names, email, reminders_ok) VALUES ($1, 1, 'Luis', 'luis@x.mx', true) RETURNING id`, b.clinicA).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	var apptID string
	if err := b.pool.QueryRow(context.Background(), `INSERT INTO appointments (clinic_id, patient_id, curp, names, last_names, date, start_hour, end_hour, professional_id)
	        VALUES ($1, $2, '', 'Luis', '', $3, '10:00', '10:30', $4) RETURNING id`, b.clinicA, pid, b.date, b.pro).Scan(&apptID); err != nil {
		t.Fatal(err)
	}
	// the staff-side code calls this after saving an appointment; do the same through the exported hook
	if err := api.ScheduleRemindersFor(context.Background(), b.pool, b.cfg, b.mail, b.clinicA, apptID); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = b.pool.QueryRow(context.Background(), `SELECT count(*) FROM appointment_reminders WHERE appointment_id = $1 AND channel = 'email'`, apptID).Scan(&n)
	if n != 2 {
		t.Fatalf("patient e-mail reminders: %d", n)
	}
	// another clinic's id never schedules anything for this appointment
	b.exec(`DELETE FROM appointment_reminders`)
	if err := api.ScheduleRemindersFor(context.Background(), b.pool, b.cfg, b.mail, b.clinicB, apptID); err != nil {
		t.Fatal(err)
	}
	_ = b.pool.QueryRow(context.Background(), `SELECT count(*) FROM appointment_reminders`).Scan(&n)
	if n != 0 {
		t.Fatal("clinic isolation")
	}
	// a late reminder is only created when the visit is more than 30 minutes away
	loc, _ := time.LoadLocation("America/Mexico_City")
	soon := time.Now().In(loc).Add(time.Hour)
	if soon.Hour() == 23 { // a visit starting at 23:xx cannot end at 23:59 in a valid span: go past midnight instead
		soon = soon.Add(90 * time.Minute)
	}
	b.exec(`UPDATE appointments SET date = $2, start_hour = $3, end_hour = '23:59' WHERE id = $1`, apptID, soon.Format("2006-01-02"), soon.Format("15:04"))
	if err := api.ScheduleRemindersFor(context.Background(), b.pool, b.cfg, b.mail, b.clinicA, apptID); err != nil {
		t.Fatal(err)
	}
	_ = b.pool.QueryRow(context.Background(), `SELECT count(*) FROM appointment_reminders WHERE appointment_id = $1`, apptID).Scan(&n)
	if n != 1 {
		t.Fatalf("late reminder: %d", n)
	}
	b.exec(`DELETE FROM appointment_reminders`)
	soon = time.Now().In(loc).Add(20 * time.Minute)
	b.exec(`UPDATE appointments SET date = $2, start_hour = $3 WHERE id = $1`, apptID, soon.Format("2006-01-02"), soon.Format("15:04"))
	_ = api.ScheduleRemindersFor(context.Background(), b.pool, b.cfg, b.mail, b.clinicA, apptID)
	_ = b.pool.QueryRow(context.Background(), `SELECT count(*) FROM appointment_reminders WHERE appointment_id = $1`, apptID).Scan(&n)
	if n != 0 {
		t.Fatalf("too late: %d", n)
	}
}

func TestStaffBookingMailsThePatientRightAway(t *testing.T) {
	b := newBookingEnv(t, false)
	recep := b.login("recep_a")
	body := func(email string, consent bool, hour string) map[string]any {
		return map[string]any{"names": "Ana", "last_names": "López", "date": b.date, "startHour": hour, "endHour": hour[:3] + "30", "email": email, "reminders_consent": consent, "professional_id": b.pro}
	}
	recep.expect(201, "POST", "/api/appointments", body("ana@correo.mx", true, "10:00"))
	m := b.mail.wait(t, 1)
	if m.To[0] != "ana@correo.mx" || !strings.Contains(m.Subject, "Tu cita quedó agendada") || !strings.Contains(m.Text, "/cita/") {
		t.Fatalf("booking mail: %+v", m)
	}
	// without consent, or without an e-mail, nothing is sent
	recep.expect(201, "POST", "/api/appointments", body("otra@correo.mx", false, "11:00"))
	recep.expect(201, "POST", "/api/appointments", body("", true, "12:00"))
	time.Sleep(300 * time.Millisecond)
	if n := b.mail.count(); n != 1 {
		t.Fatalf("only the consenting patient with an e-mail is mailed: %d", n)
	}
}

func TestBookingCheckPointsToWhatIsMissing(t *testing.T) {
	b := newBookingEnv(t, false)
	admin := b.login("admin_a")
	problem := func(slug string) map[string]any {
		return admin.expect(200, "GET", "/api/agenda/booking-check?slug="+slug, nil)
	}
	if p := problem(b.slugA)["problem"]; p != "" {
		t.Fatalf("everything is set: %v", p)
	}
	if r := problem("clinica-aa"); r["problem"] != "slug_mismatch" || r["to"] != "/"+b.slugA+"/reservar" {
		t.Fatalf("mismatch: %v", r)
	}
	b.exec(`UPDATE professional_settings SET bookable = false WHERE clinic_id = $1`, b.clinicA)
	if r := problem(b.slugA); r["problem"] != "no_bookable" || r["to"] != "/ajustes?s=agenda" {
		t.Fatalf("no bookable: %v", r)
	}
	b.exec(`UPDATE agenda_settings SET booking_enabled = false WHERE clinic_id = $1`, b.clinicA)
	if r := problem(b.slugA); r["problem"] != "disabled" || r["to"] != "/ajustes?s=agenda" {
		t.Fatalf("disabled: %v", r)
	}
	b.anon().expect(401, "GET", "/api/agenda/booking-check?slug=x", nil)
}
