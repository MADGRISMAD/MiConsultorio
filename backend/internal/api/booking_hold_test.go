package api_test

import (
	"sync"
	"testing"
)

func depositEnv(t *testing.T) (*bookingEnv, *fakeMP, string, func(start, email, phone string) (int, map[string]any)) {
	t.Helper()
	b := newBookingEnv(t, false)
	fake, srv := newFakeMP(t)
	connectMP(t, b, srv.URL)
	var svc string
	if err := b.pool.QueryRow(t.Context(), `SELECT id::text FROM catalog_items WHERE clinic_id = $1 AND name = 'Consulta general'`, b.clinicA).Scan(&svc); err != nil {
		t.Fatal(err)
	}
	b.login("admin_a").expect(200, "PUT", "/api/agenda/services/"+svc+"/booking", map[string]any{"modality": "presencial", "deposit_percent": 30})
	anon := b.anon()
	book := func(start, email, phone string) (int, map[string]any) {
		return anon.do("POST", "/api/public/booking/"+b.slugA+"/appointments", map[string]any{
			"professional_id": b.pro, "service_id": svc, "date": b.date, "start": start, "names": "Ana", "last_names": "López",
			"email": email, "phone": phone, "reason": "Dolor", "accept_privacy": true, "accept_reminders": true,
		})
	}
	return b, fake, svc, book
}

// A service with a deposit holds the time for 15 minutes; without the payment the time is free again, even before the sweep runs.
func TestDepositHoldIsFifteenMinutes(t *testing.T) {
	b, _, _, book := depositEnv(t)
	code, out := book("10:00", "b@x.mx", "5522222222")
	if code != 201 {
		t.Fatalf("book: %d %v", code, out)
	}
	var minutes float64
	if err := b.pool.QueryRow(t.Context(), `SELECT extract(epoch FROM (deposit_expires_at - now())) / 60 FROM appointments WHERE confirm_token = $1`, sub(out, "appointment")["token"]).Scan(&minutes); err != nil {
		t.Fatal(err)
	}
	if minutes < 14 || minutes > 15.1 {
		t.Fatalf("the hold lasts 15 minutes, got %.1f", minutes)
	}
	if code, _ := book("10:00", "c@x.mx", "5533333333"); code != 409 {
		t.Fatalf("held time is taken, got %d", code)
	}
	// the 15 minutes pass without a payment: the time is offered again at once (the sweep has not run)
	b.exec(`UPDATE appointments SET deposit_expires_at = now() - interval '1 second' WHERE confirm_token = $1`, sub(out, "appointment")["token"])
	if code, out := book("10:00", "c@x.mx", "5533333333"); code != 201 {
		t.Fatalf("an unpaid hold that ran out must free the time: %d %v", code, out)
	}
}

// Cancelling an unpaid booking while its payment link is still open: a payment that lands afterwards is returned, not kept.
func TestPaymentAfterCancellationIsRefunded(t *testing.T) {
	b, fake, _, book := depositEnv(t)
	code, out := book("11:00", "d@x.mx", "5544444444")
	if code != 201 {
		t.Fatalf("book: %d %v", code, out)
	}
	token := sub(out, "appointment")["token"].(string)
	var id string
	_ = b.pool.QueryRow(t.Context(), `SELECT id::text FROM appointments WHERE confirm_token = $1`, token).Scan(&id)
	b.anon().expect(200, "POST", "/api/public/appointments/"+token+"/cancel", map[string]any{})
	var state string
	_ = b.pool.QueryRow(t.Context(), `SELECT deposit_state FROM appointments WHERE id = $1`, id).Scan(&state)
	if state != "pending" {
		t.Fatalf("while the link is open the mark stays pending, got %q", state)
	}
	fake.addPayment("9100", "caresia-dep:"+id, 150.00, "approved")
	b.anon().expect(200, "POST", "/api/webhooks/mercadopago?type=payment&dep="+id, map[string]any{"type": "payment", "data": map[string]any{"id": "9100"}})
	if len(fake.refunded) != 1 || fake.refunded[0] != "payment:9100" {
		t.Fatalf("a payment after the cancellation must be refunded: %v", fake.refunded)
	}
	_ = b.pool.QueryRow(t.Context(), `SELECT deposit_state FROM appointments WHERE id = $1`, id).Scan(&state)
	if state != "refunded" {
		t.Fatalf("state: %s", state)
	}
}

// Staff and online booking race for the same time of the same professional: exactly one of them gets it.
func TestStaffAndOnlineBookingDoNotDoubleBook(t *testing.T) {
	b := newBookingEnv(t, false)
	var svc string
	if err := b.pool.QueryRow(t.Context(), `SELECT id::text FROM catalog_items WHERE clinic_id = $1 AND name = 'Consulta general'`, b.clinicA).Scan(&svc); err != nil {
		t.Fatal(err)
	}
	recep, anon := b.login("recep_a"), b.anon()
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var code int
			if i%2 == 0 {
				code, _ = recep.do("POST", "/api/appointments", apptBody(map[string]any{"professional_id": b.pro, "date": b.date, "startHour": "16:00", "endHour": "16:30", "names": "Staff", "last_names": string(rune('A' + i))}))
			} else {
				code, _ = anon.do("POST", "/api/public/booking/"+b.slugA+"/appointments", map[string]any{
					"professional_id": b.pro, "service_id": svc, "date": b.date, "start": "16:00", "names": "Web", "last_names": string(rune('A' + i)),
					"email": string(rune('a'+i)) + "@x.mx", "phone": "55000000" + string(rune('0'+i)) + "0", "reason": "Dolor", "accept_privacy": true, "accept_reminders": true,
				})
			}
			if code == 201 {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	var n int
	_ = b.pool.QueryRow(t.Context(), `SELECT count(*) FROM appointments WHERE clinic_id = $1 AND professional_id = $2 AND date = $3::date AND start_hour = '16:00' AND status NOT IN ('cancelled','no_show')`, b.clinicA, b.pro, b.date).Scan(&n)
	if n != 1 || ok != 1 {
		t.Fatalf("one winner expected: stored=%d created=%d", n, ok)
	}
}
