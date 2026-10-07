package api

import (
	"testing"
	"time"
)

func arcoDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestArcoAddBusinessDays(t *testing.T) {
	cases := []struct {
		from string
		n    int
		want string
	}{
		{"2026-03-02", 1, "2026-03-03"},  // Monday + 1
		{"2026-03-06", 1, "2026-03-09"},  // Friday + 1 skips the weekend
		{"2026-03-07", 1, "2026-03-09"},  // Saturday + 1
		{"2026-03-08", 1, "2026-03-09"},  // Sunday + 1
		{"2026-03-02", 5, "2026-03-09"},  // a full week
		{"2026-03-02", 20, "2026-03-30"}, // four weeks
		{"2026-03-04", 20, "2026-04-01"},
		{"2026-03-02", 0, "2026-03-02"},
	}
	for _, c := range cases {
		if got := arcoAddBusinessDays(arcoDate(c.from), c.n).Format("2006-01-02"); got != c.want {
			t.Errorf("%s + %d = %s, want %s", c.from, c.n, got, c.want)
		}
	}
}

func TestArcoBusinessDaysLeft(t *testing.T) {
	cases := []struct {
		today, due string
		want       int
	}{
		{"2026-03-02", "2026-03-02", 0},
		{"2026-03-02", "2026-03-06", 4},
		{"2026-03-06", "2026-03-09", 1},
		{"2026-03-09", "2026-03-06", -1},
		{"2026-03-09", "2026-03-02", -5},
		{"2026-03-07", "2026-03-09", 1}, // from a Saturday
	}
	for _, c := range cases {
		if got := arcoBusinessDaysLeft(arcoDate(c.today), arcoDate(c.due)); got != c.want {
			t.Errorf("left(%s -> %s) = %d, want %d", c.today, c.due, got, c.want)
		}
	}
}

func TestArcoStateAndDeadlines(t *testing.T) {
	if arcoState(-1) != "overdue" || arcoState(0) != "soon" || arcoState(3) != "soon" || arcoState(4) != "ok" {
		t.Fatal("arcoState thresholds")
	}
	due := arcoDeadlines(arcoDate("2026-03-02"))
	if due.Answer.Format("2006-01-02") != "2026-03-30" || due.Ack.Format("2006-01-02") != "2026-03-05" {
		t.Fatalf("deadlines: %v", due)
	}
}
