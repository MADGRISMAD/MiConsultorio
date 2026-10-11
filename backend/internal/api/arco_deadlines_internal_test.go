package api

import (
	"testing"
	"time"
)

// Un plazo vencido nunca debe verse «por vencer», aunque entre la fecha límite y hoy solo haya fin de semana.
func TestArcoOverdueOverWeekend(t *testing.T) {
	day := func(s string) time.Time { d, _ := time.Parse("2006-01-02", s); return d }
	due := day("2026-10-09") // viernes
	for _, today := range []string{"2026-10-10", "2026-10-11", "2026-10-12"} {
		if got := arcoState(arcoBusinessDaysLeft(day(today), due)); got != "overdue" {
			t.Errorf("hoy %s, vencía el viernes 9: debe estar vencida, salió %q", today, got)
		}
	}
	if got := arcoState(arcoBusinessDaysLeft(day("2026-10-09"), due)); got != "soon" {
		t.Errorf("el mismo día del plazo sigue siendo «por vencer»: %q", got)
	}
}
