package api_test

import (
	"context"
	"net/http"
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
	if page["tagline"] != "Cuidamos tu sonrisa" || page["booking_url"] != "/reservar/"+b.slugA || len(page["professionals"].([]any)) != 2 {
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

// Photos of the public page: one profile picture, one banner, up to five gallery photos and one per specialist; the page
// follows the team (a hidden or disabled specialist, and a giro the clinic left, disappear).
func TestClinicPageMedia(t *testing.T) {
	b := newBookingEnv(t, false)
	admin := b.login("admin_a")
	anon := b.anon()
	png := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
	admin.expect(200, "PUT", "/api/clinic/profile", map[string]any{"enabled": true, "tagline": "Hola", "show_reviews": true, "survey_delay_hours": 3, "maps_min_rating": 4, "contact_email": "hola@clinica.mx"})

	// validation
	admin.expect(400, "POST", "/api/clinic/media", map[string]any{"slot": "cover", "image": "data:image/png;base64,AAAA"})                 // not an image
	admin.expect(400, "POST", "/api/clinic/media", map[string]any{"slot": "cover", "image": "data:image/gif;base64,R0lGODlhAQABAAAAACw="}) // not a type we take
	admin.expect(400, "POST", "/api/clinic/media", map[string]any{"slot": "nope", "image": png})
	admin.expect(400, "POST", "/api/clinic/media", map[string]any{"slot": "pro", "user_id": "x", "image": png})
	b.login("recep_a").expect(403, "POST", "/api/clinic/media", map[string]any{"slot": "cover", "image": png})

	first := admin.expect(201, "POST", "/api/clinic/media", map[string]any{"slot": "cover", "image": png})["id"].(string)
	second := admin.expect(201, "POST", "/api/clinic/media", map[string]any{"slot": "cover", "image": png})["id"].(string) // replaces
	if first == second {
		t.Fatal("a new photo is a new row")
	}
	admin.expect(201, "POST", "/api/clinic/media", map[string]any{"slot": "profile", "image": png})
	var gal []string
	for i := 0; i < 5; i++ {
		gal = append(gal, admin.expect(201, "POST", "/api/clinic/media", map[string]any{"slot": "gallery", "image": png})["id"].(string))
	}
	admin.expect(409, "POST", "/api/clinic/media", map[string]any{"slot": "gallery", "image": png}) // the sixth

	page := anon.expect(200, "GET", "/api/public/clinic/"+b.slugA, nil)
	if len(page["gallery"].([]any)) != 5 || !strings.HasSuffix(page["cover_url"].(string), second) || page["profile_url"] == "" || page["email"] != "hola@clinica.mx" {
		t.Fatalf("page: %v", page)
	}
	res, err := http.Get(b.srv.URL + page["cover_url"].(string))
	if err != nil || res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("cover: %v %v", err, res)
	}
	res.Body.Close()
	req, _ := http.NewRequest("GET", b.srv.URL+page["cover_url"].(string), nil)
	req.Header.Set("If-None-Match", res.Header.Get("ETag"))
	if res2, _ := http.DefaultClient.Do(req); res2.StatusCode != 304 {
		t.Fatalf("etag: %d", res2.StatusCode)
	}
	admin.expect(200, "DELETE", "/api/clinic/media/"+gal[0], nil)
	admin.expect(404, "DELETE", "/api/clinic/media/"+gal[0], nil)
	if res3, _ := http.Get(b.srv.URL + "/api/public/clinic/" + b.slugA + "/media/" + gal[0]); res3.StatusCode != 404 {
		t.Fatalf("deleted photo: %d", res3.StatusCode)
	}

	// the team: a photo per specialist, hide one, disable another
	doc := b.userID("doc_a")
	photo := admin.expect(201, "POST", "/api/clinic/media", map[string]any{"slot": "pro", "user_id": doc, "image": png})["id"].(string)
	pros := func() []any {
		return anon.expect(200, "GET", "/api/public/clinic/"+b.slugA, nil)["professionals"].([]any)
	}
	if len(pros()) != 2 {
		t.Fatalf("team: %v", pros())
	}
	admin.expect(200, "PUT", "/api/clinic/profile/professionals/"+doc, map[string]any{"hidden": true})
	if len(pros()) != 1 {
		t.Fatalf("hidden: %v", pros())
	}
	if res4, _ := http.Get(b.srv.URL + "/api/public/clinic/" + b.slugA + "/media/" + photo); res4.StatusCode != 404 {
		t.Fatalf("a hidden specialist keeps no public photo: %d", res4.StatusCode)
	}
	admin.expect(200, "PUT", "/api/clinic/profile/professionals/"+doc, map[string]any{"hidden": false})
	b.exec(`UPDATE users SET disabled = true WHERE id = $1`, doc)
	if len(pros()) != 1 {
		t.Fatalf("disabled: %v", pros())
	}
	b.exec(`UPDATE users SET disabled = false WHERE id = $1`, doc)
	// a specialist of a giro the clinic no longer works in
	b.exec(`UPDATE users SET areas = '{DERMATOLOGY}' WHERE id = $1`, doc)
	if len(pros()) != 1 {
		t.Fatalf("giro left: %v", pros())
	}
	ov := admin.expect(200, "GET", "/api/clinic/media", nil)
	if len(ov["professionals"].([]any)) < 1 || ov["cover"] != second {
		t.Fatalf("overview: %v", ov)
	}
}
