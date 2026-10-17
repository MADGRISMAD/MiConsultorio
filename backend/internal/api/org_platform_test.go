package api_test

import "testing"

func platformList(t *testing.T, root *client) (map[string]map[string]any, map[string]any) {
	t.Helper()
	out := root.expect(200, "GET", "/api/platform/clinics", nil)
	rows := map[string]map[string]any{}
	for _, r := range out["clinics"].([]any) {
		m := r.(map[string]any)
		rows[m["id"].(string)] = m
	}
	return rows, out["counts"].(map[string]any)
}

// The branches are not clients of their own (they are billed through the matrix): the general list, the counts and the
// revenue leave them out, the matrix says how many it has, and its profile lists them.
func TestPlatformKeepsBranchesInsideTheirMatrix(t *testing.T) {
	e := orgSetup(t)
	owner := e.login("admin_a")
	root := e.login("root")
	_, before := platformList(t, root)
	north := orgNewBranch(t, owner, "Sucursal Norte")
	south := orgNewBranch(t, owner, "Sucursal Sur")

	rows, counts := platformList(t, root)
	if _, ok := rows[north]; ok {
		t.Fatalf("a branch is listed on its own")
	}
	if _, ok := rows[south]; ok {
		t.Fatalf("a branch is listed on its own")
	}
	m := rows[e.clinicA]
	if m["is_matrix"] != true || m["branch_count"] != float64(2) {
		t.Fatalf("matrix row: %v", m)
	}
	if counts["all"] != before["all"] {
		t.Fatalf("branches change the total: %v -> %v", before["all"], counts["all"])
	}

	// the matrix is found by the name of one of its branches
	found := root.expect(200, "GET", "/api/platform/clinics?q=Sucursal%20Norte", nil)["clinics"].([]any)
	if len(found) != 1 || found[0].(map[string]any)["id"] != e.clinicA {
		t.Fatalf("search by branch name: %v", found)
	}

	// its profile lists them, with the state of each one
	owner.expect(200, "POST", "/api/org/branches/"+north+"/suspend", nil)
	detail := root.expect(200, "GET", "/api/platform/clinics/"+e.clinicA, nil)
	list, _ := detail["branches"].([]any)
	if len(list) != 2 {
		t.Fatalf("branches: %v", detail["branches"])
	}
	states := map[string]any{}
	for _, b := range list {
		states[b.(map[string]any)["id"].(string)] = b.(map[string]any)["state"]
	}
	if states[north] != "branch_suspended" || states[south] == "branch_suspended" {
		t.Fatalf("states: %v", states)
	}

	// each branch is still reachable by its id, and says which matrix it hangs from
	b := root.expect(200, "GET", "/api/platform/clinics/"+north, nil)
	if sub(b, "clinic")["matrix_id"] != e.clinicA {
		t.Fatalf("branch detail: %v", b["clinic"])
	}

	// the overview does not count them either
	ov := root.expect(200, "GET", "/api/platform/overview", nil)
	if sub(ov, "counts")["total"] != float64(before["all"].(float64)) {
		t.Fatalf("overview total: %v (before %v)", sub(ov, "counts")["total"], before["all"])
	}
}
