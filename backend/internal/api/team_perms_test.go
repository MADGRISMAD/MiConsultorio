package api_test

import (
	"testing"

	"github.com/madgrismad/miconsultorio/backend/internal/config"
)

func TestPerPersonPermissions(t *testing.T) {
	e := setup(t)
	admin, recep, cash := e.login("admin_a"), e.login("recep_a"), e.login("cash_a")
	ids := map[string]string{}
	for _, p := range admin.expect(200, "GET", "/api/team", nil)["people"].([]any) {
		m := p.(map[string]any)
		ids[m["username"].(string)] = m["id"].(string)
	}
	recep.expect(403, "GET", "/api/patients/", nil) // reception does not read clinical records

	patch := func(status int, who string, body map[string]any) map[string]any {
		return admin.expect(status, "PATCH", "/api/team/"+ids[who], body)
	}
	// invalid combinations
	patch(400, "recep_a", map[string]any{"permissions_extra": []string{"adminUsers"}})     // the team is never grantable
	patch(400, "recep_a", map[string]any{"permissions_extra": []string{"pos"}})            // the role already has it
	patch(400, "recep_a", map[string]any{"permissions_denied": []string{"navHistorials"}}) // the role does not have it
	patch(400, "admin_a", map[string]any{"permissions_denied": []string{"pos"}})           // administrators keep everything
	patch(400, "recep_a", map[string]any{"permissions_extra": []string{"magia"}})

	// grant: reading and writing clinical records (the basic one comes along)
	out := sub(patch(200, "recep_a", map[string]any{"permissions_extra": []string{"adminHistorials"}}), "person")
	if perms := out["permissions"].([]any); !containsAny(perms, "navHistorials") || !containsAny(perms, "adminHistorials") {
		t.Fatalf("effective permissions: %v", perms)
	}
	recep.expect(200, "GET", "/api/patients/", nil)

	// take away: a cashier who must not sell
	out = sub(patch(200, "cash_a", map[string]any{"permissions_denied": []string{"pos"}}), "person")
	if perms := out["permissions"].([]any); containsAny(perms, "pos") || containsAny(perms, "posReports") {
		t.Fatalf("taking pos takes the dependent ones too: %v", perms)
	}
	cash.expect(403, "GET", "/api/pos/sales?limit=1", nil)

	// a new role means the role's own permissions again
	out = sub(patch(200, "recep_a", map[string]any{"role": "cashier"}), "person")
	if len(out["permissions_extra"].([]any)) != 0 {
		t.Fatalf("overrides must reset with the role: %v", out)
	}
}

func containsAny(list []any, want string) bool {
	for _, x := range list {
		if x == want {
			return true
		}
	}
	return false
}

// Per-person permissions come with Crecimiento and Pro.
func TestPlanGates(t *testing.T) {
	e := setup(t)
	admin := e.login("admin_a")
	e.exec(`UPDATE clinics SET plan = 'basico' WHERE id = $1`, e.clinicA)
	var rid string
	for _, p := range admin.expect(200, "GET", "/api/team", nil)["people"].([]any) {
		if m := p.(map[string]any); m["username"] == "recep_a" {
			rid = m["id"].(string)
		}
	}
	if code, o := admin.do("PATCH", "/api/team/"+rid, map[string]any{"permissions_extra": []string{"adminHistorials"}}); code != 403 || o["code"] != "PLAN_REQUIRED" {
		t.Fatalf("permissions on Básico: %d %v", code, o)
	}
	e.exec(`UPDATE clinics SET plan = 'crecimiento' WHERE id = $1`, e.clinicA)
	admin.expect(200, "PATCH", "/api/team/"+rid, map[string]any{"permissions_extra": []string{"adminHistorials"}})
	// going back to Básico keeps what exists but allows no new overrides
	e.exec(`UPDATE clinics SET plan = 'basico' WHERE id = $1`, e.clinicA)
	admin.expect(200, "PATCH", "/api/team/"+rid, map[string]any{"permissions_extra": []string{"adminHistorials"}})
	admin.expect(200, "PATCH", "/api/team/"+rid, map[string]any{"permissions_extra": []string{}})
}

// Básico has no AI: the summary asks for a plan that includes it, and nothing is spent.
func TestBasicHasNoMagic(t *testing.T) {
	e := setupWith(t, func(c *config.Config) {
		c.GeminiAPIKey, c.GeminiModel, c.GeminiAPIBase = "k", "m", "http://127.0.0.1:1"
	})
	doc := e.login("doc_a")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	doc.expect(201, "POST", "/api/patients/"+pid+"/encounters", map[string]any{"reason": "Control"})
	e.exec(`UPDATE clinics SET plan = 'basico' WHERE id = $1`, e.clinicA)
	if code, o := doc.do("POST", "/api/patients/"+pid+"/ai-summary", map[string]any{}); code != 403 || o["code"] != "PLAN_REQUIRED" {
		t.Fatalf("no AI on Básico: %d %v", code, o)
	}
	if n := e.scalar(`SELECT count(*) FROM magic_usage`); n != 0 {
		t.Fatalf("nothing spent: %v", n)
	}
}
