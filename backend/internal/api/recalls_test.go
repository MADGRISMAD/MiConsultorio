package api_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/madgrismad/miconsultorio/backend/internal/api"
)

func TestFollowUpRecallMails(t *testing.T) {
	b := newBookingEnv(t, false)
	ctx := context.Background()
	loc, _ := time.LoadLocation("America/Mexico_City")
	now := time.Now().In(loc)
	if now.Hour() < 9 {
		now = time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, loc)
	}
	day := func(n int) string { return now.AddDate(0, 0, n).Format("2006-01-02") }
	run := func() {
		t.Helper()
		if err := api.RunRecallsOnce(ctx, b.pool, b.cfg, b.mail, now); err != nil {
			t.Fatal(err)
		}
	}
	pat := func(n int, names, email, kinds string, ok bool) string {
		var id string
		if err := b.pool.QueryRow(ctx, `INSERT INTO patients (clinic_id, file_number, names, email, reminders_ok, kinds) VALUES ($1, $2, $3, $4, $5, $6::text[]) RETURNING id`,
			b.clinicA, n, names, email, ok, kinds).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	// a vaccine due in three days
	v := pat(1, "Luis", "luis@x.mx", "{GENERAL_MEDICAL}", true)
	b.exec(`INSERT INTO vaccinations (clinic_id, patient_id, kind, name, applied_on, next_due) VALUES ($1, $2, 'vaccine', 'Influenza', $3, $4)`, b.clinicA, v, day(-360), day(3))
	// a pet's deworming due today: the owner is written to and the pet is named
	var owner, pet string
	if err := b.pool.QueryRow(ctx, `INSERT INTO owners (clinic_id, name, email) VALUES ($1, 'Marta Ruiz', 'marta@x.mx') RETURNING id`, b.clinicA).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	if err := b.pool.QueryRow(ctx, `INSERT INTO patients (clinic_id, file_number, subject, names, owner_id, reminders_ok) VALUES ($1, 2, 'animal', 'Luna', $2, true) RETURNING id`, b.clinicA, owner).Scan(&pet); err != nil {
		t.Fatal(err)
	}
	b.exec(`INSERT INTO vaccinations (clinic_id, patient_id, kind, name, applied_on, next_due) VALUES ($1, $2, 'deworming_internal', 'Desparasitación', $3, $4)`, b.clinicA, pet, day(-90), day(0))
	// a suggested next visit in two days, with nothing booked
	n := pat(3, "Ana", "ana@x.mx", "{GENERAL_MEDICAL}", true)
	b.exec(`INSERT INTO encounters (clinic_id, patient_id, author_name, next_visit) VALUES ($1, $2, 'Doc', $3)`, b.clinicA, n, day(2))
	// the same, but already booked: no reminder
	k := pat(4, "Beto", "beto@x.mx", "{GENERAL_MEDICAL}", true)
	b.exec(`INSERT INTO encounters (clinic_id, patient_id, author_name, next_visit) VALUES ($1, $2, 'Doc', $3)`, b.clinicA, k, day(2))
	b.exec(`INSERT INTO appointments (clinic_id, patient_id, curp, names, last_names, date, start_hour, end_hour, professional_id, status) VALUES ($1, $2, '', 'Beto', '', $3, '10:00', '10:30', $4, 'scheduled')`, b.clinicA, k, day(2), b.pro)
	// dental check-up six months after the last visit
	d := pat(5, "Dora", "dora@x.mx", "{DENTAL}", true)
	b.exec(`UPDATE patients SET last_encounter_at = $2 WHERE id = $1`, d, now.AddDate(0, -6, -2))
	// nobody agreed to e-mails, or no e-mail: nothing
	nc := pat(6, "Nico", "nico@x.mx", "{DENTAL}", false)
	b.exec(`UPDATE patients SET last_encounter_at = $2 WHERE id = $1`, nc, now.AddDate(0, -6, -2))
	b.exec(`INSERT INTO vaccinations (clinic_id, patient_id, kind, name, applied_on, next_due) VALUES ($1, $2, 'vaccine', 'Tétanos', $3, $4)`, b.clinicA, nc, day(-300), day(1))

	run()
	run() // never twice
	if got := b.mail.count(); got != 4 {
		t.Fatalf("vaccine, pet deworming, suggested visit and dental check-up: %d mails", got)
	}
	subjects := map[string]string{}
	for i := 1; i <= 4; i++ {
		m := b.mail.wait(t, i)
		subjects[m.To[0]] = m.Subject + "|" + m.Text
	}
	if !strings.Contains(subjects["luis@x.mx"], "Influenza") || !strings.Contains(subjects["marta@x.mx"], "Luna") || !strings.Contains(subjects["marta@x.mx"], "Hola Marta") {
		t.Fatalf("contents: %v", subjects)
	}
	if _, bad := subjects["beto@x.mx"]; bad {
		t.Fatal("already booked: no reminder")
	}
	if _, bad := subjects["nico@x.mx"]; bad {
		t.Fatal("no consent: no reminder")
	}
	if !strings.Contains(subjects["dora@x.mx"], "revisión dental") || !strings.Contains(subjects["ana@x.mx"], "siguiente consulta") || !strings.Contains(subjects["luis@x.mx"], "/baja/") {
		t.Fatalf("contents: %v", subjects)
	}
}

func TestPapRecallMail(t *testing.T) {
	b := newBookingEnv(t, false)
	ctx := context.Background()
	loc, _ := time.LoadLocation("America/Mexico_City")
	now := time.Now().In(loc)
	if now.Hour() < 9 {
		now = time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, loc)
	}
	b.exec(`UPDATE clinics SET kind = 'GYNECOLOGY', specialties = '{}' WHERE id = $1`, b.clinicA)
	doc := b.login("doc_a")
	body := person(map[string]any{"curp": "", "sex": "Mujer", "names": "Paola", "email": "paola@x.mx", "reminders_ok": true})
	prof := body["profile"].(map[string]any)
	prof["last_pap"] = now.AddDate(-1, 0, -3).Format("2006-01-02")
	prof["contraception_renewal"] = now.AddDate(0, 0, 3).Format("2006-01-02")
	made := doc.expect(201, "POST", "/api/patients/", body)
	if sub(made, "patient")["reminders_ok"] != true {
		t.Fatalf("consent saved: %v", sub(made, "patient"))
	}
	if err := api.RunRecallsOnce(ctx, b.pool, b.cfg, b.mail, now); err != nil {
		t.Fatal(err)
	}
	got := b.mail.wait(t, 1).Subject + "|" + b.mail.wait(t, 2).Subject
	if !strings.Contains(got, "Papanicolaou") || !strings.Contains(got, "método anticonceptivo") {
		t.Fatalf("pap and contraception mails: %s", got)
	}
}
