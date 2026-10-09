package api_test

import "testing"

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
