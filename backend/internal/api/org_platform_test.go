package api_test

import "testing"

func platformRow(t *testing.T, root *client, id string) map[string]any {
	t.Helper()
	out := root.expect(200, "GET", "/api/platform/clinics", nil)
	rows, _ := out["clinics"].([]any)
	for _, r := range rows {
		if m, ok := r.(map[string]any); ok && m["id"] == id {
			return m
		}
	}
	t.Fatalf("clinic %s not listed", id)
	return nil
}

func TestPlatformShowsBranchesUnderTheirMatrix(t *testing.T) {
	e := orgSetup(t)
	owner := e.login("admin_a")
	root := e.login("root")
	north := orgNewBranch(t, owner, "Sucursal Norte")

	m := platformRow(t, root, e.clinicA)
	if m["is_matrix"] != true || m["branch_count"] != float64(1) {
		t.Fatalf("matrix row: %v", m)
	}
	b := platformRow(t, root, north)
	if b["matrix_id"] != e.clinicA || b["is_matrix"] != false {
		t.Fatalf("branch row: %v", b)
	}

	detail := root.expect(200, "GET", "/api/platform/clinics/"+e.clinicA, nil)
	list, _ := detail["branches"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["id"] != north {
		t.Fatalf("branches: %v", detail["branches"])
	}

	// suspended from the matrix panel → the platform panel shows it as such
	owner.expect(200, "POST", "/api/org/branches/"+north+"/suspend", nil)
	if s := platformRow(t, root, north)["state"]; s != "branch_suspended" {
		t.Fatalf("state after suspend = %v", s)
	}
	owner.expect(200, "POST", "/api/org/branches/"+north+"/reactivate", nil)
	if s := platformRow(t, root, north)["state"]; s == "branch_suspended" {
		t.Fatalf("still suspended after reactivate")
	}
}
