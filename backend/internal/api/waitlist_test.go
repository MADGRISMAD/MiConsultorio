package api_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/madgrismad/miconsultorio/backend/internal/api"
)

type wlEnv struct {
	*bookingEnv
	loc *time.Location
}

func newWLEnv(t *testing.T) *wlEnv {
	t.Helper()
	b := newBookingEnv(t, false)
	loc, _ := time.LoadLocation("America/Mexico_City")
	return &wlEnv{bookingEnv: b, loc: loc}
}

func (w *wlEnv) weekday() int {
	d, _ := time.Parse("2006-01-02", w.date)
	return int(d.Weekday())
}

// onlyDay blocks the whole clinic on every day but w.date, so the waitlist has exactly one day to offer.
func (w *wlEnv) onlyDay() {
	today := time.Now().In(w.loc).Format("2006-01-02")
	w.exec(`INSERT INTO time_blocks (clinic_id, date_from, date_to) VALUES ($1, $2::date, $3::date - 1), ($1, $3::date + 1, $3::date + 120)`, w.clinicA, today, w.date)
}

func (w *wlEnv) scan() int {
	w.t.Helper()
	n, err := api.RunWaitlistOnce(context.Background(), w.pool, w.cfg, w.mail)
	if err != nil {
		w.t.Fatal(err)
	}
	return n
}

func (w *wlEnv) appt(c *client, start, end string) string {
	w.t.Helper()
	out := c.expect(201, "POST", "/api/appointments", map[string]any{
		"names": "Pac", "last_names": "Uno", "date": w.date, "startHour": start, "endHour": end, "professional_id": w.pro,
	})
	return sub(out, "appointment")["id"].(string)
}

func (w *wlEnv) entry(c *client, name, email string, consent bool, from, to string) string {
	w.t.Helper()
	body := map[string]any{"name": name, "email": email, "consent": consent, "professional_id": w.pro, "days": []int{w.weekday()}}
	if from != "" {
		body["from_time"], body["to_time"] = from, to
	}
	out := c.expect(201, "POST", "/api/waitlist", body)
	return sub(out, "entry")["id"].(string)
}

func (w *wlEnv) token(entryID string) string {
	w.t.Helper()
	var tok string
	if err := w.pool.QueryRow(context.Background(), `SELECT token FROM waitlist_entries WHERE id = $1`, entryID).Scan(&tok); err != nil {
		w.t.Fatal(err)
	}
	return tok
}

func (w *wlEnv) entryStatus(entryID string) string {
	w.t.Helper()
	var s string
	if err := w.pool.QueryRow(context.Background(), `SELECT status FROM waitlist_entries WHERE id = $1`, entryID).Scan(&s); err != nil {
		w.t.Fatal(err)
	}
	return s
}

func (w *wlEnv) count(sql string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		w.t.Fatal(err)
	}
	return n
}

func (w *wlEnv) service() string {
	w.t.Helper()
	var id string
	if err := w.pool.QueryRow(context.Background(), `SELECT id FROM catalog_items WHERE clinic_id = $1 AND kind = 'service' AND system_key IS NULL LIMIT 1`, w.clinicA).Scan(&id); err != nil {
		w.t.Fatal(err)
	}
	return id
}

func (w *wlEnv) availability(c *client, date, service string) []any {
	w.t.Helper()
	path := fmt.Sprintf("/api/public/booking/%s/availability?professional=%s&date=%s", w.slugA, w.pro, date)
	if service != "" {
		path += "&service=" + service
	}
	return c.expect(200, "GET", path, nil)["slots"].([]any)
}

// ---------------------------------------------------------------------------
// Duration per service
// ---------------------------------------------------------------------------

func TestServiceDuration(t *testing.T) {
	w := newWLEnv(t)
	svc := w.service()
	admin, recep, other := w.login("admin_a"), w.login("recep_a"), w.login("admin_b")
	path := "/api/agenda/services/" + svc + "/duration"

	recep.expect(403, "PUT", path, map[string]any{"duration_minutes": 60}) // adminUsers only
	w.anon().expect(401, "PUT", path, map[string]any{"duration_minutes": 60})
	other.expect(404, "PUT", path, map[string]any{"duration_minutes": 60}) // another clinic's service
	admin.expect(400, "PUT", path, map[string]any{"duration_minutes": 3})
	admin.expect(400, "PUT", path, map[string]any{"duration_minutes": 481})
	admin.expect(404, "PUT", "/api/agenda/services/not-a-uuid/duration", map[string]any{"duration_minutes": 30})
	var product string
	_ = w.pool.QueryRow(context.Background(), `SELECT id FROM catalog_items WHERE clinic_id = $1 AND kind = 'product'`, w.clinicA).Scan(&product)
	admin.expect(404, "PUT", "/api/agenda/services/"+product+"/duration", map[string]any{"duration_minutes": 30}) // products have none
	admin.expect(200, "PUT", path, map[string]any{"duration_minutes": 60})

	list := recep.expect(200, "GET", "/api/agenda/services", nil)["services"].([]any)
	found := false
	for _, it := range list { // the clinic's own service and the default "Consulta"
		if m := it.(map[string]any); m["name"] == "Consulta general" && m["duration_minutes"] == float64(60) {
			found = true
		}
	}
	if len(list) != 2 || !found {
		t.Fatalf("agenda services: %v", list)
	}
	if other.expect(200, "GET", "/api/agenda/services", nil)["services"] == nil {
		t.Fatal("services must be a list")
	}

	// Online: the slot is as long as the service.
	anon := w.anon()
	plain, long := w.availability(anon, w.date, ""), w.availability(anon, w.date, svc)
	if len(plain) != 18 || len(long) != 17 { // 09:00-18:00 on a 30-minute grid; a 60-minute service cannot start at 17:30
		t.Fatalf("slots: %d plain, %d with the service", len(plain), len(long))
	}
	first := long[0].(map[string]any)
	if first["start"] != "09:00" || first["end"] != "10:00" {
		t.Fatalf("first slot with the service: %v", first)
	}
	body := func(start string) map[string]any {
		return map[string]any{"professional_id": w.pro, "service_id": svc, "date": w.date, "start": start, "names": "Ana", "last_names": "López",
			"email": "ana@correo.mx", "accept_privacy": true}
	}
	code, _ := anon.do("POST", "/api/public/booking/"+w.slugA+"/appointments", body("17:30"))
	if code != 409 {
		t.Fatalf("a 60-minute service must not fit at 17:30: %d", code)
	}
	out := anon.expect(201, "POST", "/api/public/booking/"+w.slugA+"/appointments", body("10:00"))
	if sub(out, "appointment")["end"] != "11:00" {
		t.Fatalf("booked end: %v", out)
	}
	// 09:30 would end at 10:30, over the booked hour; 11:00 is free again.
	for _, s := range w.availability(anon, w.date, svc) {
		if st := s.(map[string]any)["start"]; st == "09:30" || st == "10:00" || st == "10:30" {
			t.Fatalf("slot %v overlaps the booked hour", st)
		}
	}
	if got := w.availability(anon, w.date, ""); len(got) != 16 { // 18 - 10:00 and 10:30
		t.Fatalf("without a service only the booked half hours go: %d", len(got))
	}
	// A service without duration keeps the professional's slot.
	admin.expect(200, "PUT", path, map[string]any{"duration_minutes": nil})
	if got := w.availability(anon, w.date, svc); len(got) != 16 {
		t.Fatalf("cleared duration: %d", len(got))
	}

	// Internal appointments: the end comes from the service when the form leaves it out.
	admin.expect(200, "PUT", path, map[string]any{"duration_minutes": 45})
	made := recep.expect(201, "POST", "/api/appointments", map[string]any{
		"names": "Pac", "last_names": "Dos", "date": w.date, "startHour": "14:00", "professional_id": w.pro, "service_id": svc,
	})
	if sub(made, "appointment")["endHour"] != "14:45" {
		t.Fatalf("internal appointment end: %v", made)
	}
	made = recep.expect(201, "POST", "/api/appointments", map[string]any{
		"names": "Pac", "last_names": "Tres", "date": w.date, "startHour": "16:00", "professional_id": w.pro,
	})
	if sub(made, "appointment")["endHour"] != "16:30" { // the professional's slot
		t.Fatalf("no service: %v", made)
	}
	made = recep.expect(201, "POST", "/api/appointments", map[string]any{"names": "Pac", "last_names": "Cuatro", "date": w.date, "startHour": "15:00"})
	if sub(made, "appointment")["endHour"] != "15:30" { // neither: the clinic's slot
		t.Fatalf("no professional: %v", made)
	}

	// The catalog form carries it too, and editing without the field keeps it.
	item := admin.expect(200, "PUT", "/api/pos/items/"+svc, map[string]any{
		"kind": "service", "name": "Consulta general", "price_cents": 50000, "unit": "pza", "duration_minutes": 20,
	})
	if sub(item, "item")["duration_minutes"] != float64(20) {
		t.Fatalf("catalog item: %v", item)
	}
	item = admin.expect(200, "PUT", "/api/pos/items/"+svc, map[string]any{"kind": "service", "name": "Consulta general", "price_cents": 50000, "unit": "pza"})
	if sub(item, "item")["duration_minutes"] != float64(20) {
		t.Fatalf("an edit that does not send the duration must keep it: %v", item)
	}
	admin.expect(400, "PUT", "/api/pos/items/"+svc, map[string]any{"kind": "service", "name": "Consulta general", "price_cents": 50000, "unit": "pza", "duration_minutes": 2})
	item = admin.expect(200, "PUT", "/api/pos/items/"+svc, map[string]any{"kind": "service", "name": "Consulta general", "price_cents": 50000, "unit": "pza", "duration_minutes": nil})
	if sub(item, "item")["duration_minutes"] != nil {
		t.Fatalf("null clears it: %v", item)
	}
	info := sub(anon.expect(200, "GET", "/api/public/booking/"+w.slugA, nil), "booking")
	if _, has := info["services"].([]any)[0].(map[string]any)["duration_minutes"]; has {
		t.Fatal("no duration to publish")
	}
}

// ---------------------------------------------------------------------------
// Monthly availability and load
// ---------------------------------------------------------------------------

func monthOf(date string) string { return date[:7] }

func (w *wlEnv) monthDays(c *client, service string) map[string]int {
	w.t.Helper()
	path := fmt.Sprintf("/api/public/booking/%s/month?professional=%s&month=%s", w.slugA, w.pro, monthOf(w.date))
	if service != "" {
		path += "&service=" + service
	}
	out := map[string]int{}
	for _, d := range c.expect(200, "GET", path, nil)["days"].([]any) {
		m := d.(map[string]any)
		out[m["date"].(string)] = int(m["slots"].(float64))
	}
	return out
}

// expectedDays asks availability day by day, the slow way, for comparison.
func (w *wlEnv) expectedDays(c *client, service string) map[string]int {
	w.t.Helper()
	first, _ := time.Parse("2006-01-02", monthOf(w.date)+"-01")
	out := map[string]int{}
	for d := first; d.Month() == first.Month(); d = d.AddDate(0, 0, 1) {
		if n := len(w.availability(c, d.Format("2006-01-02"), service)); n > 0 {
			out[d.Format("2006-01-02")] = n
		}
	}
	return out
}

func sameDays(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestMonthAvailability(t *testing.T) {
	w := newWLEnv(t)
	anon, recep, admin := w.anon(), w.login("recep_a"), w.login("admin_a")
	svc := w.service()

	got := w.monthDays(anon, "")
	if len(got) == 0 || got[w.date] != 18 {
		t.Fatalf("month: %v", got)
	}
	if want := w.expectedDays(anon, ""); !sameDays(got, want) {
		t.Fatalf("month must equal the union of the daily availability:\n got %v\nwant %v", got, want)
	}
	for d := range got {
		if wd, _ := time.Parse("2006-01-02", d); wd.Weekday() == time.Saturday || wd.Weekday() == time.Sunday {
			t.Fatalf("weekend offered: %s", d)
		}
	}

	// Appointments, a partial block and a whole-day block.
	w.appt(recep, "09:00", "10:00")
	w.exec(`INSERT INTO time_blocks (clinic_id, professional_id, date_from, date_to, start_hour, end_hour) VALUES ($1, $2, $3, $3, '12:00', '13:00')`, w.clinicA, w.pro, w.date)
	got = w.monthDays(anon, "")
	if got[w.date] != 18-2-2 {
		t.Fatalf("slots on the busy day: %d", got[w.date])
	}
	if want := w.expectedDays(anon, ""); !sameDays(got, want) {
		t.Fatalf("with load: got %v want %v", got, want)
	}

	// A whole-day appointment removes the day only for a service that needs long slots.
	admin.expect(200, "PUT", "/api/agenda/services/"+svc+"/duration", map[string]any{"duration_minutes": 480})
	w.appt(recep, "13:00", "17:00")
	withSvc := w.monthDays(anon, svc)
	if _, ok := withSvc[w.date]; ok {
		t.Fatalf("an 8-hour service cannot fit on %s: %v", w.date, withSvc)
	}
	if want := w.expectedDays(anon, svc); !sameDays(withSvc, want) {
		t.Fatalf("with service: got %v want %v", withSvc, want)
	}
	if _, ok := w.monthDays(anon, "")[w.date]; !ok {
		t.Fatal("the day still has short slots")
	}

	// A clinic-wide block empties the day; a cancelled appointment gives its slots back.
	w.exec(`INSERT INTO time_blocks (clinic_id, date_from, date_to) VALUES ($1, $2, $2)`, w.clinicA, w.date)
	if _, ok := w.monthDays(anon, "")[w.date]; ok {
		t.Fatal("blocked day offered")
	}
	if len(w.availability(anon, w.date, "")) != 0 {
		t.Fatal("blocked day must have no slots")
	}

	// Validation and isolation.
	base := "/api/public/booking/" + w.slugA + "/month"
	anon.expect(400, "GET", base+"?professional="+w.pro+"&month=2026-13", nil)
	anon.expect(400, "GET", base+"?professional="+w.pro, nil)
	anon.expect(400, "GET", base+"?professional=zzz&month="+monthOf(w.date), nil)
	anon.expect(400, "GET", base+"?professional="+w.pro+"&month="+monthOf(w.date)+"&service=nope", nil)
	anon.expect(404, "GET", "/api/public/booking/clinica-b/month?month="+monthOf(w.date), nil) // disabled
	anon.expect(404, "GET", "/api/public/booking/no-existe/month?month="+monthOf(w.date), nil)
	foreign := anon.expect(200, "GET", base+"?professional="+w.userID("doc_b")+"&month="+monthOf(w.date), nil)
	if len(foreign["days"].([]any)) != 0 {
		t.Fatalf("another clinic's professional has no days here: %v", foreign)
	}
	// Without a professional: any bookable one.
	anyPro := anon.expect(200, "GET", base+"?month="+monthOf(w.date), nil)
	if len(anyPro["days"].([]any)) == 0 {
		t.Fatal("any professional")
	}
}

func TestAgendaMonthLoad(t *testing.T) {
	w := newWLEnv(t)
	recep, doc, cash, other := w.login("recep_a"), w.login("doc_a"), w.login("cash_a"), w.login("admin_b")
	w.appt(recep, "10:00", "10:30")
	w.appt(recep, "11:00", "12:00")
	cancelled := w.appt(recep, "15:00", "16:00")
	recep.expect(200, "POST", "/api/appointments/"+cancelled+"/status", map[string]any{"status": "cancelled"})

	path := "/api/agenda/month-load?month=" + monthOf(w.date)
	days := func(c *client) map[string]map[string]any {
		out := map[string]map[string]any{}
		for _, d := range c.expect(200, "GET", path, nil)["days"].([]any) {
			m := d.(map[string]any)
			out[m["date"].(string)] = m
		}
		return out
	}
	d := days(recep)[w.date]
	if d["appointments"] != float64(2) || d["booked_minutes"] != float64(90) || d["capacity_minutes"] != float64(540) {
		t.Fatalf("day load: %v", d)
	}
	if l := d["load"].(float64); l < 0.16 || l > 0.17 {
		t.Fatalf("load: %v", l)
	}
	if d["blocked"] != false {
		t.Fatal("not blocked")
	}
	for _, c := range []*client{doc, cash} {
		c.expect(200, "GET", path, nil)
	}
	if n := len(days(other)); n == 0 || days(other)[w.date]["appointments"] != float64(0) {
		t.Fatalf("clinic B must not see clinic A's appointments: %v", days(other)[w.date])
	}
	w.exec(`INSERT INTO time_blocks (clinic_id, date_from, date_to) VALUES ($1, $2, $2)`, w.clinicA, w.date)
	d = days(recep)[w.date]
	if d["blocked"] != true || d["capacity_minutes"] != float64(0) {
		t.Fatalf("blocked day: %v", d)
	}
	recep.expect(400, "GET", "/api/agenda/month-load?month=hoy", nil)
	recep.expect(400, "GET", path+"&professional=x", nil)
	w.anon().expect(401, "GET", path, nil)
	w.login("root").expect(403, "GET", path, nil)
}

// ---------------------------------------------------------------------------
// Waitlist: staff side
// ---------------------------------------------------------------------------

func TestWaitlistStaffCRUDAndPermissions(t *testing.T) {
	w := newWLEnv(t)
	recep, doc, cash, other := w.login("recep_a"), w.login("doc_a"), w.login("cash_a"), w.login("admin_b")
	good := map[string]any{"name": "Laura Pérez", "phone": "55 1234 5678", "email": "Laura@Correo.mx", "consent": true,
		"professional_id": w.pro, "days": []int{3, 1, 1}, "from_time": "10:00", "to_time": "12:00", "notes": "Prefiere tarde"}
	out := recep.expect(201, "POST", "/api/waitlist", good)
	e := sub(out, "entry")
	id := e["id"].(string)
	if e["status"] != "waiting" || e["email"] != "laura@correo.mx" || e["phone"] != "+525512345678" || e["professional_name"] != "DOC_A" {
		t.Fatalf("entry: %v", e)
	}
	if days := e["days"].([]any); len(days) != 2 || days[0] != float64(1) || days[1] != float64(3) {
		t.Fatalf("days sorted and unique: %v", days)
	}

	// Permissions: anyone who sees the agenda reads; only who manages it writes.
	for _, c := range []*client{doc, cash} {
		if got := c.expect(200, "GET", "/api/waitlist", nil)["entries"].([]any); len(got) != 1 {
			t.Fatalf("list: %v", got)
		}
		c.expect(403, "POST", "/api/waitlist", good)
		c.expect(403, "PUT", "/api/waitlist/"+id, good)
		c.expect(403, "DELETE", "/api/waitlist/"+id, nil)
		c.expect(403, "POST", "/api/waitlist/scan", nil)
	}
	w.anon().expect(401, "GET", "/api/waitlist", nil)
	w.login("help").expect(403, "GET", "/api/waitlist", nil)

	// Isolation between clinics.
	if got := other.expect(200, "GET", "/api/waitlist", nil)["entries"].([]any); len(got) != 0 {
		t.Fatalf("clinic B sees clinic A's list: %v", got)
	}
	other.expect(404, "PUT", "/api/waitlist/"+id, map[string]any{"name": "X Y", "email": "x@y.mx"})
	other.expect(404, "DELETE", "/api/waitlist/"+id, nil)
	foreign := map[string]any{"name": "X Y", "email": "x@y.mx", "professional_id": w.pro} // clinic A's doctor
	other.expect(400, "POST", "/api/waitlist", foreign)
	foreign = map[string]any{"name": "X Y", "email": "x@y.mx", "service_id": w.service()}
	other.expect(400, "POST", "/api/waitlist", foreign)

	// Validation.
	bad := func(mod func(m map[string]any)) {
		t.Helper()
		m := map[string]any{"name": "Ana Ruiz", "email": "ana@x.mx"}
		mod(m)
		recep.expect(400, "POST", "/api/waitlist", m)
	}
	bad(func(m map[string]any) { m["name"] = " " })
	bad(func(m map[string]any) { delete(m, "email") })
	bad(func(m map[string]any) { m["email"] = "no-es-correo" })
	bad(func(m map[string]any) { m["phone"] = "123" })
	bad(func(m map[string]any) { m["days"] = []int{7} })
	bad(func(m map[string]any) { m["from_time"] = "10:00" })
	bad(func(m map[string]any) { m["from_time"], m["to_time"] = "12:00", "10:00" })
	bad(func(m map[string]any) { m["professional_id"] = "zzz" })
	bad(func(m map[string]any) { m["patient_id"] = "00000000-0000-0000-0000-000000000000" })
	bad(func(m map[string]any) { m["status"] = "offered" })

	// Edit, mark as booked, take off.
	good["name"] = "Laura P. Pérez"
	upd := sub(recep.expect(200, "PUT", "/api/waitlist/"+id, good), "entry")
	if upd["name"] != "Laura P. Pérez" {
		t.Fatalf("update: %v", upd)
	}
	good["status"] = "booked"
	recep.expect(200, "PUT", "/api/waitlist/"+id, good)
	good["status"] = "waiting"
	recep.expect(409, "PUT", "/api/waitlist/"+id, good) // a booked person does not come back
	if got := recep.expect(200, "GET", "/api/waitlist", nil)["entries"].([]any); len(got) != 0 {
		t.Fatalf("booked people leave the active list: %v", got)
	}
	id2 := w.entry(recep, "Marta Díaz", "marta@correo.mx", true, "", "")
	recep.expect(204, "DELETE", "/api/waitlist/"+id2, nil)
	recep.expect(404, "DELETE", "/api/waitlist/"+id2, nil) // already off the list
	if st := w.entryStatus(id2); st != "cancelled" {
		t.Fatalf("a removal keeps the row as cancelled: %s", st)
	}
	if n := w.count(`SELECT count(*) FROM activity_log WHERE clinic_id = $1 AND type LIKE 'waitlist_%'`, w.clinicA); n < 4 {
		t.Fatalf("waitlist actions must be audited: %d", n)
	}
}

// ---------------------------------------------------------------------------
// Waitlist: public sign-up
// ---------------------------------------------------------------------------

func TestWaitlistPublicJoin(t *testing.T) {
	w := newWLEnv(t)
	anon := w.anon()
	path := "/api/public/booking/" + w.slugA + "/waitlist"
	body := func(mod func(m map[string]any)) map[string]any {
		m := map[string]any{"names": "Carla", "last_names": "Soto", "email": "Carla@Correo.mx", "phone": "5512345678", "professional_id": w.pro,
			"days": []int{1, 2}, "from_time": "09:00", "to_time": "12:00", "accept_privacy": true, "accept_notices": true}
		if mod != nil {
			mod(m)
		}
		return m
	}
	anon.expect(201, "POST", path, body(nil))
	m := w.mail.wait(t, 1)
	if m.To[0] != "carla@correo.mx" || !strings.Contains(m.Text, "/espera/") {
		t.Fatalf("sign-up mail: %+v", m)
	}
	var created, consent, email string
	if err := w.pool.QueryRow(context.Background(), `SELECT created_via, consent::text, email FROM waitlist_entries WHERE clinic_id = $1`, w.clinicA).Scan(&created, &consent, &email); err != nil {
		t.Fatal(err)
	}
	if created != "online" || consent != "true" || email != "carla@correo.mx" {
		t.Fatalf("row: %s %s %s", created, consent, email)
	}

	// The same person asking for the same thing again changes nothing (and reveals nothing).
	anon.expect(201, "POST", path, body(nil))
	if n := w.count(`SELECT count(*) FROM waitlist_entries`); n != 1 {
		t.Fatalf("duplicate sign-up: %d", n)
	}
	anon.expect(201, "POST", path, body(func(m map[string]any) { delete(m, "professional_id") })) // a different wish
	if n := w.count(`SELECT count(*) FROM waitlist_entries`); n != 2 {
		t.Fatalf("different sign-up: %d", n)
	}

	for name, mod := range map[string]func(m map[string]any){
		"no privacy":        func(m map[string]any) { m["accept_privacy"] = false },
		"no consent":        func(m map[string]any) { m["accept_notices"] = false },
		"no email":          func(m map[string]any) { m["email"] = "" },
		"bad email":         func(m map[string]any) { m["email"] = "xx" },
		"no surname":        func(m map[string]any) { m["last_names"] = " " },
		"foreign pro":       func(m map[string]any) { m["professional_id"] = w.userID("doc_b") },
		"unbookable pro":    func(m map[string]any) { m["professional_id"] = w.userID("recep_a") },
		"bad service":       func(m map[string]any) { m["service_id"] = "x" },
		"bad day":           func(m map[string]any) { m["days"] = []int{9} },
		"half window":       func(m map[string]any) { delete(m, "to_time") },
		"unknown field":     func(m map[string]any) { m["hack"] = 1 },
		"bad phone":         func(m map[string]any) { m["phone"] = "12" },
		"too long notes":    func(m map[string]any) { m["notes"] = strings.Repeat("x", 501) },
		"service of others": func(m map[string]any) { m["service_id"] = "00000000-0000-0000-0000-000000000000" },
	} {
		m := body(mod)
		if name != "no email" && name != "bad email" {
			m["email"] = fmt.Sprintf("p%d@correo.mx", len(name))
		}
		if code, out := anon.do("POST", path, m); code != 400 {
			t.Fatalf("%s: got %d %v", name, code, out)
		}
	}

	// Honeypot: looks like success, stores nothing.
	before := w.count(`SELECT count(*) FROM waitlist_entries`)
	anon.expect(201, "POST", path, body(func(m map[string]any) { m["website"] = "http://spam"; m["email"] = "bot@x.mx" }))
	if w.count(`SELECT count(*) FROM waitlist_entries`) != before {
		t.Fatal("the honeypot must not store anything")
	}
	anon.expect(404, "POST", "/api/public/booking/clinica-b/waitlist", body(nil)) // booking disabled
	anon.expect(404, "POST", "/api/public/booking/no-existe/waitlist", body(nil))
	if n := w.count(`SELECT count(*) FROM waitlist_entries WHERE clinic_id = $1`, w.clinicB); n != 0 {
		t.Fatal("nothing for clinic B")
	}
}

func TestWaitlistPublicJoinLimits(t *testing.T) {
	w := newWLEnv(t)
	path := "/api/public/booking/" + w.slugA + "/waitlist"
	mk := func(n int, email string) map[string]any {
		return map[string]any{"names": "Carla", "last_names": "Soto", "email": email, "days": []int{n}, "accept_privacy": true, "accept_notices": true}
	}
	// Per contact: 3 sign-ups an hour.
	anon := w.anon()
	for i := 0; i < 3; i++ {
		anon.expect(201, "POST", path, mk(i, "uno@correo.mx"))
	}
	anon.expect(429, "POST", path, mk(5, "uno@correo.mx"))
	// Per IP: 6 an hour.
	for i := 0; i < 3; i++ {
		anon.expect(201, "POST", path, mk(i, fmt.Sprintf("otro%d@correo.mx", i)))
	}
	anon.expect(429, "POST", path, mk(1, "ultimo@correo.mx"))
}

// ---------------------------------------------------------------------------
// Waitlist: offers
// ---------------------------------------------------------------------------

func (w *wlEnv) post(c *client, token, action string) (int, map[string]any) {
	return c.do("POST", "/api/public/waitlist/"+token, map[string]any{"action": action})
}

func TestWaitlistOfferAcceptFlow(t *testing.T) {
	w := newWLEnv(t)
	w.onlyDay()
	recep, admin, doc, cash := w.login("recep_a"), w.login("admin_a"), w.login("doc_a"), w.login("cash_a")
	anon := w.anon()

	// The whole 10:00 slot is taken, so nothing can be offered yet.
	taken := w.appt(recep, "10:00", "10:30")
	first := w.entry(recep, "Laura Pérez", "laura@correo.mx", true, "10:00", "10:30")
	second := w.entry(recep, "Mario Ruiz", "mario@correo.mx", true, "10:00", "10:30")
	if n := w.scan(); n != 0 {
		t.Fatalf("no free slot, no offer: %d", n)
	}

	// The slot frees up: a cancellation offers it to the first in line, once.
	recep.expect(200, "POST", "/api/appointments/"+taken+"/status", map[string]any{"status": "cancelled"})
	if n := w.scan(); n != 1 {
		t.Fatalf("one slot, one offer: %d", n)
	}
	if n := w.scan(); n != 0 { // the slot is held: the next person in line waits
		t.Fatalf("second scan: %d", n)
	}
	if w.entryStatus(first) != "offered" || w.entryStatus(second) != "waiting" {
		t.Fatalf("first come first served: %s %s", w.entryStatus(first), w.entryStatus(second))
	}
	m := w.mail.wait(t, 1)
	tok := w.token(first)
	if m.To[0] != "laura@correo.mx" || !strings.Contains(m.Text, "/espera/"+tok) || !strings.Contains(m.Subject, "Clinica a") {
		t.Fatalf("offer mail: %+v", m)
	}
	var offerCount int
	_ = w.pool.QueryRow(context.Background(), `SELECT count(*) FROM waitlist_offers WHERE status = 'offered' AND start_hour = '10:00'`).Scan(&offerCount)
	if offerCount != 1 {
		t.Fatalf("live offers for the slot: %d", offerCount)
	}

	// Staff see the offer in the list; a notification went out to who manages the agenda.
	list := recep.expect(200, "GET", "/api/waitlist", nil)["entries"].([]any)
	var offered map[string]any
	for _, x := range list {
		if x.(map[string]any)["id"] == first {
			offered = x.(map[string]any)
		}
	}
	if offered == nil || offered["status"] != "offered" || offered["offer"] == nil || sub(offered, "offer")["start"] != "10:00" {
		t.Fatalf("staff list: %v", offered)
	}
	if n := len(admin.expect(200, "GET", "/api/notifications?unread=1", nil)["notifications"].([]any)); n < 1 {
		t.Fatal("the offer must notify the agenda managers")
	}
	for _, c := range []*client{doc, cash} { // doc_a is the professional but can manage the agenda? No: targeted
		c.expect(200, "GET", "/api/notifications", nil)
	}

	// The public page shows it; the slot is held against online bookings and the public availability.
	view := sub(anon.expect(200, "GET", "/api/public/waitlist/"+tok, nil), "waitlist")
	if view["status"] != "offered" || sub(view, "offer")["start"] != "10:00" || view["name"] != "Laura" {
		t.Fatalf("public view: %v", view)
	}
	for _, s := range w.availability(anon, w.date, "") {
		if s.(map[string]any)["start"] == "10:00" {
			t.Fatal("a held slot must not be offered to the public")
		}
	}
	if code, _ := w.book(anon, "10:00", "otro@x.mx", ""); code != 409 {
		t.Fatalf("a held slot must not be bookable online: %d", code)
	}
	if w.monthDays(anon, "")[w.date] != 17 {
		t.Fatalf("month availability counts held slots as taken: %d", w.monthDays(anon, "")[w.date])
	}

	// Accept: the appointment exists, the entry is booked.
	code, out := w.post(anon, tok, "accept")
	if code != 200 || sub(out, "waitlist")["status"] != "booked" {
		t.Fatalf("accept: %d %v", code, out)
	}
	var apptID, src, status, email string
	if err := w.pool.QueryRow(context.Background(), `SELECT id, source, status, email FROM appointments WHERE clinic_id = $1 AND date = $2::date AND start_hour = '10:00' AND status <> 'cancelled'`, w.clinicA, w.date).
		Scan(&apptID, &src, &status, &email); err != nil {
		t.Fatalf("appointment: %v", err)
	}
	if src != "online" || status != "scheduled" || email != "laura@correo.mx" {
		t.Fatalf("appointment: %s %s %s", src, status, email)
	}
	if w.entryStatus(first) != "booked" {
		t.Fatal("entry must be booked")
	}
	if code, _ = w.post(anon, tok, "accept"); code != 409 {
		t.Fatalf("a second accept: %d", code)
	}
	w.mail.wait(t, 2) // the appointment confirmation
	if n := w.count(`SELECT count(*) FROM notifications WHERE clinic_id = $1 AND user_id IS NULL AND kind = 'waitlist_accepted'`, w.clinicA); n != 1 {
		t.Fatalf("accepted notification: %d", n)
	}
	// Nothing leaks across clinics.
	if n := w.count(`SELECT count(*) FROM notifications WHERE clinic_id = $1`, w.clinicB); n != 0 {
		t.Fatalf("clinic B notifications: %d", n)
	}
}

func TestWaitlistDeclineExpiryAndNextInLine(t *testing.T) {
	w := newWLEnv(t)
	w.onlyDay()
	recep := w.login("recep_a")
	anon := w.anon()
	a := w.entry(recep, "Ana Uno", "a@correo.mx", true, "10:00", "10:30")
	b := w.entry(recep, "Beto Dos", "b@correo.mx", true, "10:00", "10:30")
	c := w.entry(recep, "Carla Tres", "c@correo.mx", true, "10:00", "10:30")
	if n := w.scan(); n != 1 {
		t.Fatalf("offers: %d", n)
	}
	if w.entryStatus(a) != "offered" {
		t.Fatal("a first")
	}
	// A declines: back to waiting, but the same slot is not offered to them again; B gets it.
	if code, _ := w.post(anon, w.token(a), "decline"); code != 200 {
		t.Fatalf("decline: %d", code)
	}
	if w.entryStatus(a) != "waiting" {
		t.Fatalf("declined: %s", w.entryStatus(a))
	}
	if n := w.scan(); n != 1 || w.entryStatus(b) != "offered" || w.entryStatus(a) != "waiting" {
		t.Fatalf("after decline: %s %s %s", w.entryStatus(a), w.entryStatus(b), w.entryStatus(c))
	}
	// B never answers in time: accepting late is refused and B goes back to the list.
	w.exec(`UPDATE waitlist_offers SET expires_at = now() - interval '1 minute' WHERE status = 'offered'`)
	code, out := w.post(anon, w.token(b), "accept")
	if code != 409 {
		t.Fatalf("expired accept: %d %v", code, out)
	}
	if w.entryStatus(b) != "waiting" || w.count(`SELECT count(*) FROM appointments WHERE clinic_id = $1 AND source = 'online'`, w.clinicA) != 0 {
		t.Fatal("an expired offer must not create an appointment")
	}
	// C is next; when C does not answer either, the worker expires the offer.
	if n := w.scan(); n != 1 || w.entryStatus(c) != "offered" {
		t.Fatalf("c next in line: %d %s", n, w.entryStatus(c))
	}
	w.exec(`UPDATE waitlist_offers SET expires_at = now() - interval '1 minute' WHERE entry_id = $1 AND status = 'offered'`, c)
	if n := w.scan(); n != 0 { // everybody already had this slot
		t.Fatalf("expiry pass: %d", n)
	}
	if w.entryStatus(c) != "waiting" || w.count(`SELECT count(*) FROM waitlist_offers WHERE entry_id = $1 AND status = 'expired'`, c) != 1 {
		t.Fatalf("the worker must expire C's offer: %s", w.entryStatus(c))
	}
	// A new person gets it; leaving withdraws the offer.
	d := w.entry(recep, "Diego Cuatro", "d@correo.mx", true, "10:00", "10:30")
	if n := w.scan(); n != 1 || w.entryStatus(d) != "offered" {
		t.Fatalf("d: %d %s", n, w.entryStatus(d))
	}
	if code, _ := w.post(anon, w.token(d), "leave"); code != 200 {
		t.Fatalf("leave: %d", code)
	}
	if w.entryStatus(d) != "cancelled" || w.count(`SELECT count(*) FROM waitlist_offers WHERE status = 'offered'`) != 0 {
		t.Fatal("leaving withdraws the offer")
	}
	if code, _ := w.post(anon, w.token(d), "accept"); code != 409 {
		t.Fatalf("accepting after leaving: %d", code)
	}
}

func TestWaitlistAcceptRevalidatesSlot(t *testing.T) {
	w := newWLEnv(t)
	w.onlyDay()
	recep := w.login("recep_a")
	anon := w.anon()
	a := w.entry(recep, "Ana Uno", "a@correo.mx", true, "10:00", "10:30")
	w.scan()
	if w.entryStatus(a) != "offered" {
		t.Fatal("offered")
	}
	// Staff (who are not bound by the hold) book the slot meanwhile.
	w.appt(recep, "10:00", "10:30")
	code, out := w.post(anon, w.token(a), "accept")
	if code != 409 || !strings.Contains(fmt.Sprint(out["message"]), "ya no está disponible") {
		t.Fatalf("taken meanwhile: %d %v", code, out)
	}
	if w.entryStatus(a) != "waiting" {
		t.Fatalf("the person goes back to the list: %s", w.entryStatus(a))
	}
	if n := w.count(`SELECT count(*) FROM appointments WHERE clinic_id = $1 AND date = $2::date AND start_hour = '10:00' AND status <> 'cancelled'`, w.clinicA, w.date); n != 1 {
		t.Fatalf("only the staff's appointment: %d", n)
	}
	// And a block does the same.
	b := w.entry(recep, "Beto Dos", "b@correo.mx", true, "11:00", "11:30")
	w.scan()
	if w.entryStatus(b) != "offered" {
		t.Fatalf("b: %s", w.entryStatus(b))
	}
	w.exec(`INSERT INTO time_blocks (clinic_id, professional_id, date_from, date_to, start_hour, end_hour) VALUES ($1, $2, $3, $3, '11:00', '12:00')`, w.clinicA, w.pro, w.date)
	if code, _ := w.post(anon, w.token(b), "accept"); code != 409 {
		t.Fatalf("blocked meanwhile: %d", code)
	}
	if w.entryStatus(b) != "waiting" {
		t.Fatalf("b back to the list: %s", w.entryStatus(b))
	}
}

func TestWaitlistNotContactableNotifiesStaffOnce(t *testing.T) {
	w := newWLEnv(t)
	w.onlyDay()
	recep, admin := w.login("recep_a"), w.login("admin_a")
	// No consent: the system never mails them, it tells the team to call.
	w.entry(recep, "Sin Permiso", "sp@correo.mx", false, "10:00", "10:30")
	if n := w.scan(); n != 0 {
		t.Fatalf("no offer without consent: %d", n)
	}
	w.scan()
	if w.mail.count() != 0 || w.count(`SELECT count(*) FROM waitlist_offers`) != 0 {
		t.Fatal("nothing is held or mailed")
	}
	if n := w.count(`SELECT count(*) FROM notifications WHERE clinic_id = $1 AND kind = 'waitlist_match'`, w.clinicA); n != 1 {
		t.Fatalf("one internal notice per person and slot: %d", n)
	}
	// ... and the next person in line, who can be mailed, still gets the slot.
	w.entry(recep, "Con Permiso", "cp@correo.mx", true, "10:00", "10:30")
	if n := w.scan(); n != 1 {
		t.Fatalf("offer to the next in line: %d", n)
	}
	// The manual scan button works for managers only.
	admin.expect(200, "POST", "/api/waitlist/scan", nil)
}

// Several workers (several API instances) scanning at once must hand the slot to one person only.
func TestWaitlistConcurrentScansOfferOnce(t *testing.T) {
	w := newWLEnv(t)
	w.onlyDay()
	recep := w.login("recep_a")
	var ids []string
	for i := 0; i < 5; i++ {
		ids = append(ids, w.entry(recep, fmt.Sprintf("Persona %d Uno", i), fmt.Sprintf("p%d@correo.mx", i), true, "10:00", "10:30"))
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	total := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := api.RunWaitlistOnce(context.Background(), w.pool, w.cfg, w.mail)
			if err != nil {
				t.Error(err)
			}
			mu.Lock()
			total += n
			mu.Unlock()
		}()
	}
	wg.Wait()
	if total != 1 {
		t.Fatalf("one slot must produce one offer, got %d", total)
	}
	if n := w.count(`SELECT count(*) FROM waitlist_offers WHERE status = 'offered'`); n != 1 {
		t.Fatalf("live offers: %d", n)
	}
	if n := w.count(`SELECT count(*) FROM waitlist_entries WHERE status = 'offered'`); n != 1 {
		t.Fatalf("offered entries: %d", n)
	}
	if w.entryStatus(ids[0]) != "offered" {
		t.Fatal("the first in line gets it")
	}
	w.mail.wait(t, 1)
	time.Sleep(100 * time.Millisecond)
	if w.mail.count() != 1 {
		t.Fatalf("one offer mail, got %d", w.mail.count())
	}
}

// Accepting the same offer many times at once, while staff try to take the same slot, yields one appointment.
func TestWaitlistConcurrentAcceptAndStaffBooking(t *testing.T) {
	w := newWLEnv(t)
	w.onlyDay()
	recep := w.login("recep_a")
	a := w.entry(recep, "Ana Uno", "a@correo.mx", true, "10:00", "10:30")
	if w.scan() != 1 {
		t.Fatal("offer")
	}
	tok := w.token(a)
	var wg sync.WaitGroup
	codes := make(chan int, 12)
	for i := 0; i < 6; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			code, _ := w.post(w.anon(), tok, "accept")
			codes <- code
		}()
		go func(i int) {
			defer wg.Done()
			code, _ := w.login("recep_a").do("POST", "/api/appointments", map[string]any{
				"names": "Pac", "last_names": fmt.Sprint(i), "date": w.date, "startHour": "10:00", "endHour": "10:30", "professional_id": w.pro,
			})
			codes <- code
		}(i)
	}
	wg.Wait()
	close(codes)
	created := 0
	for c := range codes {
		if c == http.StatusOK || c == http.StatusCreated {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("exactly one request may take the slot, got %d", created)
	}
	if n := w.count(`SELECT count(*) FROM appointments WHERE clinic_id = $1 AND date = $2::date AND start_hour = '10:00' AND status NOT IN ('cancelled', 'no_show')`, w.clinicA, w.date); n != 1 {
		t.Fatalf("appointments in the slot: %d", n)
	}
	if st := w.entryStatus(a); st != "booked" && st != "waiting" {
		t.Fatalf("entry: %s", st)
	}
}

// A slot that opens for one clinic is never offered to another clinic's list.
func TestWaitlistClinicIsolation(t *testing.T) {
	w := newWLEnv(t)
	w.onlyDay()
	adminB := w.login("admin_b")
	docB := w.userID("doc_b")
	out := adminB.expect(201, "POST", "/api/waitlist", map[string]any{"name": "Persona De B", "email": "b@correo.mx", "consent": true, "professional_id": docB})
	idB := sub(out, "entry")["id"].(string)
	recep := w.login("recep_a")
	w.entry(recep, "Laura Pérez", "laura@correo.mx", true, "10:00", "10:30")
	w.scan()
	if n := w.count(`SELECT count(*) FROM waitlist_offers o JOIN waitlist_entries e ON e.id = o.entry_id WHERE o.clinic_id <> e.clinic_id`); n != 0 {
		t.Fatalf("offers across clinics: %d", n)
	}
	if n := w.count(`SELECT count(*) FROM waitlist_offers WHERE clinic_id = $1 AND professional_id = $2`, w.clinicA, docB); n != 0 {
		t.Fatalf("B's professional offered in A: %d", n)
	}
	if n := w.count(`SELECT count(*) FROM waitlist_offers WHERE clinic_id = $1 AND professional_id = $2`, w.clinicB, w.pro); n != 0 {
		t.Fatalf("A's professional offered in B: %d", n)
	}
	// Tokens do not cross either: each page shows only its own clinic's name.
	tokB := w.token(idB)
	view := sub(w.anon().expect(200, "GET", "/api/public/waitlist/"+tokB, nil), "waitlist")
	if sub(view, "clinic")["name"] != "Clinica b" {
		t.Fatalf("view: %v", view)
	}
	w.anon().expect(404, "GET", "/api/public/waitlist/"+strings.Repeat("a", 43), nil)
	w.anon().expect(404, "GET", "/api/public/waitlist/corto", nil)
	if code, _ := w.post(w.anon(), strings.Repeat("a", 43), "accept"); code != 404 {
		t.Fatalf("unknown token: %d", code)
	}
	if code, _ := w.post(w.anon(), tokB, "explode"); code != 400 {
		t.Fatalf("unknown action: %d", code)
	}
}

// ---------------------------------------------------------------------------
// Notifications
// ---------------------------------------------------------------------------

func (w *wlEnv) notes(c *client, query string) ([]map[string]any, int) {
	w.t.Helper()
	out := c.expect(200, "GET", "/api/notifications"+query, nil)
	var list []map[string]any
	for _, x := range out["notifications"].([]any) {
		list = append(list, x.(map[string]any))
	}
	return list, int(out["unread"].(float64))
}

func kinds(list []map[string]any) string {
	var k []string
	for _, n := range list {
		k = append(k, n["kind"].(string))
	}
	return strings.Join(k, ",")
}

func TestNotificationsByRoleAndRead(t *testing.T) {
	w := newWLEnv(t)
	anon := w.anon()
	admin, recep, doc, cash, adminB := w.login("admin_a"), w.login("recep_a"), w.login("doc_a"), w.login("cash_a"), w.login("admin_b")

	// An online booking notifies those who manage the agenda and the professional (a doctor).
	code, out := w.book(anon, "10:00", "ana@correo.mx", "")
	if code != 201 {
		t.Fatal(out)
	}
	tok := w.token2(out)
	for name, c := range map[string]*client{"admin": admin, "recep": recep, "doc": doc} {
		list, unread := w.notes(c, "?unread=1")
		if len(list) != 1 || unread != 1 || list[0]["kind"] != "booking_new" || list[0]["read"] != false {
			t.Fatalf("%s: %v (%d)", name, list, unread)
		}
		if !strings.Contains(list[0]["body"].(string), "Ana") || list[0]["link"] != "/admin/navegar-citas" {
			t.Fatalf("%s body: %v", name, list[0])
		}
	}
	if list, unread := w.notes(cash, ""); len(list) != 0 || unread != 0 {
		t.Fatalf("a cashier does not manage the agenda: %v", list)
	}
	if list, _ := w.notes(adminB, ""); len(list) != 0 {
		t.Fatalf("clinic B must see nothing: %v", list)
	}
	w.anon().expect(401, "GET", "/api/notifications", nil)
	w.login("root").expect(403, "GET", "/api/notifications", nil) // platform staff have no clinic bell

	// The patient confirms and then cancels from the link.
	anon.expect(200, "POST", "/api/public/appointments/"+tok+"/confirm", nil)
	anon.expect(200, "POST", "/api/public/appointments/"+tok+"/cancel", map[string]any{"reason": "Viaje"})
	list, unread := w.notes(recep, "")
	if unread != 3 || kinds(list) != "appointment_cancelled_patient,appointment_confirmed_patient,booking_new" {
		t.Fatalf("recep: %s (%d)", kinds(list), unread)
	}

	// Reading is per person: marking one as read for reception leaves the administrator's unread.
	id := list[0]["id"].(string)
	got := recep.expect(200, "POST", "/api/notifications/"+id+"/read", nil)
	if got["unread"] != float64(2) {
		t.Fatalf("after read: %v", got)
	}
	if _, u := w.notes(admin, ""); u != 3 {
		t.Fatalf("the administrator's own state: %d", u)
	}
	if l, _ := w.notes(recep, "?unread=1"); len(l) != 2 {
		t.Fatalf("unread filter: %d", len(l))
	}
	recep.expect(200, "POST", "/api/notifications/"+id+"/read", nil) // idempotent
	recep.expect(404, "POST", "/api/notifications/"+id+"/read-x", nil)
	recep.expect(404, "POST", "/api/notifications/not-a-uuid/read", nil)
	cash.expect(404, "POST", "/api/notifications/"+id+"/read", nil) // not theirs to see
	adminB.expect(404, "POST", "/api/notifications/"+id+"/read", nil)
	// The doctor's targeted copy is theirs alone.
	var docNote string
	_ = w.pool.QueryRow(context.Background(), `SELECT id FROM notifications WHERE user_id = $1 AND kind = 'booking_new'`, w.pro).Scan(&docNote)
	recep.expect(404, "POST", "/api/notifications/"+docNote+"/read", nil)
	doc.expect(200, "POST", "/api/notifications/"+docNote+"/read", nil)
	var readAt *time.Time
	_ = w.pool.QueryRow(context.Background(), `SELECT read_at FROM notifications WHERE id = $1`, docNote).Scan(&readAt)
	if readAt == nil {
		t.Fatal("targeted notifications keep read_at")
	}

	if got := recep.expect(200, "GET", "/api/notifications/count", nil); got["unread"] != float64(2) {
		t.Fatalf("count: %v", got)
	}
	got = recep.expect(200, "POST", "/api/notifications/read-all", nil)
	if got["unread"] != float64(0) {
		t.Fatalf("read-all: %v", got)
	}
	if _, u := w.notes(admin, ""); u != 3 {
		t.Fatalf("read-all is per person: %d", u)
	}
	if _, u := w.notes(doc, ""); u != 2 { // booking_new read, the confirm and the cancel are not
		t.Fatalf("doctor's unread: %d", u)
	}
	// A limit and the cap.
	if l, _ := w.notes(admin, "?limit=1"); len(l) != 1 {
		t.Fatalf("limit: %d", len(l))
	}
	// Old notifications drop out of the list.
	w.exec(`UPDATE notifications SET created_at = now() - interval '90 days' WHERE kind = 'booking_new'`)
	if l, _ := w.notes(admin, ""); kinds(l) != "appointment_cancelled_patient,appointment_confirmed_patient" {
		t.Fatalf("old ones hidden: %s", kinds(l))
	}
}

func (w *wlEnv) token2(out map[string]any) string { return sub(out, "appointment")["token"].(string) }

func TestNotificationFailedReminderAndStock(t *testing.T) {
	b := newBookingEnv(t, true)
	w := &wlEnv{bookingEnv: b}
	b.exec(`UPDATE agenda_settings SET remind_email = false WHERE clinic_id = $1`, b.clinicA)
	_, out := b.book(b.anon(), "10:00", "", "5512345678")
	tok := b.token(out)
	b.wa.mu.Lock()
	b.wa.status = 400 // permanent
	b.wa.mu.Unlock()
	b.makeDue(tok)
	b.tick()
	admin, recep, cash := b.login("admin_a"), b.login("recep_a"), b.login("cash_a")
	n := 0
	list, _ := w.notes(recep, "")
	for _, x := range list {
		if x["kind"] == "reminder_failed" {
			n++
		}
	}
	if n != 2 { // 24 h and 2 h reminders
		t.Fatalf("failed reminders notified: %d (%s)", n, kinds(list))
	}
	if l, _ := w.notes(cash, ""); len(l) != 0 {
		t.Fatalf("cashier: %v", l)
	}

	// Stock job: lots about to expire and products at their minimum notify who manages the POS, once a day.
	b.exec(`INSERT INTO catalog_items (clinic_id, kind, name, price_cents, track_stock, stock, min_stock) VALUES ($1, 'product', 'Gasas', 100, true, 2, 5)`, b.clinicA)
	b.exec(`INSERT INTO stock_lots (clinic_id, item_id, lot_code, qty, expires_on)
		SELECT $1, id, 'L1', 10, current_date + 10 FROM catalog_items WHERE name = 'Jarabe'`, b.clinicA)
	b.exec(`UPDATE catalog_items SET track_stock = true WHERE name = 'Jarabe'`)
	made, err := api.RunNotificationJobsOnce(context.Background(), b.pool, b.cfg, b.mail)
	if err != nil || made != 1 {
		t.Fatalf("stock job: %d %v", made, err)
	}
	made, _ = api.RunNotificationJobsOnce(context.Background(), b.pool, b.cfg, b.mail)
	if made != 0 {
		t.Fatalf("the job must be idempotent within a day: %d", made)
	}
	list, _ = w.notes(admin, "")
	found := false
	for _, x := range list {
		if x["kind"] == "stock_alert" {
			found = true
			if !strings.Contains(x["body"].(string), "mínima") || !strings.Contains(x["body"].(string), "por caducar") {
				t.Fatalf("stock body: %v", x["body"])
			}
		}
	}
	if !found {
		t.Fatalf("admin must get the stock alert: %s", kinds(list))
	}
	for name, c := range map[string]*client{"recep": recep, "cash": cash} {
		l, _ := w.notes(c, "")
		for _, x := range l {
			if x["kind"] == "stock_alert" {
				t.Fatalf("%s must not see stock alerts", name)
			}
		}
	}
	if l, _ := w.notes(b.login("admin_b"), ""); len(l) != 0 {
		t.Fatalf("clinic B: %v", l)
	}
}
