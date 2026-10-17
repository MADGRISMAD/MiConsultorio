package api_test

import "testing"

// The SQL rule of the trigger and the plan catalog must say the same: only the plans with branches keep them running.
func TestPlanAllowsBranchesMatchesTheCatalog(t *testing.T) {
	e := setup(t)
	plans := e.login("root").expect(200, "GET", "/api/platform/plans", nil)["plans"].([]any)
	if len(plans) == 0 {
		t.Fatal("no plans")
	}
	for _, p := range plans {
		pl := p.(map[string]any)
		var allows bool
		if err := e.pool.QueryRow(t.Context(), `SELECT plan_allows_branches($1)`, pl["id"]).Scan(&allows); err != nil {
			t.Fatal(err)
		}
		if want := pl["max_branches"].(float64) > 1; allows != want {
			t.Fatalf("plan %v: SQL says %v, the catalog max_branches=%v", pl["id"], allows, pl["max_branches"])
		}
	}
}

// When the matrix's plan stops including branches they go out of service by themselves and cannot be used; when a plan
// with branches returns, the ones the license took out come back, and the one the owner switched off stays off.
func TestBranchesFollowTheMatrixLicense(t *testing.T) {
	e := orgSetup(t)
	owner := e.login("admin_a")
	root := e.login("root")
	north := orgNewBranch(t, owner, "Sucursal Norte")
	south := orgNewBranch(t, owner, "Sucursal Sur")
	owner.expect(200, "POST", "/api/org/branches/"+south+"/suspend", nil) // the owner switched this one off

	by := func(id string) string {
		var s string
		if err := e.pool.QueryRow(t.Context(), `SELECT coalesce(branch_suspended_by, '') FROM clinics WHERE id = $1`, id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	if by(north) != "" || by(south) != "owner" {
		t.Fatalf("before: north=%q south=%q", by(north), by(south))
	}

	// the owner is working inside the north branch when the license drops
	orgSwitch(t, owner, north)
	owner.expect(200, "GET", "/api/appointments", nil)
	root.expect(200, "PATCH", "/api/platform/clinics/"+e.clinicA, map[string]any{"plan": "crecimiento", "billing_status": "active"})
	if by(north) != "license" || by(south) != "owner" {
		t.Fatalf("after the drop: north=%q south=%q", by(north), by(south))
	}
	out := owner.expect(403, "GET", "/api/appointments", nil)
	if out["code"] != "SUBSCRIPTION_REQUIRED" {
		t.Fatalf("a branch without license must be locked: %v", out)
	}
	if st := sub(root.expect(200, "GET", "/api/platform/clinics/"+north, nil), "clinic")["state"]; st != "branch_license" {
		t.Fatalf("platform state = %v", st)
	}
	// the matrix itself keeps working, and the owner cannot bring a branch back on a plan without branches
	owner.expect(200, "POST", "/api/org/switch", map[string]any{"branch_id": e.clinicA})
	owner.expect(200, "GET", "/api/appointments", nil)
	owner.expect(409, "POST", "/api/org/branches/"+north+"/reactivate", nil)

	// a plan with branches again: the license-suspended branch returns, the owner's stays off
	root.expect(200, "PATCH", "/api/platform/clinics/"+e.clinicA, map[string]any{"plan": "pro", "billing_status": "active"})
	if by(north) != "" || by(south) != "owner" {
		t.Fatalf("after the return: north=%q south=%q", by(north), by(south))
	}
	orgSwitch(t, owner, north)
	owner.expect(200, "GET", "/api/appointments", nil)
}

// Branches that were left running by an old plan change (the rows say Básico but nothing flagged them) are locked at
// once, and the sweep brings the flags in line, the same as the trigger would have.
func TestLegacyBranchesWithoutLicenseAreLocked(t *testing.T) {
	e := orgSetup(t)
	owner := e.login("admin_a")
	root := e.login("root")
	north := orgNewBranch(t, owner, "Sucursal Norte")
	south := orgNewBranch(t, owner, "Sucursal Sur")
	owner.expect(200, "POST", "/api/org/branches/"+south+"/suspend", nil)

	orgSwitch(t, owner, north) // the owner is working inside the branch

	// the plan changed behind the trigger's back, as in data that predates the rule
	e.exec(`ALTER TABLE clinics DISABLE TRIGGER USER`)
	e.exec(`UPDATE clinics SET plan = 'basico' WHERE organization_id IS NOT NULL`)
	e.exec(`ALTER TABLE clinics ENABLE TRIGGER USER`)
	flag := func(id string) string {
		var s string
		if err := e.pool.QueryRow(t.Context(), `SELECT coalesce(branch_suspended_by, '') FROM clinics WHERE id = $1`, id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	if flag(north) != "" {
		t.Fatalf("the test needs a branch that is not flagged yet")
	}

	// locked before any sweep: its session is refused, it cannot be entered, and the platform shows it without license
	if out := owner.expect(403, "GET", "/api/appointments", nil); out["code"] != "SUBSCRIPTION_REQUIRED" {
		t.Fatalf("an unflagged branch without license must be locked: %v", out)
	}
	if st := sub(root.expect(200, "GET", "/api/platform/clinics/"+north, nil), "clinic")["state"]; st != "branch_license" {
		t.Fatalf("platform state = %v", st)
	}
	owner.expect(200, "POST", "/api/org/switch", map[string]any{"branch_id": e.clinicA}) // out of the locked branch
	owner.expect(409, "POST", "/api/org/switch", map[string]any{"branch_id": north})     // and not back into it

	// the sweep sets the flags, leaves the owner's alone, and is idempotent
	var n int
	if err := e.pool.QueryRow(t.Context(), `SELECT reconcile_branch_licenses()`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("sweep changed %d (%v), want 1", n, err)
	}
	if flag(north) != "license" || flag(south) != "owner" {
		t.Fatalf("flags: north=%q south=%q", flag(north), flag(south))
	}
	if err := e.pool.QueryRow(t.Context(), `SELECT reconcile_branch_licenses()`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("second sweep changed %d (%v), want 0", n, err)
	}

	// and when the license returns by any route, the sweep gives the branch back
	e.exec(`ALTER TABLE clinics DISABLE TRIGGER USER`)
	e.exec(`UPDATE clinics SET plan = 'pro' WHERE organization_id IS NOT NULL`)
	e.exec(`ALTER TABLE clinics ENABLE TRIGGER USER`)
	if err := e.pool.QueryRow(t.Context(), `SELECT reconcile_branch_licenses()`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("restore sweep changed %d (%v), want 1", n, err)
	}
	if flag(north) != "" || flag(south) != "owner" {
		t.Fatalf("after restore: north=%q south=%q", flag(north), flag(south))
	}
}
