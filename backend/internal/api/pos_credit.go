package api

import (
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Sales on account (abonos). A sale with a down payment stays "open" with a balance; every abono goes
// into the register that is open when it arrives and into sale_payments, and the sale turns "paid"
// when the balance reaches zero. Open sales cannot be invoiced.

// addSalePayments records abonos on an open sale.
func (s *Server) addSalePayments(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Venta no encontrada.")
		return
	}
	var req struct {
		Payments []salePaymentIn `json:"payments"`
	}
	if !decode(w, r, &req) {
		return
	}
	if len(req.Payments) == 0 || len(req.Payments) > 6 {
		writeError(w, http.StatusBadRequest, "Indica cómo se pagó el abono.")
		return
	}
	p := principalFrom(r.Context())
	var out sale
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		cfg, err := loadPosSettings(r.Context(), tx, p.ClinicID)
		if err != nil {
			return err
		}
		var status string
		var folio, balance int
		err = tx.QueryRow(r.Context(), `SELECT status, folio, balance_cents FROM sales WHERE clinic_id=$1 AND id=$2 FOR UPDATE`, p.ClinicID, id).Scan(&status, &folio, &balance)
		if errors.Is(err, pgx.ErrNoRows) {
			return fail(http.StatusNotFound, "Venta no encontrada.")
		}
		if err != nil {
			return err
		}
		switch status {
		case "void":
			return fail(http.StatusConflict, "La venta está cancelada.")
		case "paid":
			return fail(http.StatusConflict, "La venta ya está pagada.")
		}
		var sessionID *string
		var sid string
		err = tx.QueryRow(r.Context(), `SELECT id FROM cash_sessions WHERE clinic_id = $1 AND closed_at IS NULL`, p.ClinicID).Scan(&sid)
		switch {
		case err == nil:
			sessionID = &sid
		case errors.Is(err, pgx.ErrNoRows):
			if cfg.RequireOpenCash {
				return &httpError{Status: http.StatusConflict, Code: "CASH_CLOSED", Msg: "Abre la caja antes de cobrar."}
			}
		default:
			return err
		}
		pp, err := parsePayments(r.Context(), tx, p.ClinicID, cfg, req.Payments)
		if err != nil {
			return err
		}
		if pp.paid > balance {
			return fail(http.StatusBadRequest, "El abono ($"+cents(pp.paid)+") supera el saldo ($"+cents(balance)+").")
		}
		for _, sp := range pp.list {
			if _, err := tx.Exec(r.Context(), `
				INSERT INTO sale_payments (sale_id, method, amount_cents, received_cents, change_cents, reference, session_id) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
				id, sp.Method, sp.AmountCents, sp.ReceivedCents, sp.ChangeCents, sp.Reference, sessionID); err != nil {
				return err
			}
		}
		for _, cid := range pp.intents {
			if _, err := tx.Exec(r.Context(), `UPDATE mp_charges SET sale_id = $2 WHERE id = $1`, cid, id); err != nil {
				return err
			}
		}
		left := balance - pp.paid
		newStatus := "open"
		if left == 0 {
			newStatus = "paid"
		}
		if _, err := tx.Exec(r.Context(), `UPDATE sales SET balance_cents = $3, status = $4 WHERE clinic_id = $1 AND id = $2`, p.ClinicID, id, left, newStatus); err != nil {
			return err
		}
		msg := "Registró un abono de $" + cents(pp.paid) + " a la venta #" + itoa(folio)
		if left == 0 {
			msg += " (saldada)"
		}
		audit(r.Context(), tx, p.ClinicID, p, "sale_payment", msg, map[string]any{"folio": folio, "amount_cents": pp.paid, "balance_cents": left})
		out, err = loadSale(r.Context(), tx, p.ClinicID, id)
		return err
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"sale": out})
}

type receivableSale struct {
	ID           string    `json:"id"`
	Folio        int       `json:"folio"`
	CreatedAt    time.Time `json:"created_at"`
	TotalCents   int       `json:"total_cents"`
	BalanceCents int       `json:"balance_cents"`
	AgeDays      int       `json:"age_days"`
}

type receivableCustomer struct {
	Key          string           `json:"key"`
	PatientID    *string          `json:"patient_id"`
	Name         string           `json:"name"`
	BalanceCents int              `json:"balance_cents"`
	OldestDays   int              `json:"oldest_days"`
	Sales        []receivableSale `json:"sales"`
}

// listReceivables groups the open sales by patient (or by customer name) with their age.
func (s *Server) listReceivables(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(), `
		SELECT id, folio, created_at, total_cents, balance_cents, patient_id::text, customer_name
		FROM sales WHERE clinic_id = $1 AND status = 'open' AND balance_cents > 0 ORDER BY created_at`, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	now := time.Now()
	customers := []*receivableCustomer{}
	byKey := map[string]*receivableCustomer{}
	buckets := map[string]int{"0_30": 0, "31_60": 0, "61_90": 0, "over_90": 0}
	total := 0
	for rows.Next() {
		var rs receivableSale
		var patientID *string
		var name string
		if err := rows.Scan(&rs.ID, &rs.Folio, &rs.CreatedAt, &rs.TotalCents, &rs.BalanceCents, &patientID, &name); err != nil {
			serverError(w, r, err)
			return
		}
		rs.AgeDays = int(now.Sub(rs.CreatedAt).Hours() / 24)
		key := "name:" + strings.ToLower(strings.TrimSpace(name))
		if patientID != nil {
			key = "patient:" + *patientID
		}
		c, ok := byKey[key]
		if !ok {
			c = &receivableCustomer{Key: key, PatientID: patientID, Name: name}
			if c.Name == "" {
				c.Name = "Sin nombre"
			}
			byKey[key] = c
			customers = append(customers, c)
		}
		c.Sales = append(c.Sales, rs)
		c.BalanceCents += rs.BalanceCents
		if rs.AgeDays > c.OldestDays {
			c.OldestDays = rs.AgeDays
		}
		total += rs.BalanceCents
		switch {
		case rs.AgeDays <= 30:
			buckets["0_30"] += rs.BalanceCents
		case rs.AgeDays <= 60:
			buckets["31_60"] += rs.BalanceCents
		case rs.AgeDays <= 90:
			buckets["61_90"] += rs.BalanceCents
		default:
			buckets["over_90"] += rs.BalanceCents
		}
	}
	if rows.Err() != nil {
		serverError(w, r, rows.Err())
		return
	}
	sort.SliceStable(customers, func(i, j int) bool { return customers[i].BalanceCents > customers[j].BalanceCents })
	writeJSON(w, http.StatusOK, map[string]any{"customers": customers, "total_cents": total, "aging": buckets})
}
