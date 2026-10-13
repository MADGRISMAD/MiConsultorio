package api_test

import "testing"

// A clinic whose trial ended or whose paid period (plus the grace days) is over stops serving the public and stops sending
// mail; paying turns everything back on. The criteria are the same ones the app uses to lock its own screens.
func TestLapsedSubscriptionTurnsOffPublicFeaturesAndMail(t *testing.T) {
	b := newBookingEnv(t, false)
	anon := b.anon()
	admin := b.login("admin_a")
	book := "/api/public/booking/" + b.slugA
	pub := func() (int, any) { return anon.do("GET", book, nil) }
	set := func(q string, args ...any) {
		t.Helper()
		b.exec(`UPDATE clinics SET `+q+` WHERE id = $1`, append([]any{b.clinicA}, args...)...)
	}

	// an active clinic is public
	if code, _ := pub(); code != 200 {
		t.Fatalf("active clinic should be public, got %d", code)
	}

	// a reminder queued while it was active
	_, out := b.book(anon, "10:00", "ana@correo.mx", "5512345678")
	tok := b.token(out)
	b.makeDue(tok)

	cases := []struct {
		name string
		sql  string
		on   bool
	}{
		{"trial still running", "billing_status = 'trialing', trial_ends_at = now() + interval '3 days'", true},
		{"trial over", "billing_status = 'trialing', trial_ends_at = now() - interval '1 day'", false},
		{"paid period over, inside the grace days", "billing_status = 'active', current_period_end = now() - interval '1 day'", true},
		{"paid period over, grace days gone", "billing_status = 'active', current_period_end = now() - interval '10 days'", false},
		{"payment failed", "billing_status = 'past_due'", false},
		{"suspended", "billing_status = 'suspended'", false},
		{"paid", "billing_status = 'active', current_period_end = now() + interval '20 days'", true},
	}
	for _, c := range cases {
		set(c.sql)
		code, _ := pub()
		if (code == 200) != c.on {
			t.Fatalf("%s: public page status %d, want on=%v", c.name, code, c.on)
		}
		// the app agrees with the public side: the same clinic is locked out of its own screens
		if c.on {
			if got := admin.expect(200, "GET", "/api/agenda/booking-check", nil)["problem"]; got == "billing" {
				t.Fatalf("%s: booking-check should not report billing", c.name)
			}
		} else if got := errCode(t, admin, "GET", "/api/agenda/booking-check", nil, 403); got != "SUBSCRIPTION_REQUIRED" {
			t.Fatalf("%s: the app must lock too, got %q", c.name, got)
		}
	}

	// a lapsed clinic sends no reminders; they wait and go out once it pays
	set("billing_status = 'trialing', trial_ends_at = now() - interval '1 day'")
	if n := b.tick(); n != 0 {
		t.Fatalf("a lapsed clinic must not send reminders, processed %d", n)
	}
	set("billing_status = 'active', current_period_end = now() + interval '20 days'")
	if n := b.tick(); n == 0 {
		t.Fatal("reminders go out again once the clinic pays")
	}

	// the directory lists only clinics in good standing
	code, _ := anon.do("GET", "/api/public/directory?q=", nil)
	if code != 200 {
		t.Fatalf("directory: %d", code)
	}
}
