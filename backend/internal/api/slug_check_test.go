package api_test

import "testing"

// The address check says whether a public address is free and offers free variants when it is not.
func TestSlugCheck(t *testing.T) {
	b := newBookingEnv(t, false)
	admin := b.login("admin_a")
	b.exec(`INSERT INTO clinic_profile (clinic_id, city) VALUES ($1, 'Mérida') ON CONFLICT (clinic_id) DO UPDATE SET city = 'Mérida'`, b.clinicA)
	check := func(c *client, slug string) map[string]any {
		return c.expect(200, "GET", "/api/agenda/slug-check?slug="+slug, nil)
	}
	b.login("doc_a").expect(403, "GET", "/api/agenda/slug-check?slug=x", nil)

	// free, and the clinic's own address, are fine
	if r := check(admin, "mi-clinica-nueva"); r["available"] != true || r["state"] != "ok" {
		t.Fatalf("free: %v", r)
	}
	if r := check(admin, b.slugA); r["available"] != true {
		t.Fatalf("own address is available to its owner: %v", r)
	}
	// taken by another clinic: the other clinic sees it as taken and gets free variants
	other := b.login("admin_b")
	r := check(other, b.slugA)
	if r["available"] != false || r["state"] != "taken" {
		t.Fatalf("taken: %v", r)
	}
	sug := r["suggestions"].([]any)
	if len(sug) == 0 || len(sug) > 4 {
		t.Fatalf("suggestions: %v", sug)
	}
	for _, s := range sug {
		if s == b.slugA {
			t.Fatalf("a suggestion must not be the taken address: %v", sug)
		}
	}
	// every suggestion is really free: it can be saved
	other.expect(200, "PUT", "/api/agenda/settings", map[string]any{"slot_minutes": 30, "rooms": []string{}, "booking_enabled": false, "booking_slug": sug[0], "booking_lead_hours": 2, "booking_horizon_days": 30, "cancel_min_hours": 2, "remind_hours": []int{24}})
	// reserved and invalid ones explain themselves; the invalid one is cleaned into a variant
	if r := check(admin, "admin"); r["state"] != "reserved" || r["available"] != false {
		t.Fatalf("reserved: %v", r)
	}
	r = check(admin, "Clínica%20Ñandú!!")
	if r["state"] != "invalid" {
		t.Fatalf("invalid: %v", r)
	}
	if s := r["suggestions"].([]any); len(s) == 0 || s[0] != "clinica-nandu" {
		t.Fatalf("cleaned name first: %v", s)
	}
}
