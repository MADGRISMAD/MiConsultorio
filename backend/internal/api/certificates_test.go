package api_test

import (
	"context"
	"regexp"
	"strings"
	"testing"
)

func TestCertificates(t *testing.T) {
	e := setup(t)
	doc, recep, anon := e.login("doc_a"), e.login("recep_a"), e.anon()
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	url := "/api/patients/" + pid + "/certificates"
	body := map[string]any{"purpose": "Escolar", "statement": "Certifico que el paciente se encuentra clínicamente sano.", "findings": "Exploración sin datos patológicos", "aptitude": "apto", "valid_days": 30}

	// the cédula is needed, like for a receta
	if code, o := doc.do("POST", url, body); code != 409 || o["code"] != "CEDULA_REQUIRED" {
		t.Fatalf("cédula: %d %v", code, o)
	}
	doc = rxDoctor(t, e, "doc_a")
	doc.expect(400, "POST", url, map[string]any{"purpose": "Magia", "statement": "x"})
	doc.expect(400, "POST", url, map[string]any{"purpose": "Escolar", "statement": ""})
	doc.expect(400, "POST", url, map[string]any{"purpose": "Escolar", "statement": "x", "aptitude": "quizá"})
	recep.expect(403, "POST", url, body)

	c := sub(doc.expect(201, "POST", url, body), "certificate")
	if c["kind"] != "medical" || c["folio"] != float64(1) || c["valid_until"] == nil || c["author_license"] != "12345678" {
		t.Fatalf("certificate: %v", c)
	}
	doc.expect(201, "POST", url, body)
	list := doc.expect(200, "GET", url, nil)["certificates"].([]any)
	if len(list) != 2 || list[0].(map[string]any)["statement"] == "" {
		t.Fatalf("list: %v", list)
	}
	// the text is sealed in the table
	if raw := e.scalarText(`SELECT statement FROM certificates WHERE id = $1`, c["id"]); strings.Contains(raw, "clínicamente sano") {
		t.Fatalf("statement is stored in clear: %s", raw)
	}

	// print data and the public check
	pd := doc.expect(200, "GET", "/api/certificates/"+c["id"].(string), nil)
	tok := regexp.MustCompile(`/verificar/(.+)$`).FindStringSubmatch(pd["verify_url"].(string))[1]
	v := anon.expect(200, "GET", "/api/public/rx/"+tok, nil)
	if v["status"] != "vigente" || v["document"] != "certificate_medical" || v["professional_license"] != "12345678" {
		t.Fatalf("verify: %v", v)
	}
	if _, ok := v["statement"]; ok {
		t.Fatalf("the check must not show the content: %v", v)
	}

	// voiding keeps the record and the check says so
	doc.expect(400, "POST", "/api/certificates/"+c["id"].(string)+"/void", map[string]any{"reason": ""})
	doc.expect(200, "POST", "/api/certificates/"+c["id"].(string)+"/void", map[string]any{"reason": "Error de captura"})
	doc.expect(409, "POST", "/api/certificates/"+c["id"].(string)+"/void", map[string]any{"reason": "otra vez"})
	if anon.expect(200, "GET", "/api/public/rx/"+tok, nil)["status"] != "anulada" {
		t.Fatal("voided")
	}
	// another clinic does not see it
	e.login("doc_b").expect(404, "GET", "/api/certificates/"+c["id"].(string), nil)

	// an animal gets the veterinary one
	var animal string
	if err := e.pool.QueryRow(context.Background(), `INSERT INTO patients (clinic_id, file_number, subject, names, guardian_name) VALUES ($1, 99, 'animal', 'Luna', 'Marta') RETURNING id`, e.clinicA).Scan(&animal); err != nil {
		t.Fatal(err)
	}
	vc := sub(doc.expect(201, "POST", "/api/patients/"+animal+"/certificates", map[string]any{"purpose": "Viaje nacional", "statement": "Animal clínicamente sano.", "destination": "Guadalajara", "microchip": "985112000123456"}), "certificate")
	if vc["kind"] != "veterinary" || vc["extra"].(map[string]any)["destination"] != "Guadalajara" {
		t.Fatalf("veterinary: %v", vc)
	}
	doc.expect(400, "POST", "/api/patients/"+animal+"/certificates", map[string]any{"purpose": "Escolar", "statement": "x"}) // a person's purpose
	doc.expect(400, "POST", "/api/patients/"+animal+"/certificates", map[string]any{"purpose": "Salud general", "statement": "x", "aptitude": "apto"})
}
