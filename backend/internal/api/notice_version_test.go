package api_test

import "testing"

// Each acceptance of the privacy notice / terms is kept with the version of the texts that was in force.
func TestNoticeVersionIsRecorded(t *testing.T) {
	b := newBookingEnv(t, false)
	anon := b.anon()
	var svc string
	if err := b.pool.QueryRow(t.Context(), `SELECT id::text FROM catalog_items WHERE clinic_id = $1 AND name = 'Consulta general'`, b.clinicA).Scan(&svc); err != nil {
		t.Fatal(err)
	}
	code, out := anon.do("POST", "/api/public/booking/"+b.slugA+"/appointments", map[string]any{
		"professional_id": b.pro, "service_id": svc, "date": b.date, "start": "10:00", "names": "Ana", "last_names": "López",
		"email": "ana@x.mx", "phone": "5522222222", "reason": "Dolor", "accept_privacy": true, "accept_reminders": false,
	})
	if code != 201 {
		t.Fatalf("book: %d %v", code, out)
	}
	var version string
	if err := b.pool.QueryRow(t.Context(), `SELECT privacy_version FROM appointments WHERE confirm_token = $1`, sub(out, "appointment")["token"]).Scan(&version); err != nil || version == "" {
		t.Fatalf("booking privacy_version = %q (%v)", version, err)
	}

	anon.expect(200, "POST", "/api/register", map[string]any{
		"clinic_name": "Consultorio Versionado", "phone": "5511111111", "name": "Doctora Prueba", "email": "ver@x.mx",
		"username": "versionada", "password": "una-clave-larga-123", "accepted_terms": true,
	})
	var termsVersion string
	if err := b.pool.QueryRow(t.Context(), `SELECT terms_version FROM users WHERE username = 'versionada' AND terms_accepted_at IS NOT NULL`).Scan(&termsVersion); err != nil || termsVersion != version {
		t.Fatalf("terms_version = %q want %q (%v)", termsVersion, version, err)
	}
}
