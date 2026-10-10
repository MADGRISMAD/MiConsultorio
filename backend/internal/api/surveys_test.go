package api_test

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/madgrismad/miconsultorio/backend/internal/api"
)

func TestSurveyAndPublicProfile(t *testing.T) {
	b := newBookingEnv(t, false)
	ctx := context.Background()
	admin := b.login("admin_a")
	anon := b.anon()

	// the page is private until the clinic publishes it
	anon.expect(404, "GET", "/api/public/clinic/"+b.slugA, nil)
	admin.expect(400, "PUT", "/api/clinic/profile", map[string]any{"enabled": true, "survey_delay_hours": 0, "maps_min_rating": 4})
	admin.expect(400, "PUT", "/api/clinic/profile", map[string]any{"website": "http://inseguro.mx", "survey_delay_hours": 3, "maps_min_rating": 4})
	save := map[string]any{"enabled": true, "tagline": "Cuidamos tu sonrisa", "about": "Clínica familiar", "show_reviews": true, "survey_enabled": true,
		"survey_delay_hours": 3, "maps_min_rating": 4, "google_place_id": "ChIJN1t_tDeuEmsRUsoyG83frY4", "maps_url": "https://maps.app.goo.gl/abc"}
	out := admin.expect(200, "PUT", "/api/clinic/profile", save)
	if out["public_url"] != "/clinica/"+b.slugA || !strings.Contains(out["review_url"].(string), "writereview?placeid=ChIJ") {
		t.Fatalf("settings: %v", out)
	}
	page := anon.expect(200, "GET", "/api/public/clinic/"+b.slugA, nil)
	if page["tagline"] != "Cuidamos tu sonrisa" || page["booking_url"] != "/reservar/"+b.slugA || len(page["professionals"].([]any)) != 1 {
		t.Fatalf("public page: %v", page)
	}

	// a finished consultation with a patient who agreed to e-mails
	var pid, appt string
	if err := b.pool.QueryRow(ctx, `INSERT INTO patients (clinic_id, file_number, names, email, reminders_ok) VALUES ($1, 1, 'Ana', 'ana@x.mx', true) RETURNING id`, b.clinicA).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := b.pool.QueryRow(ctx, `INSERT INTO appointments (clinic_id, curp, names, last_names, date, start_hour, end_hour, patient_id, professional_id, status, finished_at)
		VALUES ($1, 'X', 'Ana', 'L', current_date, '09:00', '09:30', $2, $3, 'completed', $4) RETURNING id`, b.clinicA, pid, b.pro, now.Add(-4*time.Hour)).Scan(&appt); err != nil {
		t.Fatal(err)
	}
	// a visit that ended an hour ago is not asked about yet
	b.exec(`INSERT INTO appointments (clinic_id, curp, names, last_names, date, start_hour, end_hour, patient_id, professional_id, status, finished_at)
		VALUES ($1, 'X', 'Ana', 'L', current_date, '10:00', '10:30', $2, $3, 'completed', $4)`, b.clinicA, pid, b.pro, now.Add(-time.Hour))
	if err := api.RunSurveysOnce(ctx, b.pool, b.cfg, b.mail, now); err != nil {
		t.Fatal(err)
	}
	if err := api.RunSurveysOnce(ctx, b.pool, b.cfg, b.mail, now); err != nil { // never twice
		t.Fatal(err)
	}
	if n := b.mail.count(); n != 1 {
		t.Fatalf("one survey for the visit that is due: %d", n)
	}
	m := b.mail.wait(t, 1)
	tok := regexp.MustCompile(`/encuesta/([A-Za-z0-9_-]+)`).FindStringSubmatch(m.Text)[1]

	v := anon.expect(200, "GET", "/api/public/survey/"+tok, nil)
	if v["answered"] != false || v["clinic_name"] == "" {
		t.Fatalf("survey: %v", v)
	}
	anon.expect(404, "GET", "/api/public/survey/nope", nil)
	anon.expect(400, "POST", "/api/public/survey/"+tok, map[string]any{"rating": 9})
	done := anon.expect(200, "POST", "/api/public/survey/"+tok, map[string]any{"rating": 5, "comment": "Excelente trato", "public_ok": true})
	if !strings.Contains(done["review_url"].(string), "writereview") {
		t.Fatalf("a good rating is invited to Google: %v", done)
	}
	anon.expect(409, "POST", "/api/public/survey/"+tok, map[string]any{"rating": 1})

	page = anon.expect(200, "GET", "/api/public/clinic/"+b.slugA, nil)
	rating := page["rating"].(map[string]any)
	if rating["count"] != float64(1) || rating["average"] != float64(5) || len(page["reviews"].([]any)) != 1 {
		t.Fatalf("rating on the page: %v", page)
	}
	sum := admin.expect(200, "GET", "/api/clinic/surveys", nil)
	if sum["sent"] != float64(1) || sum["answered"] != float64(1) {
		t.Fatalf("summary: %v", sum)
	}
	// the receptionist does not see the results
	b.login("recep_a").expect(403, "GET", "/api/clinic/surveys", nil)
}
