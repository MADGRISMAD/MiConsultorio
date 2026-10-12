package api_test

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestPublicDirectory(t *testing.T) {
	b := newBookingEnv(t, false)
	ctx := context.Background()
	admin := b.login("admin_a")
	anon := b.anon()
	b.exec(`UPDATE clinics SET kind = 'DENTAL' WHERE id = $1`, b.clinicA)
	search := func(q string) map[string]any { return anon.expect(200, "GET", "/api/public/directory?"+q, nil) }
	total := func(q string) float64 { return search(q)["total"].(float64) }

	// todos entran solos, con el perfil vacío; sin página no hay enlace ni nada publicado
	empty := search("")
	if empty["total"].(float64) != 2 {
		t.Fatalf("todos los consultorios con suscripción vigente aparecen sin pedirlo: %v", empty)
	}
	for _, r := range empty["results"].([]any) {
		h := r.(map[string]any)
		if h["has_page"] != false || h["slug"] != "" || h["completeness"].(float64) >= 20 {
			t.Fatalf("sin perfil: tarjeta informativa, sin enlace y con poco puntaje: %v", h)
		}
	}

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
	// validaciones
	admin.expect(400, "PUT", "/api/clinic/profile", with(map[string]any{"city": "Tijuana", "state": "Texas"}))    // estado inexistente
	admin.expect(400, "PUT", "/api/clinic/profile", with(map[string]any{"payment_methods": []string{"Bitcoin"}})) // forma de pago fuera de la lista

	svcID := ""
	for _, x := range admin.expect(200, "GET", "/api/clinic/profile", nil)["services"].([]any) {
		if x.(map[string]any)["name"] == "Consulta general" {
			svcID = x.(map[string]any)["id"].(string)
		}
	}
	if svcID == "" {
		t.Fatal("el editor lista los servicios del catálogo")
	}
	admin.expect(200, "PUT", "/api/clinic/profile", with(map[string]any{
		"city": "Tijuana", "state": "Baja California", "neighborhood": "Zona Río", "tagline": "Cuidamos tu sonrisa",
		"insurances": []string{"GNP", "AXA", " GNP "}, "languages": []string{"Español", "Inglés"}, "payment_methods": []string{"Efectivo", "Tarjeta de crédito"},
		"public_services": []string{svcID},
	}))
	// otra clínica con la página y la ciudad, pero nada más
	b.exec(`UPDATE agenda_settings SET booking_enabled = true WHERE clinic_id = $1`, b.clinicB)
	b.exec(`INSERT INTO clinic_profile (clinic_id, enabled, city, state) VALUES ($1, true, 'Tijuana', 'Baja California')`, b.clinicB)

	// la que llenó todo sube; la que tiene datos faltantes baja, aunque ambas salgan
	res := search("")
	if res["total"].(float64) != 2 {
		t.Fatalf("salen las dos: %v", res)
	}
	hits := res["results"].([]any)
	first, second := hits[0].(map[string]any), hits[1].(map[string]any)
	if first["slug"] != "clinica-a" || second["slug"] != "clinica-b" || first["completeness"].(float64) <= second["completeness"].(float64) {
		t.Fatalf("la más completa va arriba: %v", hits)
	}
	if second["completeness"].(float64) != 45 || second["has_page"] != true {
		t.Fatalf("página + reservas + ubicación = 45 puntos: %v", second)
	}
	// un perfil más completo sube sin importar el nombre; cada dato que falta la baja
	b.exec(`UPDATE clinic_profile SET tagline = 'Hola', whatsapp = '5215512345678', hours_text = 'L-V' WHERE clinic_id = $1`, b.clinicB)
	if got := search("")["results"].([]any)[1].(map[string]any)["completeness"].(float64); got != 58 {
		t.Fatalf("más datos, más puntos: %v", got)
	}
	// el consultorio ve el mismo puntaje y qué le falta para subir
	cmp := admin.expect(200, "GET", "/api/clinic/profile", nil)["completeness"].(map[string]any)
	if cmp["score"].(float64) != first["completeness"].(float64) {
		t.Fatalf("el puntaje del editor debe ser el del directorio: %v vs %v", cmp["score"], first["completeness"])
	}
	pending := 0
	for _, it := range cmp["items"].([]any) {
		if it.(map[string]any)["done"] == false {
			pending++
		}
	}
	if pending == 0 || len(cmp["items"].([]any)) < 10 {
		t.Fatalf("la lista dice qué falta: %v", cmp)
	}
	hit := first
	if hit["slug"] != "clinica-a" || hit["city"] != "Tijuana" || hit["price_from_cents"].(float64) != 50000 || !strings.Contains(strings.Join(toStrings(hit["areas"]), ","), "Odontología") {
		t.Fatalf("resultado: %v", hit)
	}
	if ins := toStrings(hit["insurances"]); len(ins) != 2 {
		t.Fatalf("aseguradoras sin repetidos: %v", ins)
	}
	if ns, ok := hit["next_slot"].(map[string]any); !ok || ns["date"] == "" || ns["professional"] == "" {
		t.Fatalf("el resultado trae el próximo horario libre: %v", hit["next_slot"])
	}

	// filtros
	for q, want := range map[string]float64{
		"area=DENTAL": 1, "area=PSYCHOLOGY": 0, "city=tijuana": 2, "city=Tijuana": 2, "city=ensenada": 0,
		"state=" + url.QueryEscape("Baja California"): 2, "state=Sonora": 0, "q=sonrisa": 1, "q=DOC_A": 1, "q=zzz": 0, "q=" + url.QueryEscape("%"): 0,
		"area=DENTAL&city=tijuana&q=consult": 0,
	} {
		if got := total(q); got != want {
			t.Fatalf("filtro %q: %v, se esperaba %v", q, got, want)
		}
	}

	// opciones para los filtros y el mapa del sitio
	opt := anon.expect(200, "GET", "/api/public/directory/options", nil)
	cities := opt["cities"].([]any)
	if len(cities) != 1 || cities[0].(map[string]any)["slug"] != "tijuana" || cities[0].(map[string]any)["count"].(float64) != 2 || len(opt["states"].([]any)) != 32 {
		t.Fatalf("opciones: %v", opt)
	}
	res2, err := http.Get(b.srv.URL + "/api/public/sitemap.xml")
	if err != nil {
		t.Fatal(err)
	}
	xmlBody, _ := io.ReadAll(res2.Body)
	res2.Body.Close()
	for _, want := range []string{"http://app.test/directorio", "http://app.test/directorio/dentistas/tijuana", "http://app.test/clinica-a"} {
		if !strings.Contains(string(xmlBody), "<loc>"+want+"</loc>") {
			t.Fatalf("el mapa del sitio debe incluir %s:\n%s", want, xmlBody)
		}
	}
	// quien se oculta sale del directorio y del mapa del sitio
	b.login("admin_b").expect(200, "PUT", "/api/clinic/profile", map[string]any{"enabled": true, "survey_delay_hours": 3, "maps_min_rating": 4, "show_reviews": true, "city": "Tijuana", "state": "Baja California", "directory_hidden": true})
	if total("") != 1 {
		t.Fatal("una clínica oculta no aparece")
	}
	res3, err := http.Get(b.srv.URL + "/api/public/sitemap.xml")
	if err != nil {
		t.Fatal(err)
	}
	xmlBody, _ = io.ReadAll(res3.Body)
	res3.Body.Close()
	if strings.Contains(string(xmlBody), "clinica-b") {
		t.Fatal("el mapa del sitio no incluye clínicas ocultas")
	}

	// perfil completo: ubicación, aseguradoras, servicios con precio, cédula y semblanza
	admin.expect(200, "PUT", "/api/clinic/profile/professionals/"+b.pro, map[string]any{"hidden": false, "bio": "Egresado de la UNAM, 10 años de experiencia"})
	b.exec(`UPDATE users SET cedula = '12345678' WHERE id = $1`, b.pro)
	page := anon.expect(200, "GET", "/api/public/clinic/"+b.slugA, nil)
	svcs := page["services"].([]any)
	if page["city"] != "Tijuana" || page["neighborhood"] != "Zona Río" || len(svcs) != 1 || svcs[0].(map[string]any)["price_cents"].(float64) != 50000 ||
		strings.Join(toStrings(page["payment_methods"]), ",") != "Efectivo,Tarjeta de crédito" {
		t.Fatalf("perfil público: %v", page)
	}
	var doc map[string]any
	for _, p := range page["professionals"].([]any) {
		if p.(map[string]any)["cedula"] == "12345678" {
			doc = p.(map[string]any)
		}
	}
	if doc == nil || doc["bio"] != "Egresado de la UNAM, 10 años de experiencia" {
		t.Fatalf("profesional con cédula y semblanza: %v", page["professionals"])
	}

	// opinión verificada (de una cita real) y respuesta del consultorio
	var appt, survey string
	if err := b.pool.QueryRow(ctx, `INSERT INTO appointments (clinic_id, curp, names, last_names, date, start_hour, end_hour, professional_id, status)
		VALUES ($1, 'X', 'Ana', 'L', current_date, '09:00', '09:30', $2, 'completed') RETURNING id`, b.clinicA, b.pro).Scan(&appt); err != nil {
		t.Fatal(err)
	}
	if err := b.pool.QueryRow(ctx, `INSERT INTO satisfaction_surveys (clinic_id, appointment_id, professional_id, token, sent_at, answered_at, rating, comment, public_ok)
		VALUES ($1, $2, $3, 'tok-dir', now(), now(), 5, 'Excelente atención', true) RETURNING id`, b.clinicA, appt, b.pro).Scan(&survey); err != nil {
		t.Fatal(err)
	}
	if code := status(b.login("doc_a"), "PUT", "/api/clinic/surveys/"+survey+"/reply"); code != 403 {
		t.Fatalf("solo el administrador responde: %d", code)
	}
	b.login("admin_b").expect(404, "PUT", "/api/clinic/surveys/"+survey+"/reply", map[string]any{"reply": "x"})
	admin.expect(400, "PUT", "/api/clinic/surveys/"+survey+"/reply", map[string]any{"reply": strings.Repeat("a", 601)})
	admin.expect(200, "PUT", "/api/clinic/surveys/"+survey+"/reply", map[string]any{"reply": "¡Gracias por tu confianza!"})
	rev := anon.expect(200, "GET", "/api/public/clinic/"+b.slugA, nil)["reviews"].([]any)
	if len(rev) != 1 || rev[0].(map[string]any)["reply"] != "¡Gracias por tu confianza!" || rev[0].(map[string]any)["verified"] != true {
		t.Fatalf("opinión con respuesta: %v", rev)
	}
	if r := search("")["results"].([]any)[0].(map[string]any)["rating"].(map[string]any); r["count"].(float64) != 1 || r["average"].(float64) != 5 {
		t.Fatalf("la calificación sale en el directorio: %v", r)
	}

	// con la suscripción suspendida desaparece del directorio
	b.exec(`UPDATE clinics SET billing_status = 'suspended' WHERE id = $1`, b.clinicA)
	if total("") != 0 {
		t.Fatal("un consultorio suspendido no aparece")
	}
	b.exec(`UPDATE clinics SET billing_status = 'active' WHERE id = $1`, b.clinicA)
	if total("") != 1 {
		t.Fatal("al reactivar la suscripción vuelve")
	}
}

func toStrings(v any) []string {
	out := []string{}
	if list, ok := v.([]any); ok {
		for _, x := range list {
			out = append(out, x.(string))
		}
	}
	return out
}
