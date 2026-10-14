package api

import (
	"testing"
	"time"
)

func TestBlogSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Cómo reducir inasistencias en tu clínica": "como-reducir-inasistencias-en-tu-clinica",
		"  ¿Qué es la NOM-004?  ":                  "que-es-la-nom-004",
		"Niños y señales: guía 2026!":              "ninos-y-senales-guia-2026",
		"¡¡!!":                                     "",
	} {
		if got := blogSlug(in); got != want {
			t.Errorf("blogSlug(%q) = %q, want %q", in, got, want)
		}
	}
	long := blogSlug("palabra palabra palabra palabra palabra palabra palabra palabra palabra palabra palabra palabra palabra")
	if len(long) > 80 || long[len(long)-1] == '-' {
		t.Errorf("long slug: %q", long)
	}
}

func TestBlogDays(t *testing.T) {
	mon := time.Date(2026, 10, 12, 10, 0, 0, 0, blogLoc) // lunes
	if !blogDay(mon) || !blogDay(mon.AddDate(0, 0, 3)) || blogDay(mon.AddDate(0, 0, 1)) || blogDay(mon.AddDate(0, 0, 6)) {
		t.Fatal("el blog sale los lunes y jueves")
	}
}
