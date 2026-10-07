package api

import (
	"math"
	"testing"
)

// All the numbers here are synthetic: they check the maths, they are not WHO or CDC reference values.

func growthNear(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Fatalf("%s: got %v want %v", name, got, want)
	}
}

func TestGrowthZLMS(t *testing.T) {
	// L=1: z = (x/M - 1) / S.
	z, ok := growthZ(11, 1, 10, 0.1)
	if !ok {
		t.Fatal("no z")
	}
	growthNear(t, "L=1", z, 1, 1e-12)
	// L=0: z = ln(x/M) / S.
	z, _ = growthZ(10*math.Exp(0.2), 0, 10, 0.1)
	growthNear(t, "L=0", z, 2, 1e-9)
	// L=2: z = ((x/M)^2 - 1) / (2S); x = M*sqrt(1 + 2*S*z).
	z, _ = growthZ(10*math.Sqrt(1+2*0.1*1.5), 2, 10, 0.1)
	growthNear(t, "L=2", z, 1.5, 1e-9)
	// L=-1: z = (M/x - 1) / (-S)
	z, _ = growthZ(8, -1, 10, 0.1)
	growthNear(t, "L=-1", z, -2.5, 1e-9)
	// The median has z = 0 for any L.
	for _, l := range []float64{-2, -0.5, 0, 0.7, 3} {
		z, _ = growthZ(25, l, 25, 0.12)
		growthNear(t, "median", z, 0, 1e-12)
	}
	// Round trip with the inverse.
	for _, l := range []float64{-1.5, 0, 0.3, 1, 2} {
		for _, want := range []float64{-2.5, -1, 0, 0.5, 1.88, 3} {
			x, ok := growthValueAtZ(want, l, 20, 0.09)
			if !ok {
				continue
			}
			z, _ = growthZ(x, l, 20, 0.09)
			growthNear(t, "round trip", z, want, 1e-9)
		}
	}
	// Invalid inputs give no score.
	for _, c := range [][4]float64{{0, 1, 10, 0.1}, {-1, 1, 10, 0.1}, {5, 1, 0, 0.1}, {5, 1, 10, 0}, {5, math.NaN(), 10, 0.1}} {
		if _, ok := growthZ(c[0], c[1], c[2], c[3]); ok {
			t.Fatalf("scored %v", c)
		}
	}
	// A z beyond the support of the transform has no value.
	if _, ok := growthValueAtZ(-50, 2, 10, 0.1); ok {
		t.Fatal("value for impossible z")
	}
}

func TestGrowthPercentile(t *testing.T) {
	growthNear(t, "z0", growthPercentile(0), 50, 1e-9)
	growthNear(t, "z1", growthPercentile(1), 84.1344746, 1e-5)
	growthNear(t, "z-1.96", growthPercentile(-1.96), 2.4997896, 1e-5)
	growthNear(t, "inv 97.5", growthZFromPercentile(97.5), 1.959964, 1e-5)
	growthNear(t, "inv 3", growthZFromPercentile(3), -1.880794, 1e-5)
	growthNear(t, "inv 50", growthZFromPercentile(50), 0, 1e-12)
	for _, p := range []float64{1, 3, 15, 50, 85, 97, 99} {
		growthNear(t, "round trip", growthPercentile(growthZFromPercentile(p)), p, 1e-9)
	}
}

func growthFP(v float64) *float64 { return &v }

func TestGrowthInterpAndScore(t *testing.T) {
	rows := []growthRef{
		{Age: 0, L: growthFP(1), M: growthFP(3), S: growthFP(0.1)},
		{Age: 12, L: growthFP(1), M: growthFP(9), S: growthFP(0.1)},
	}
	if _, ok := growthInterp(rows, -1); ok {
		t.Fatal("extrapolated below")
	}
	if _, ok := growthInterp(rows, 12.01); ok {
		t.Fatal("extrapolated above")
	}
	mid, ok := growthInterp(rows, 6)
	if !ok || *mid.M != 6 || *mid.L != 1 {
		t.Fatalf("interp: %+v", mid)
	}
	end, _ := growthInterp(rows, 12)
	if *end.M != 9 {
		t.Fatalf("exact age: %+v", end)
	}
	z, pct, ok := growthScore(mid, 6.6)
	if !ok {
		t.Fatal("no score")
	}
	growthNear(t, "z", z, 1, 1e-9)
	growthNear(t, "pct", pct, 84.1344746, 1e-5)

	// Percentile-only rows.
	pr := growthRef{Age: 0, Pcts: map[int]float64{3: 45, 50: 50, 97: 55}}
	z, pct, ok = growthScore(pr, 50)
	if !ok || pct != 50 || z != 0 {
		t.Fatalf("p50: %v %v %v", z, pct, ok)
	}
	_, pct, _ = growthScore(pr, 52.5)
	growthNear(t, "between p50 and p97", pct, 50+2.5/5*47, 1e-9)
	if _, _, ok := growthScore(pr, 44); ok {
		t.Fatal("scored below the table's own range")
	}
	if _, _, ok := growthScore(pr, 56); ok {
		t.Fatal("scored above the table's own range")
	}
}

func TestParseGrowthCSV(t *testing.T) {
	ok := parseGrowthCSV("Indicator, Sex ,age_months,L,M,S,p3,p50\nweight_for_age,m,0,1,3,0.1,2.5,3\nweight_for_age,M,1,1,4,0.1,,\n\n")
	if ok.Total != 0 || len(ok.Rows) != 2 || !ok.HasLMS {
		t.Fatalf("valid: %+v", ok)
	}
	if ok.Rows[0].Sex != "M" || ok.Rows[0].Pcts[3] != 2.5 || len(ok.Rows[1].Pcts) != 0 || *ok.Rows[1].M != 4 {
		t.Fatalf("rows: %+v", ok.Rows)
	}
	many := "indicator,sex,age_months,l,m,s\n"
	for i := 0; i < 40; i++ {
		many += "bad,M,0,1,3,0.1\n"
	}
	r := parseGrowthCSV(many)
	if r.Total != 40 || len(r.Errors) != growthMaxListedErrors || r.Errors[0].Line != 2 {
		t.Fatalf("error listing: total %d listed %d first line %d", r.Total, len(r.Errors), r.Errors[0].Line)
	}
	if r := parseGrowthCSV(""); r.Total == 0 {
		t.Fatal("empty file accepted")
	}
	if r := parseGrowthCSV("indicator,sex,age_months,l,m,s\nweight_for_age,M,0,1,3,\"0.1\n"); r.Total == 0 {
		t.Fatal("broken quoting accepted")
	}
}

func TestLabFlag(t *testing.T) {
	lo, hi := growthFP(10), growthFP(20)
	cases := []struct {
		v       float64
		lo, hi  *float64
		want    string
		comment string
	}{
		{15, lo, hi, "normal", "inside"},
		{10, lo, hi, "normal", "lower edge"},
		{20, lo, hi, "normal", "upper edge"},
		{9.99, lo, hi, "bajo", "below"},
		{20.01, lo, hi, "alto", "above"},
		{250, nil, hi, "alto", "only upper"},
		{5, nil, hi, "normal", "only upper, inside"},
		{5, lo, nil, "bajo", "only lower"},
		{50, lo, nil, "normal", "only lower, inside"},
		{50, nil, nil, "na", "no range"},
	}
	for _, c := range cases {
		if got := labFlag(c.v, c.lo, c.hi); got != c.want {
			t.Fatalf("%s: got %s want %s", c.comment, got, c.want)
		}
	}
}

func TestLabCatalogIntegrity(t *testing.T) {
	ids := map[string]bool{}
	for _, p := range labCatalogPanels {
		if ids[p.ID] || p.Name == "" || len(p.Analytes) == 0 {
			t.Fatalf("panel %q", p.ID)
		}
		ids[p.ID] = true
		seen := map[string]bool{}
		for _, a := range p.Analytes {
			if seen[a.Name] || a.Name == "" {
				t.Fatalf("%s: analyte %q repeated or empty", p.ID, a.Name)
			}
			seen[a.Name] = true
			for _, b := range []*labBound{a.Ref, a.RefMale, a.RefFemale} {
				if b != nil && b.Low != nil && b.High != nil && *b.Low > *b.High {
					t.Fatalf("%s/%s: inverted range", p.ID, a.Name)
				}
			}
			if a.Kind == "num" && a.Ref == nil && (a.RefMale == nil || a.RefFemale == nil) && a.Note == "" {
				t.Fatalf("%s/%s: numeric analyte without range", p.ID, a.Name)
			}
		}
	}
	lo, hi, unit, ok := labCatalogRef("bh", "Hemoglobina", "Mujer")
	if !ok || *lo != 12.0 || *hi != 15.5 || unit != "g/dL" {
		t.Fatalf("female hemoglobin: %v %v %s", lo, hi, unit)
	}
	if lo, hi, _, _ := labCatalogRef("bh", "Hemoglobina", ""); lo != nil || hi != nil {
		t.Fatal("a sex-specific range was guessed without the sex")
	}
	if _, _, _, ok := labCatalogRef("bh", "Inventado", "Mujer"); ok {
		t.Fatal("unknown analyte found")
	}
}
