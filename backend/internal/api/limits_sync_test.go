package api_test

import (
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/madgrismad/miconsultorio/backend/internal/api"
	"github.com/madgrismad/miconsultorio/backend/internal/db"
)

// The forms of the frontend repeat a few limits the server enforces. They live in frontend/src/lib/limits.ts and this test keeps
// them equal to the backend constants, so changing one side alone fails here.
func TestFrontendLimitsMatchTheServer(t *testing.T) {
	src, err := os.ReadFile("../../../frontend/src/lib/limits.ts")
	if err != nil {
		t.Fatal(err)
	}
	read := func(name string) int {
		m := regexp.MustCompile(`export const ` + name + ` = (\d+);`).FindSubmatch(src)
		if m == nil {
			t.Fatalf("%s is not defined in limits.ts", name)
		}
		n, _ := strconv.Atoi(string(m[1]))
		return n
	}
	for name, want := range map[string]int{
		"PASSWORD_MIN":   db.PasswordMinRunes,
		"PASSWORD_MAX":   db.PasswordMaxBytes,
		"MAX_HIGHLIGHTS": api.MaxHighlights,
		"MAX_FAQS":       api.MaxFAQs,
	} {
		if got := read(name); got != want {
			t.Errorf("%s is %d in the frontend and %d in the server", name, got, want)
		}
	}
}
