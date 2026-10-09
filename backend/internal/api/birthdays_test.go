package api_test

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/madgrismad/miconsultorio/backend/internal/api"
)

func TestBirthdayGreetings(t *testing.T) {
	b := newBookingEnv(t, false)
	ctx := context.Background()
	var owner, luna string
	if err := b.pool.QueryRow(ctx, `INSERT INTO owners (clinic_id, name, email) VALUES ($1, 'Marta Ruiz', 'marta@x.mx') RETURNING id`, b.clinicA).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	b.exec(`INSERT INTO patients (clinic_id, file_number, names, email, reminders_ok, birth_date) VALUES ($1, 1, 'Luis', 'luis@x.mx', true, '1990-03-15')`, b.clinicA)
	b.exec(`INSERT INTO patients (clinic_id, file_number, names, email, reminders_ok) VALUES ($1, 2, 'Sin Fecha', 'sin@x.mx', true)`, b.clinicA)
	b.exec(`INSERT INTO patients (clinic_id, file_number, names, email, reminders_ok, birth_date) VALUES ($1, 3, 'Sin Permiso', 'no@x.mx', false, '1985-03-15')`, b.clinicA)
	b.exec(`INSERT INTO patients (clinic_id, file_number, names, email, reminders_ok, birth_date) VALUES ($1, 4, 'Otro Día', 'otro@x.mx', true, '1985-03-16')`, b.clinicA)
	if err := b.pool.QueryRow(ctx, `INSERT INTO patients (clinic_id, file_number, subject, names, owner_id, guardian_name, reminders_ok, birth_date)
		VALUES ($1, 5, 'animal', 'Luna', $2, 'Marta Ruiz', true, '2022-03-15') RETURNING id`, b.clinicA, owner).Scan(&luna); err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("America/Mexico_City")
	run := func(h int) {
		t.Helper()
		if err := api.RunBirthdaysOnce(ctx, b.pool, b.cfg, b.mail, time.Date(2027, 3, 15, h, 30, 0, 0, loc)); err != nil {
			t.Fatal(err)
		}
	}
	run(7) // too early: greetings leave from 9:00
	if n := b.mail.count(); n != 0 {
		t.Fatalf("before 9:00 nothing is sent: %d", n)
	}
	run(10)
	run(11) // the same day again: never twice
	if n := b.mail.count(); n != 2 {
		t.Fatalf("the person and the pet's owner, once each: %d", n)
	}
	var person, pet string
	for i := 1; i <= 2; i++ {
		m := b.mail.wait(t, i)
		switch m.To[0] {
		case "luis@x.mx":
			person = m.Text + m.Subject
		case "marta@x.mx":
			pet = m.Text + m.Subject
		}
	}
	if !strings.Contains(person, "Luis") || !strings.Contains(person, "/baja/") {
		t.Fatalf("person greeting: %s", person)
	}
	if !strings.Contains(pet, "Luna") || !strings.Contains(pet, "5 años") || !strings.Contains(pet, "Marta") {
		t.Fatalf("pet greeting goes to the owner and is about the pet: %s", pet)
	}
	// the link takes the pet's owner off the list
	tok := regexp.MustCompile(`/baja/([A-Za-z0-9.\-]+)`).FindStringSubmatch(pet)[1]
	anon := b.anon()
	anon.expect(404, "GET", "/api/public/unsubscribe/"+tok[:len(tok)-1]+"0", nil)
	anon.expect(200, "GET", "/api/public/unsubscribe/"+tok, nil)
	anon.expect(200, "POST", "/api/public/unsubscribe/"+tok, nil)
	var ok bool
	_ = b.pool.QueryRow(ctx, `SELECT reminders_ok FROM patients WHERE id = $1`, luna).Scan(&ok)
	if ok {
		t.Fatal("unsubscribed")
	}
	// the owner's own birthday is greeted too, but only with a birth date and consent
	b.exec(`UPDATE owners SET birth_date = '1980-03-15' WHERE id = $1`, owner)
	b.exec(`UPDATE patients SET reminders_ok = true WHERE id = $1`, luna)
	run(12)
	if n := b.mail.count(); n != 3 {
		t.Fatalf("the owner gets a greeting of their own: %d", n)
	}
}

func TestPetAppointmentMailsTheOwner(t *testing.T) {
	b := newBookingEnv(t, false)
	ctx := context.Background()
	var owner, luna string
	if err := b.pool.QueryRow(ctx, `INSERT INTO owners (clinic_id, name, email) VALUES ($1, 'Marta Ruiz', 'marta@x.mx') RETURNING id`, b.clinicA).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	if err := b.pool.QueryRow(ctx, `INSERT INTO patients (clinic_id, file_number, subject, names, owner_id, guardian_name, reminders_ok)
		VALUES ($1, 1, 'animal', 'Luna', $2, 'Marta Ruiz', true) RETURNING id`, b.clinicA, owner).Scan(&luna); err != nil {
		t.Fatal(err)
	}
	// no e-mail typed in the appointment: it goes to the owner, greeting the owner and naming the pet
	b.login("recep_a").expect(201, "POST", "/api/appointments", map[string]any{"patient_id": luna, "names": "Luna", "date": b.date, "startHour": "10:00", "endHour": "10:30", "professional_id": b.pro})
	m := b.mail.wait(t, 1)
	if m.To[0] != "marta@x.mx" || !strings.Contains(m.Text, "Hola Marta") || !strings.Contains(m.Text, "la cita de Luna") || !strings.Contains(m.Text, "Mascota: Luna") {
		t.Fatalf("pet booking mail: %+v", m)
	}
}
