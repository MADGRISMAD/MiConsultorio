package api_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestEncryptedColumnsOnlyThroughHelpers walks the source of the API and fails when SQL touches a sealed column
// (encounters.subjective/exam/assessment/plan/notes, prescriptions.diagnosis/instructions, patients.profile,
// appointments.details) from a file that has not been reviewed. A new query on those columns must go through
// encField/decField/encProfile/decProfile (or the scan helpers that call them) and its file be added below.
func TestEncryptedColumnsOnlyThroughHelpers(t *testing.T) {
	// Files that read or write sealed columns: each one must call at least one helper.
	helperFiles := map[string]bool{
		"appointments.go": true, "booking_online.go": true, "encounters.go": true, "patients.go": true, "patient_merge.go": true,
		"portal_data.go": true, "nutrition_ai.go": true, "followup.go": true, "recalls.go": true, "prescriptions.go": true, "vaccinations.go": true, "waitlist_public_routes.go": true, "lab.go": true,
	}
	// Files that mention one of the words for a different table or column (clinics.plan, medications.notes,
	// treatment_plans.notes...) or only read the clear profile key; reviewed by hand.
	otherTables := map[string]bool{
		"middleware.go": true, "mp.go": true, "plans.go": true, "platform.go": true, "team.go": true,
		"rx_catalog.go": true, "rx_routes.go": true, "treatment_plans.go": true, "reports_clinical.go": true, "waitlist.go": true,
		"org_core.go": true, "org_reports.go": true, "org_routes.go": true, // read clinics.plan only
	}
	// Files allowed to export whole rows (to_jsonb / SELECT *) of the sealed tables.
	wholeRow := map[string]bool{"patients_export.go": true, "vaccinations.go": true}
	helperTokens := []string{"encField(", "decField(", "encFields(", "decFields(", "encProfile(", "decProfile(", "scanEncounter(", "scanRx(", "scanPatient(",
		"loadAppointment(", "openAppointmentDetails(", "openJSONColumn("}

	words := regexp.MustCompile(`(?i)\b(subjective|exam|assessment|plan|notes|diagnosis|instructions|profile|details)\b`)
	tables := regexp.MustCompile(`(?i)\b(encounters|prescriptions|appointments|patients)\b`)
	whole := regexp.MustCompile(`(?i)to_jsonb\(|row_to_json\(|select\s+\*`)
	clearProfile := regexp.MustCompile(`(?i)profile\s*->>\s*'species'`)

	files, _ := filepath.Glob("*.go")
	sort.Strings(files)
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") || name == "encfields.go" {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		touches, uses := false, false
		for _, tok := range helperTokens {
			if strings.Contains(string(src), tok) {
				uses = true
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING || !strings.HasPrefix(lit.Value, "`") {
				return true
			}
			sql := lit.Value
			if whole.MatchString(sql) && tables.MatchString(sql) && !wholeRow[name] {
				t.Errorf("%s: %s dumps whole rows of a table with sealed columns; open them with the helpers and list the file in wholeRow",
					name, fset.Position(lit.Pos()))
			}
			if !words.MatchString(sql) {
				return true
			}
			// "profile" alone is fine when only the clear species key is read.
			rest := clearProfile.ReplaceAllString(sql, "")
			if !words.MatchString(rest) {
				return true
			}
			touches = true
			if otherTables[name] && tables.MatchString(rest) && words.FindString(rest) != "plan" && !strings.Contains(strings.ToLower(rest), "treatment_plans") &&
				!strings.Contains(strings.ToLower(rest), "clinic_medications") {
				t.Errorf("%s: %s names a sealed table next to %q in a file reviewed for other tables", name, fset.Position(lit.Pos()), words.FindString(rest))
			}
			return true
		})
		switch {
		case touches && !helperFiles[name] && !otherTables[name]:
			t.Errorf("%s reads or writes a sealed column (or a column with the same name) without being reviewed: use encField/decField/encProfile/decProfile and list it in helperFiles (or in otherTables when it is another table)", name)
		case helperFiles[name] && !uses:
			t.Errorf("%s is listed in helperFiles but calls none of the helpers", name)
		}
	}
}
