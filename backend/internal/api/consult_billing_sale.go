package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
)

// How a pre-account meets the sale that charges it (createSale with consult_charge_id).
//
// Stock rule, so a unit never leaves the inventory twice:
//   - A pre-account line marked consumed took its units out of stock when it was saved.
//   - Units recorded for the same consultation through POST /encounters/{id}/consumables (not tied to a
//     pre-account line) and not yet billed count as consumed too.
//
// While the sale prices its lines, each tracked product line skips, up to its quantity, whatever is left in
// that pool for its catalog item; only the rest is taken from stock. Supplies used but not billed
// (no_charge, price 0) never enter the pool: they are not on the ticket. When the sale is written, the
// consumptions it billed are stamped with billed_sale_id and the pre-account becomes 'charged' with
// sale_id, in the same transaction. Voiding the sale puts back only what the sale itself took
// (restockSale looks at sale_id), never the supplies that were physically spent in the consultation.
// A legacy consumption is claimed whole by the first sale that bills its item; any surplus is not carried over.

type cbLink struct {
	ID             string
	PatientID      string
	AppointmentID  *string
	ProfessionalID *string
	EncounterID    *string
	// pool is what is already out of stock and not yet billed, per catalog item; used is what sale lines took from it.
	pool, used map[string]float64
}

// cbLoadForSale locks an open pre-account (draft or sent) of the clinic and builds its stock pool.
func cbLoadForSale(ctx context.Context, tx pgx.Tx, clinicID, id string) (*cbLink, error) {
	l := &cbLink{ID: id, pool: map[string]float64{}, used: map[string]float64{}}
	var status string
	err := tx.QueryRow(ctx, `SELECT status, patient_id::text, appointment_id::text, professional_id::text, encounter_id::text
		FROM encounter_charges WHERE clinic_id = $1 AND id = $2 FOR UPDATE`, clinicID, id).
		Scan(&status, &l.PatientID, &l.AppointmentID, &l.ProfessionalID, &l.EncounterID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fail(http.StatusNotFound, "La pre-cuenta no existe en este consultorio.")
	}
	if err != nil {
		return nil, err
	}
	if status != "draft" && status != "sent" {
		return nil, &httpError{Status: http.StatusConflict, Code: "CHARGE_CLOSED", Msg: "La pre-cuenta ya fue cobrada o cancelada."}
	}
	fill := func(sql string, args ...any) error {
		rows, err := tx.Query(ctx, sql, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item string
			var qty float64
			if err := rows.Scan(&item, &qty); err != nil {
				return err
			}
			l.pool[item] += qty
		}
		return rows.Err()
	}
	if err := fill(`
		SELECT catalog_item_id::text, sum(qty)::float8 FROM encounter_charge_items
		WHERE charge_id = $1 AND consumed AND catalog_item_id IS NOT NULL AND unit_price_cents > 0 GROUP BY 1`, id); err != nil {
		return nil, err
	}
	if l.EncounterID != nil {
		// two pre-accounts of one consultation must not claim the same consumptions at once
		if _, err := tx.Exec(ctx, `SELECT 1 FROM encounters WHERE id = $1 FOR NO KEY UPDATE`, *l.EncounterID); err != nil {
			return nil, err
		}
		if err := fill(`
			SELECT item_id::text, sum(-delta)::float8 FROM stock_movements
			WHERE clinic_id = $1 AND encounter_id = $2 AND reason = 'consumption' AND delta < 0
			  AND charge_item_id IS NULL AND billed_sale_id IS NULL AND sale_id IS NULL GROUP BY 1`, clinicID, *l.EncounterID); err != nil {
			return nil, err
		}
	}
	return l, nil
}

// take returns how much of qty is already out of stock for the item, and removes it from the pool.
func (l *cbLink) take(itemID string, qty float64) float64 {
	if l == nil {
		return 0
	}
	n := l.pool[itemID]
	if n > qty {
		n = qty
	}
	if n <= qtyEps {
		return 0
	}
	l.pool[itemID] -= n
	l.used[itemID] += n
	return n
}

// finish marks the pre-account charged by the sale and stamps the consumptions the sale billed.
func (l *cbLink) finish(ctx context.Context, tx pgx.Tx, clinicID, saleID string) error {
	if _, err := tx.Exec(ctx, `UPDATE encounter_charges SET status = 'charged', sale_id = $3, charged_at = now(), updated_at = now() WHERE clinic_id = $1 AND id = $2`,
		clinicID, l.ID, saleID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE stock_movements SET billed_sale_id = $2 WHERE clinic_id = $1 AND billed_sale_id IS NULL
		  AND charge_item_id IN (SELECT id FROM encounter_charge_items WHERE charge_id = $3 AND unit_price_cents > 0)`, clinicID, saleID, l.ID); err != nil {
		return err
	}
	if l.EncounterID == nil || len(l.used) == 0 {
		return nil
	}
	items := make([]string, 0, len(l.used))
	for id := range l.used {
		items = append(items, id)
	}
	_, err := tx.Exec(ctx, `
		UPDATE stock_movements SET billed_sale_id = $2 WHERE clinic_id = $1 AND encounter_id = $3 AND reason = 'consumption' AND delta < 0
		  AND charge_item_id IS NULL AND billed_sale_id IS NULL AND sale_id IS NULL AND item_id = ANY($4::uuid[])`, clinicID, saleID, *l.EncounterID, items)
	return err
}
