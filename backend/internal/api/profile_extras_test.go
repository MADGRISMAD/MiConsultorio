package api_test

import "testing"

// The public page carries featured services with a description, FAQs, reasons to choose the clinic, insurance and social links.
func TestPublicPageExtras(t *testing.T) {
	b := newBookingEnv(t, false)
	admin, anon := b.login("admin_a"), b.anon()
	base := map[string]any{"enabled": true, "survey_delay_hours": 3, "maps_min_rating": 4, "show_reviews": true}
	with := func(extra map[string]any) map[string]any {
		m := map[string]any{}
		for k, v := range base {
			m[k] = v
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	svc := ""
	for _, x := range admin.expect(200, "GET", "/api/clinic/profile", nil)["services"].([]any) {
		if x.(map[string]any)["name"] == "Consulta general" {
			svc = x.(map[string]any)["id"].(string)
		}
	}

	// validation
	admin.expect(400, "PUT", "/api/clinic/profile", with(map[string]any{"faqs": []any{map[string]any{"q": "¿Duele?", "a": ""}}}))
	admin.expect(400, "PUT", "/api/clinic/profile", with(map[string]any{"highlights": []string{"a", "b", "c", "d", "e", "f", "g"}}))
	admin.expect(400, "PUT", "/api/clinic/profile", with(map[string]any{"instagram_url": "https://evil.example/clinica"}))
	admin.expect(400, "PUT", "/api/clinic/profile", with(map[string]any{"facebook_url": "no es un usuario!"}))
	admin.expect(400, "PUT", "/api/clinic/profile", with(map[string]any{"service_details": []any{map[string]any{"id": svc, "description": string(make([]byte, 0)) + repeat("x", 201)}}}))

	admin.expect(200, "PUT", "/api/clinic/profile", with(map[string]any{
		"tagline": "Cuidamos tu sonrisa", "whatsapp": "5215512345678",
		"accepts_insurance": true, "insurances": []string{"GNP", "AXA"},
		"highlights":    []string{"Equipo especializado", "Ambiente moderno", "Equipo especializado"},
		"faqs":          []any{map[string]any{"q": "¿Duele?", "a": "Usamos anestesia."}, map[string]any{"q": "", "a": ""}},
		"instagram_url": "@clinica.sonrisa", "facebook_url": "https://www.facebook.com/clinicasonrisa",
		"public_services": []string{svc},
		"service_details": []any{map[string]any{"id": svc, "description": "Una sola cita para salir con diagnóstico y presupuesto", "featured": true}},
	}))
	page := anon.expect(200, "GET", "/api/public/clinic/"+b.slugA, nil)
	if page["accepts_insurance"] != true || len(page["insurances"].([]any)) != 2 {
		t.Fatalf("insurance: %v", page)
	}
	if h := page["highlights"].([]any); len(h) != 2 {
		t.Fatalf("highlights are deduplicated: %v", h)
	}
	if f := page["faqs"].([]any); len(f) != 1 || f[0].(map[string]any)["q"] != "¿Duele?" {
		t.Fatalf("empty faqs are dropped: %v", f)
	}
	if page["instagram_url"] != "https://www.instagram.com/clinica.sonrisa" || page["facebook_url"] != "https://www.facebook.com/clinicasonrisa" {
		t.Fatalf("social links: %v %v", page["instagram_url"], page["facebook_url"])
	}
	found := false
	for _, s := range page["services"].([]any) {
		m := s.(map[string]any)
		if m["id"] == svc {
			found = m["featured"] == true && m["description"] == "Una sola cita para salir con diagnóstico y presupuesto"
		}
	}
	if !found {
		t.Fatalf("featured service with its description: %v", page["services"])
	}

	// no insurance: the list is not kept nor shown
	admin.expect(200, "PUT", "/api/clinic/profile", with(map[string]any{"accepts_insurance": false, "insurances": []string{"GNP"}, "public_services": []string{svc}}))
	page = anon.expect(200, "GET", "/api/public/clinic/"+b.slugA, nil)
	if page["accepts_insurance"] != false || len(page["insurances"].([]any)) != 0 {
		t.Fatalf("without insurance there is no list: %v", page)
	}
	// at most four featured services
	var five []any
	for i := 0; i < 5; i++ {
		five = append(five, map[string]any{"id": svc, "description": "x", "featured": true})
	}
	admin.expect(400, "PUT", "/api/clinic/profile", with(map[string]any{"service_details": five}))
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
