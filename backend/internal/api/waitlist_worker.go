package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/madgrismad/miconsultorio/backend/internal/config"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

const (
	// A scan hands out at most this many offers per clinic, so one pass stays short.
	waitlistMaxOffersPerScan = 50
	// Offers are searched at most this many days ahead, whatever the clinic's booking horizon.
	waitlistMaxHorizonDays = 60
	// A slot that starts sooner than the offer's life plus this margin is not worth offering.
	waitlistStartMargin = 5 * time.Minute
)

// waitlistKick wakes the worker when a slot may have been freed (a cancellation, a move, a block removed).
var waitlistKick = make(chan struct{}, 1)

// waitlistWake asks the worker to look for offers soon. It never blocks.
func waitlistWake() {
	select {
	case waitlistKick <- struct{}{}:
	default:
	}
}

// runWaitlist hands out freed slots and expires unanswered offers until ctx ends. It also runs once a
// minute, which covers every way a slot can free up (including ones that do not call waitlistWake).
func (s *Server) waitlistPass(ctx context.Context) {
	if _, err := s.waitlistRun(ctx); err != nil && ctx.Err() == nil {
		log.Printf("waitlist: %v", err)
	}
}

// RunWaitlistOnce expires old offers and hands out the freed slots right now (used by tests and tooling).
// It returns how many offers were made.
func RunWaitlistOnce(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, mailer mail.Sender) (int, error) {
	return newServer(pool, cfg, mailer).waitlistRun(ctx)
}

func (s *Server) waitlistRun(ctx context.Context) (int, error) {
	if err := s.wlExpire(ctx); err != nil {
		return 0, err
	}
	rows, err := s.db.Query(ctx, `SELECT DISTINCT clinic_id::text FROM waitlist_entries WHERE status = 'waiting'`)
	if err != nil {
		return 0, err
	}
	clinics, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return 0, err
	}
	total := 0
	for _, id := range clinics {
		n, err := s.wlScanClinic(ctx, id)
		if err != nil {
			log.Printf("waitlist: clinic %s: %v", id, err)
			continue
		}
		total += n
	}
	return total, nil
}

// wlExpire closes the offers nobody answered in time (their people go back to waiting) and lets old
// entries lapse. SKIP LOCKED keeps several API instances from fighting over the same rows.
func (s *Server) wlExpire(ctx context.Context) error {
	err := inTx(ctx, s.db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			WITH due AS (
				SELECT id FROM waitlist_offers WHERE status = 'offered' AND expires_at <= now()
				ORDER BY expires_at LIMIT 200 FOR UPDATE SKIP LOCKED)
			UPDATE waitlist_offers o SET status = 'expired', responded_at = now() FROM due WHERE o.id = due.id
			RETURNING o.entry_id::text`)
		if err != nil {
			return err
		}
		entries, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			return nil
		}
		_, err = tx.Exec(ctx, `UPDATE waitlist_entries SET status = 'waiting', updated_at = now()
			WHERE id = ANY($1::uuid[]) AND status = 'offered'
			  AND NOT EXISTS (SELECT 1 FROM waitlist_offers o WHERE o.entry_id = waitlist_entries.id AND o.status = 'offered')`, entries)
		return err
	})
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `UPDATE waitlist_entries SET status = 'expired', updated_at = now()
		WHERE status = 'waiting' AND created_at < now() - $1::float8 * interval '1 second'`, waitlistEntryTTL.Seconds())
	return err
}

// wlClinic loads the scheduling data of a clinic whose subscription is alive, whether or not it takes
// online bookings.
func (s *Server) wlClinic(ctx context.Context, clinicID string) (*bookingClinic, error) {
	var c bookingClinic
	var settings []byte
	var tz string
	err := s.db.QueryRow(ctx, `
		SELECT c.id::text, c.name, c.kind, c.address, c.phone_number, c.settings, coalesce(c.settings->>'timezone', ''),
		       coalesce(a.booking_horizon_days, 30), coalesce(a.slot_minutes, 30)
		FROM clinics c LEFT JOIN agenda_settings a ON a.clinic_id = c.id
		WHERE c.id = $1 AND c.billing_status IN ('active', 'trialing', 'past_due') AND c.branch_suspended_at IS NULL`, clinicID).
		Scan(&c.ID, &c.Name, &c.Kind, &c.Address, &c.Phone, &settings, &tz, &c.HorizonDays, &c.SlotMinutes)
	if err != nil {
		return nil, err
	}
	c.Loc = locationOrDefault(tz)
	_ = json.Unmarshal(settings, &c.Hours)
	c.Hours = c.Hours.normalized()
	if c.HorizonDays > waitlistMaxHorizonDays {
		c.HorizonDays = waitlistMaxHorizonDays
	}
	return &c, nil
}

// wlProfessionals are the active professionals of a clinic with their agenda settings, in a stable
// order. bookable marks the ones offered on the public page.
func (s *Server) wlProfessionals(ctx context.Context, clinicID string) (all []bookable, isBookable map[string]bool, err error) {
	rows, err := s.db.Query(ctx, `
		SELECT u.id::text, u.name, coalesce(ps.slot_minutes, 0), coalesce(ps.hours, '{}'::jsonb), coalesce(ps.bookable, false)
		FROM users u LEFT JOIN professional_settings ps ON ps.user_id = u.id
		WHERE u.clinic_id = $1 AND NOT u.disabled AND u.role IN ('admin', 'doctor') AND coalesce(ps.consults, true) ORDER BY u.name, u.id`, clinicID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	isBookable = map[string]bool{}
	for rows.Next() {
		var p bookable
		var raw []byte
		var b bool
		if err := rows.Scan(&p.ID, &p.Name, &p.Slot, &raw, &b); err != nil {
			return nil, nil, err
		}
		_ = json.Unmarshal(raw, &p.Hours)
		all = append(all, p)
		isBookable[p.ID] = b
	}
	return all, isBookable, rows.Err()
}

type wlEntry struct {
	ID, Name, Phone, Email, ProfessionalID, ServiceID, Token string
	Days                                                     []int16
	From, To                                                 *string
	Consent                                                  bool
}

// contactable says the person can be offered a slot by e-mail: they agreed to it and left an address.
func (e wlEntry) contactable() bool { return e.Consent && e.Email != "" }

func (e wlEntry) wantsDay(w time.Weekday) bool {
	if len(e.Days) == 0 {
		return true
	}
	for _, d := range e.Days {
		if int(d) == int(w) {
			return true
		}
	}
	return false
}

func (e wlEntry) fits(start, end string) bool {
	return e.From == nil || (start >= *e.From && end <= *e.To)
}

// wlScanClinic looks for free slots for the people waiting in a clinic, first come first served, and
// offers each one the earliest slot that suits them. An advisory lock keeps two scans of the same
// clinic (two API instances) from running at once. It returns how many offers were made.
func (s *Server) wlScanClinic(ctx context.Context, clinicID string) (int, error) {
	conn, err := s.db.Acquire(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Release()
	lockKey := "waitlist:" + clinicID
	var got bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1, 0))`, lockKey).Scan(&got); err != nil || !got {
		return 0, err
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1, 0))`, lockKey)
	}()

	c, err := s.wlClinic(ctx, clinicID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT id::text, name, phone, email, coalesce(professional_id::text, ''), coalesce(service_id::text, ''), days,
		       to_char(from_time, 'HH24:MI'), to_char(to_time, 'HH24:MI'), consent, token
		FROM waitlist_entries WHERE clinic_id = $1 AND status = 'waiting' ORDER BY created_at, id LIMIT 200`, clinicID)
	if err != nil {
		return 0, err
	}
	entries, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (wlEntry, error) {
		var e wlEntry
		err := r.Scan(&e.ID, &e.Name, &e.Phone, &e.Email, &e.ProfessionalID, &e.ServiceID, &e.Days, &e.From, &e.To, &e.Consent, &e.Token)
		return e, err
	})
	if err != nil || len(entries) == 0 {
		return 0, err
	}
	ids := make([]string, len(entries))
	for i, e := range entries {
		ids[i] = e.ID
	}
	orows, err := s.db.Query(ctx, `
		SELECT entry_id::text || '|' || professional_id::text || '|' || to_char(date, 'YYYY-MM-DD') || '|' || to_char(start_hour, 'HH24:MI')
		FROM waitlist_offers WHERE clinic_id = $1 AND entry_id = ANY($2::uuid[])`, clinicID, ids)
	if err != nil {
		return 0, err
	}
	prior, err := pgx.CollectRows(orows, pgx.RowTo[string])
	if err != nil {
		return 0, err
	}
	tried := map[string]bool{}
	for _, k := range prior {
		tried[k] = true
	}
	pros, isBookable, err := s.wlProfessionals(ctx, clinicID)
	if err != nil || len(pros) == 0 {
		return 0, err
	}
	var anyPool []bookable // professionals "any" refers to: the online ones, or all when none is
	for _, p := range pros {
		if isBookable[p.ID] {
			anyPool = append(anyPool, p)
		}
	}
	if len(anyPool) == 0 {
		anyPool = pros
	}
	proIDs := make([]string, len(pros))
	for i, p := range pros {
		proIDs[i] = p.ID
	}
	now := time.Now()
	nowLocal := now.In(c.Loc)
	firstDay := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, c.Loc)
	lastDay := firstDay.AddDate(0, 0, c.HorizonDays)
	busy, err := s.loadSlotBusy(ctx, clinicID, proIDs, firstDay.Format("2006-01-02"), lastDay.Format("2006-01-02"))
	if err != nil {
		return 0, err
	}
	scan := *c
	scan.LeadHours = 0
	minStart := now.Add(waitlistOfferTTL + waitlistStartMargin)
	durCache := map[string]int{}
	made := 0
next:
	for _, e := range entries {
		if made >= waitlistMaxOffersPerScan {
			break
		}
		dur, seen := durCache[e.ServiceID]
		if !seen {
			dur = s.slotServiceMinutes(ctx, s.db, clinicID, e.ServiceID)
			durCache[e.ServiceID] = dur
		}
		cand := anyPool
		if e.ProfessionalID != "" {
			cand = nil
			for _, p := range pros {
				if p.ID == e.ProfessionalID {
					cand = []bookable{p}
				}
			}
		}
		type option struct {
			pro         bookable
			date        string
			start, end  string
			startsAt    time.Time
			proPosition int
		}
	days:
		for d := firstDay; !d.After(lastDay); d = d.AddDate(0, 0, 1) {
			if !e.wantsDay(d.Weekday()) {
				continue
			}
			date := d.Format("2006-01-02")
			var opts []option
			for i, p := range cand {
				for _, st := range scan.candidateSlots(p, date, now, dur) {
					end := p.end(&scan, st, dur)
					at, err := localTime(c.Loc, date, st)
					if err != nil || at.Before(minStart) || !e.fits(st, end) || tried[e.ID+"|"+p.ID+"|"+date+"|"+st] {
						continue
					}
					if busy.free(p.ID, date, slotClock(st), slotClock(end)) {
						opts = append(opts, option{p, date, st, end, at, i})
					}
				}
			}
			sort.Slice(opts, func(i, j int) bool {
				if opts[i].start != opts[j].start {
					return opts[i].start < opts[j].start
				}
				return opts[i].proPosition < opts[j].proPosition
			})
			for _, o := range opts {
				if !e.contactable() {
					// Nobody can be sent the link: tell the team so they can call, once per person and slot.
					s.notify(ctx, s.db, clinicID, ntfNotice{
						Perm: PermAdminAppointments, Kind: "waitlist_match", Title: "Hay un lugar para alguien en la lista de espera",
						Body:   fmt.Sprintf("%s: %s a las %s con %s. Llámale para ofrecérselo.", e.Name, o.date, o.start, o.pro.Name),
						Link:   "/agenda/espera",
						Dedupe: "wl-match:" + e.ID + ":" + o.pro.ID + ":" + o.date + ":" + o.start,
					})
					continue next
				}
				ok, err := s.wlOffer(ctx, &scan, e, o.pro, o.date, o.start, o.end, dur)
				if err != nil {
					return made, err
				}
				if !ok {
					continue
				}
				made++
				k := slotKey(o.pro.ID, o.date)
				busy.holds[k] = append(busy.holds[k], slotSpan{slotClock(o.start), slotClock(o.end)})
				break days
			}
		}
	}
	return made, nil
}

// wlOffer holds a free slot for one person and tells them. ok is false when the slot was taken (or the
// person was served) between the scan and now.
func (s *Server) wlOffer(ctx context.Context, c *bookingClinic, e wlEntry, pro bookable, date, start, end string, dur int) (bool, error) {
	var expires time.Time
	ok := false
	err := inTx(ctx, s.db, func(tx pgx.Tx) error {
		// Same locks, same order, as an online booking and as accepting an offer.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "booking|"+c.ID+"|"+pro.ID+"|"+date); err != nil {
			return err
		}
		if err := lockAgendaDay(ctx, tx, c.ID, date); err != nil {
			return err
		}
		var status string
		if err := tx.QueryRow(ctx, `SELECT status FROM waitlist_entries WHERE clinic_id = $1 AND id = $2 FOR UPDATE`, c.ID, e.ID).Scan(&status); err != nil {
			return err
		}
		if status != "waiting" {
			return nil
		}
		pid := pro.ID
		code, err := s.slotConflict(ctx, tx, c.ID, &pid, "", date, start, end, "")
		if err != nil || code != SlotFree {
			return err
		}
		held, err := s.slotHeldSpans(ctx, tx, c.ID, pro.ID, date, "")
		if err != nil || held.overlaps(start, end) {
			return err
		}
		var svc any
		if e.ServiceID != "" {
			svc = e.ServiceID
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO waitlist_offers (clinic_id, entry_id, professional_id, service_id, date, start_hour, end_hour, expires_at)
			VALUES ($1, $2, $3, $4::uuid, $5::date, $6::time, $7::time, now() + $8::float8 * interval '1 second') RETURNING expires_at`,
			c.ID, e.ID, pro.ID, svc, date, start, end, waitlistOfferTTL.Seconds()).Scan(&expires); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE waitlist_entries SET status = 'offered', updated_at = now() WHERE id = $1`, e.ID); err != nil {
			return err
		}
		audit(ctx, tx, c.ID, nil, "waitlist_offer", "Se ofreció un lugar liberado a alguien de la lista de espera", map[string]any{"entry": e.ID, "date": date, "start": start})
		s.ntfAppointment(ctx, tx, c.ID, pro.ID, "waitlist_offered", "Lugar ofrecido desde la lista de espera",
			fmt.Sprintf("%s: %s a las %s con %s (vence %s).", e.Name, date, start, pro.Name, expires.In(c.Loc).Format("15:04")), "/agenda/espera")
		ok = true
		return nil
	})
	if err != nil {
		if isUniqueViolation(err) { // another instance took the same slot or person first
			return false, nil
		}
		return false, err
	}
	if ok {
		s.wlOfferMail(c, e, pro, date, start, expires)
	}
	return ok, nil
}

func (s *Server) wlOfferMail(c *bookingClinic, e wlEntry, pro bookable, date, start string, expires time.Time) {
	at, err := localTime(c.Loc, date, start)
	if err != nil || !s.mailEnabled() || e.Email == "" {
		return
	}
	link := s.appLink("/espera/" + e.Token)
	hello := "Hola " + firstName(e.Name)
	until := clockES(expires.In(c.Loc))
	info := apptInfo{ClinicName: c.Name, ClinicAddress: c.Address, ClinicPhone: c.Phone, Professional: pro.Name, Start: at}
	subject := "Se liberó un lugar en " + c.Name
	text := hello + ", se liberó un lugar para ti.\n\n" + info.detailsText() + "\nLo guardamos para ti hasta las " + until +
		". Acéptalo o recházalo aquí:\n" + link + "\n\nRecibes este aviso porque pediste que te avisáramos si se liberaba un lugar.\n"
	body := `<p style="margin:0 0 8px">` + esc(hello) + `, se liberó un lugar para ti en <strong>` + esc(c.Name) + `</strong>.</p>` + info.details() +
		`<p style="margin:12px 0">Lo guardamos para ti hasta las <strong>` + esc(until) + `</strong>.</p>` + button(link, "Ver y aceptar el lugar") +
		`<p style="font-size:13px;color:#7a8b9b;margin:14px 0 0">Recibes este aviso porque pediste que te avisáramos si se liberaba un lugar.</p>`
	s.sendMail(mail.Message{To: []string{e.Email}, Subject: subject, Text: text, HTML: layout("Se liberó un lugar", body)})
}
