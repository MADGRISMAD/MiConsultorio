package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

// What an appointment's own time allows. A visit cannot be received, worked or closed before it is its time,
// and it cannot be marked as missed before it has started.

var monthsEs = []string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}

func humanWhen(t time.Time) string {
	return fmt.Sprintf("las %s del %d de %s", t.Format("15:04"), t.Day(), monthsEs[t.Month()-1])
}

// apptTimeGate checks, for the status the appointment is moving to, that its date and time allow it now.
// date and startHour are "2006-01-02" and "15:04" in the clinic's zone.
func apptTimeGate(loc *time.Location, date, startHour, to string, now time.Time) *httpError {
	start, err := time.ParseInLocation("2006-01-02 15:04", date+" "+startHour, loc)
	if err != nil {
		return nil
	}
	tooEarly := func(from time.Time, what string) *httpError {
		return &httpError{Status: http.StatusConflict, Code: "TOO_EARLY",
			Msg: "Todavía no es hora de esta cita: " + what + " desde " + humanWhen(from.In(loc)) + "."}
	}
	switch to {
	case "arrived", "in_progress":
		if now.Before(start.Add(-apptEarlyWindow)) {
			return tooEarly(start.Add(-apptEarlyWindow), "se puede marcar como llegada o iniciar")
		}
	case "completed", "no_show":
		if now.Before(start) {
			if to == "no_show" {
				return tooEarly(start, "se puede marcar como no asistió")
			}
			return tooEarly(start, "se puede terminar")
		}
	}
	return nil
}

// autoChargeAppointment sends the finished consultation to the register as a pre-account: the appointment's own
// service, or the clinic's default "Consulta". It does nothing when the clinic has no payments plan, the visit
// has no registered patient, it was already charged or queued, or the service has no price. Returns the pre-account id.
func (s *Server) autoChargeAppointment(ctx context.Context, tx pgx.Tx, p *Principal, apptID string) (string, int, error) {
	if !cbHasCobros(p) {
		return "", 0, nil
	}
	var patientID, professionalID, serviceID, saleID *string
	var date string
	err := tx.QueryRow(ctx, `
		SELECT patient_id::text, professional_id::text, service_id::text, sale_id::text, to_char(date, 'YYYY-MM-DD')
		FROM appointments WHERE clinic_id = $1 AND id = $2`, p.ClinicID, apptID).Scan(&patientID, &professionalID, &serviceID, &saleID, &date)
	if err != nil {
		return "", 0, err
	}
	if patientID == nil || saleID != nil {
		return "", 0, nil
	}
	var existing string
	err = tx.QueryRow(ctx, `SELECT id::text FROM encounter_charges WHERE clinic_id = $1 AND appointment_id = $2 AND status IN ('draft', 'sent', 'charged') ORDER BY created_at LIMIT 1`, p.ClinicID, apptID).Scan(&existing)
	if err == nil {
		// A pre-account the professional left as a draft goes to the register now; one already sent or charged stays.
		var status string
		if err := tx.QueryRow(ctx, `SELECT status FROM encounter_charges WHERE id = $1`, existing).Scan(&status); err != nil {
			return "", 0, err
		}
		if status != "draft" {
			return "", 0, nil
		}
		if err := cbMarkSent(ctx, tx, p, existing); err != nil {
			var he *httpError
			if errors.As(err, &he) && he.Status == http.StatusBadRequest {
				return "", 0, nil // nothing with a price in it: leave the draft as it is
			}
			return "", 0, err
		}
		var total int
		_ = tx.QueryRow(ctx, `SELECT coalesce(sum(round(qty * unit_price_cents)::int - discount_cents), 0)::int FROM encounter_charge_items WHERE charge_id = $1`, existing).Scan(&total)
		return existing, total, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", 0, err
	}

	var item struct {
		ID, Name string
		Price    int
		Tax      float64
	}
	pick := `SELECT id::text, name, price_cents, tax_rate::float8 FROM catalog_items WHERE clinic_id = $1 AND kind = 'service' AND active AND `
	if serviceID != nil {
		err = tx.QueryRow(ctx, pick+`id = $2`, p.ClinicID, *serviceID).Scan(&item.ID, &item.Name, &item.Price, &item.Tax)
	} else {
		err = tx.QueryRow(ctx, pick+`system_key = 'consulta'`, p.ClinicID).Scan(&item.ID, &item.Name, &item.Price, &item.Tax)
	}
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && item.Price <= 0) {
		return "", 0, nil
	}
	if err != nil {
		return "", 0, err
	}
	pro := professionalID
	if pro == nil && p.Role == RoleDoctor {
		pro = &p.UserID
	}
	var chargeID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO encounter_charges (clinic_id, patient_id, appointment_id, professional_id, note, created_by, created_by_name)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id::text`,
		p.ClinicID, *patientID, apptID, pro, "Consulta de la cita del "+date, p.UserID, p.actorName()).Scan(&chargeID); err != nil {
		return "", 0, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO encounter_charge_items (charge_id, clinic_id, catalog_item_id, kind, name, qty, unit_price_cents, tax_rate, discount_cents, note, position)
		VALUES ($1, $2, $3, 'service', $4, 1, $5, $6, 0, '', 0)`, chargeID, p.ClinicID, item.ID, item.Name, item.Price, item.Tax); err != nil {
		return "", 0, err
	}
	if err := cbMarkSent(ctx, tx, p, chargeID); err != nil {
		return "", 0, err
	}
	return chargeID, item.Price, nil
}

// tryAutoCharge runs autoChargeAppointment in a savepoint: closing the appointment never fails because of billing.
func (s *Server) tryAutoCharge(ctx context.Context, tx pgx.Tx, p *Principal, apptID string) (chargeID string, total int) {
	sp, err := tx.Begin(ctx)
	if err != nil {
		return "", 0
	}
	id, t, err := s.autoChargeAppointment(ctx, sp, p, apptID)
	if err != nil {
		_ = sp.Rollback(ctx)
		log.Printf("auto charge of appointment %s skipped: %v", apptID, err)
		return "", 0
	}
	if err := sp.Commit(ctx); err != nil {
		return "", 0
	}
	return id, t
}
