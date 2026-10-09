package api

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Devoluciones. A customer brings back part of what a sale included. The sale stays "paid"; the return is its
// own record that gives the stock back, refunds the money (cash out of the register, or Mercado Pago for card
// payments) and reverses the commission pro rata. Services are refunded but never restocked.

type retLine struct {
	ID              string
	ItemID          *string
	ProfessionalID  *string
	Kind, Name      string
	Qty             float64
	EffectiveCents  int // what the customer really paid for the line (after the ticket discount)
	CommissionCents int
	CommissionBase  int
	TracksStock     bool
	ReturnedQty     float64
	ReturnedCents   int
	ReturnedComm    int
	ReturnedBase    int
}

func (l retLine) remainingQty() float64 { return math.Max(0, l.Qty-l.ReturnedQty) }
func (l retLine) remainingCents() int   { return max(0, l.EffectiveCents-l.ReturnedCents) }

type retState struct {
	SaleID      string
	Folio       int
	Status      string
	TotalCents  int
	ReturnedSum int
	Invoiced    bool
	Lines       []retLine
	Refundable  map[string]int // method -> cents still refundable
	MethodOrder []string       // most recent payment first
}

// loadReturnState reads everything a return needs; lock=true takes the sale row FOR UPDATE.
func loadReturnState(ctx context.Context, tx pgx.Tx, clinicID, saleID string, lock bool) (*retState, error) {
	st := &retState{SaleID: saleID, Refundable: map[string]int{}}
	q := `SELECT folio, status, total_cents FROM sales WHERE clinic_id=$1 AND id=$2`
	if lock {
		q += ` FOR UPDATE`
	}
	if err := tx.QueryRow(ctx, q, clinicID, saleID).Scan(&st.Folio, &st.Status, &st.TotalCents); err != nil {
		return nil, err
	}
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM invoice_requests WHERE sale_id=$1 AND status='issued')`, saleID).Scan(&st.Invoiced); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `
		SELECT si.id, si.item_id::text, si.professional_id::text, si.kind, si.name, si.qty::float8, si.total_cents, si.commission_cents, si.commission_base_cents,
		       coalesce(ci.track_stock, false),
		       coalesce((SELECT sum(ri.qty)::float8 FROM sale_return_items ri WHERE ri.sale_item_id = si.id), 0),
		       coalesce((SELECT sum(ri.amount_cents)::int FROM sale_return_items ri WHERE ri.sale_item_id = si.id), 0),
		       coalesce((SELECT sum(ri.commission_cents)::int FROM sale_return_items ri WHERE ri.sale_item_id = si.id), 0),
		       coalesce((SELECT sum(ri.commission_base_cents)::int FROM sale_return_items ri WHERE ri.sale_item_id = si.id), 0)
		FROM sale_items si LEFT JOIN catalog_items ci ON ci.id = si.item_id
		WHERE si.sale_id = $1 ORDER BY si.name, si.id`, saleID)
	if err != nil {
		return nil, err
	}
	sumLines := 0
	for rows.Next() {
		var l retLine
		var lineTotal int
		if err := rows.Scan(&l.ID, &l.ItemID, &l.ProfessionalID, &l.Kind, &l.Name, &l.Qty, &lineTotal, &l.CommissionCents, &l.CommissionBase,
			&l.TracksStock, &l.ReturnedQty, &l.ReturnedCents, &l.ReturnedComm, &l.ReturnedBase); err != nil {
			rows.Close()
			return nil, err
		}
		l.EffectiveCents = lineTotal // adjusted below once the sum of the lines is known
		sumLines += lineTotal
		st.Lines = append(st.Lines, l)
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	// The ticket discount is spread over the lines pro rata, as when the sale was made.
	if sumLines > 0 && sumLines != st.TotalCents {
		for i := range st.Lines {
			st.Lines[i].EffectiveCents = int(roundDiv(int64(st.Lines[i].EffectiveCents)*int64(st.TotalCents), int64(sumLines)))
		}
	}
	for _, l := range st.Lines {
		st.ReturnedSum += l.ReturnedCents
	}
	prow, err := tx.Query(ctx, `SELECT method, sum(amount_cents)::int FROM sale_payments WHERE sale_id=$1 GROUP BY method ORDER BY max(created_at) DESC, method`, saleID)
	if err != nil {
		return nil, err
	}
	for prow.Next() {
		var m string
		var c int
		if err := prow.Scan(&m, &c); err != nil {
			prow.Close()
			return nil, err
		}
		st.Refundable[m] = c
		st.MethodOrder = append(st.MethodOrder, m)
	}
	prow.Close()
	if prow.Err() != nil {
		return nil, prow.Err()
	}
	rrows, err := tx.Query(ctx, `SELECT rf.method, sum(rf.amount_cents)::int FROM sale_return_refunds rf JOIN sale_returns r ON r.id = rf.return_id WHERE r.sale_id=$1 GROUP BY rf.method`, saleID)
	if err != nil {
		return nil, err
	}
	for rrows.Next() {
		var m string
		var c int
		if err := rrows.Scan(&m, &c); err != nil {
			rrows.Close()
			return nil, err
		}
		st.Refundable[m] -= c
	}
	rrows.Close()
	return st, rrows.Err()
}

// retAmount is the money (and commission) for returning qty of a line; the last unit takes the rounding remainder.
func retAmount(l retLine, qty float64) (cents, comm, base int) {
	if qty >= l.remainingQty()-qtyEps {
		return l.remainingCents(), max(0, l.CommissionCents-l.ReturnedComm), max(0, l.CommissionBase-l.ReturnedBase)
	}
	f := qty / l.Qty
	return min(l.remainingCents(), int(math.Round(float64(l.EffectiveCents)*f))),
		min(max(0, l.CommissionCents-l.ReturnedComm), int(math.Round(float64(l.CommissionCents)*f))),
		min(max(0, l.CommissionBase-l.ReturnedBase), int(math.Round(float64(l.CommissionBase)*f)))
}

// suggestRefunds spreads total over the payment methods, the most recent payment first.
func suggestRefunds(st *retState, total int) []methodTotal {
	out := []methodTotal{}
	left := total
	for _, m := range st.MethodOrder {
		if left <= 0 {
			break
		}
		take := min(left, max(0, st.Refundable[m]))
		if take > 0 {
			out = append(out, methodTotal{Method: m, AmountCents: take, Count: 1})
			left -= take
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// GET /pos/sales/{id}/returns : what can still be returned, and past returns
// ---------------------------------------------------------------------------

type returnableLine struct {
	SaleItemID     string  `json:"sale_item_id"`
	Name           string  `json:"name"`
	Kind           string  `json:"kind"`
	Qty            float64 `json:"qty"`
	ReturnedQty    float64 `json:"returned_qty"`
	ReturnableQty  float64 `json:"returnable_qty"`
	ReturnableCent int     `json:"returnable_cents"`
	UnitCents      int     `json:"unit_cents"` // what one unit really cost the customer
	TracksStock    bool    `json:"tracks_stock"`
}

type pastReturn struct {
	ID         string    `json:"id"`
	Folio      int       `json:"folio"`
	TotalCents int       `json:"total_cents"`
	Reason     string    `json:"reason"`
	CreditNote bool      `json:"credit_note_pending"`
	CreatedBy  string    `json:"created_by_name"`
	CreatedAt  time.Time `json:"created_at"`
}

func (s *Server) saleReturnInfo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Venta no encontrada.")
		return
	}
	p := principalFrom(r.Context())
	var out map[string]any
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		st, err := loadReturnState(r.Context(), tx, p.ClinicID, id, false)
		if errors.Is(err, pgx.ErrNoRows) {
			return fail(http.StatusNotFound, "Venta no encontrada.")
		}
		if err != nil {
			return err
		}
		lines := []returnableLine{}
		for _, l := range st.Lines {
			unit := 0
			if l.Qty > 0 {
				unit = int(math.Round(float64(l.EffectiveCents) / l.Qty))
			}
			lines = append(lines, returnableLine{SaleItemID: l.ID, Name: l.Name, Kind: l.Kind, Qty: l.Qty, ReturnedQty: l.ReturnedQty,
				ReturnableQty: l.remainingQty(), ReturnableCent: l.remainingCents(), UnitCents: unit, TracksStock: l.TracksStock && l.ItemID != nil})
		}
		refundable := []methodTotal{}
		for _, m := range st.MethodOrder {
			if st.Refundable[m] > 0 {
				refundable = append(refundable, methodTotal{Method: m, AmountCents: st.Refundable[m], Count: 1})
			}
		}
		past := []pastReturn{}
		rows, err := tx.Query(r.Context(), `SELECT id, folio, total_cents, reason, credit_note_pending, created_by_name, created_at FROM sale_returns WHERE sale_id=$1 ORDER BY folio`, id)
		if err != nil {
			return err
		}
		for rows.Next() {
			var x pastReturn
			if err := rows.Scan(&x.ID, &x.Folio, &x.TotalCents, &x.Reason, &x.CreditNote, &x.CreatedBy, &x.CreatedAt); err != nil {
				rows.Close()
				return err
			}
			past = append(past, x)
		}
		rows.Close()
		out = map[string]any{
			"sale":  map[string]any{"id": st.SaleID, "folio": st.Folio, "status": st.Status, "total_cents": st.TotalCents, "invoiced": st.Invoiced},
			"lines": lines, "refundable": refundable, "returns": past,
			"can_return": st.Status == "paid", "returned_cents": st.ReturnedSum,
		}
		return rows.Err()
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------------------------------------------------------------------------
// POST /pos/sales/{id}/returns
// ---------------------------------------------------------------------------

type returnLineIn struct {
	SaleItemID string  `json:"sale_item_id"`
	Qty        float64 `json:"qty"`
	Restock    *bool   `json:"restock"` // give the units back to the inventory (default: yes for stocked products)
}

type returnRefundIn struct {
	Method      string `json:"method"`
	AmountCents int    `json:"amount_cents"`
}

type returnIn struct {
	Reason  string           `json:"reason"`
	Lines   []returnLineIn   `json:"lines"`
	Refunds []returnRefundIn `json:"refunds"` // optional: how the money goes back; default suggestion otherwise
}

func (s *Server) createSaleReturn(w http.ResponseWriter, r *http.Request) {
	saleID := chi.URLParam(r, "id")
	if !validUUID(saleID) {
		writeError(w, http.StatusNotFound, "Venta no encontrada.")
		return
	}
	var in returnIn
	if !decode(w, r, &in) {
		return
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if in.Reason == "" || utf8.RuneCountInString(in.Reason) > 200 {
		writeError(w, http.StatusBadRequest, "Escribe el motivo de la devolución (máximo 200 caracteres).")
		return
	}
	if len(in.Lines) == 0 || len(in.Lines) > 100 {
		writeError(w, http.StatusBadRequest, "Elige qué se devuelve.")
		return
	}
	p := principalFrom(r.Context())
	var out map[string]any
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		ctx := r.Context()
		st, err := loadReturnState(ctx, tx, p.ClinicID, saleID, true)
		if errors.Is(err, pgx.ErrNoRows) {
			return fail(http.StatusNotFound, "Venta no encontrada.")
		}
		if err != nil {
			return err
		}
		switch st.Status {
		case "void":
			return &httpError{Status: http.StatusConflict, Code: "SALE_VOID", Msg: "La venta está cancelada: no hay nada que devolver."}
		case "open":
			return &httpError{Status: http.StatusConflict, Code: "OPEN_SALE", Msg: "La venta todavía tiene saldo por cobrar. Cóbrala completa o cancélala."}
		}
		byID := map[string]*retLine{}
		for i := range st.Lines {
			byID[st.Lines[i].ID] = &st.Lines[i]
		}
		type pick struct {
			line              *retLine
			qty               float64
			cents, comm, base int
			restock           bool
		}
		var picks []pick
		seen := map[string]bool{}
		total := 0
		for i, li := range in.Lines {
			l := byID[li.SaleItemID]
			if l == nil || seen[li.SaleItemID] {
				return fail(http.StatusBadRequest, "Concepto "+itoa(i+1)+": no pertenece a esta venta.")
			}
			seen[li.SaleItemID] = true
			if li.Qty <= 0 || math.IsNaN(li.Qty) || math.IsInf(li.Qty, 0) {
				return fail(http.StatusBadRequest, "«"+l.Name+"»: escribe la cantidad que regresa.")
			}
			if li.Qty > l.remainingQty()+qtyEps {
				return &httpError{Status: http.StatusConflict, Code: "OVER_RETURN", Msg: "«" + l.Name + "»: solo se pueden devolver " + strconv.FormatFloat(l.remainingQty(), 'f', -1, 64) + " (ya hay devoluciones de esta venta)."}
			}
			qty := math.Round(li.Qty*1000) / 1000
			c, cm, bs := retAmount(*l, qty)
			restock := l.TracksStock && l.ItemID != nil && (li.Restock == nil || *li.Restock)
			picks = append(picks, pick{line: l, qty: qty, cents: c, comm: cm, base: bs, restock: restock})
			total += c
		}
		if total <= 0 {
			return fail(http.StatusBadRequest, "El monto a devolver debe ser mayor a cero.")
		}
		if st.ReturnedSum+total > st.TotalCents {
			return &httpError{Status: http.StatusConflict, Code: "OVER_RETURN", Msg: "Lo devuelto no puede superar el total de la venta."}
		}

		// How the money goes back.
		refunds := []methodTotal{}
		if len(in.Refunds) == 0 {
			refunds = suggestRefunds(st, total)
		} else {
			sum := 0
			used := map[string]bool{}
			for _, rf := range in.Refunds {
				if rf.AmountCents <= 0 || !okCents(rf.AmountCents) {
					return fail(http.StatusBadRequest, "Los montos de la devolución no son válidos.")
				}
				if used[rf.Method] {
					return fail(http.StatusBadRequest, "Cada método de pago solo puede aparecer una vez.")
				}
				used[rf.Method] = true
				if rf.AmountCents > st.Refundable[rf.Method] {
					return &httpError{Status: http.StatusConflict, Code: "REFUND_EXCEEDS", Msg: "No se puede devolver más de lo que se cobró por ese método."}
				}
				sum += rf.AmountCents
				refunds = append(refunds, methodTotal{Method: rf.Method, AmountCents: rf.AmountCents, Count: 1})
			}
			if sum != total {
				return &httpError{Status: http.StatusBadRequest, Code: "REFUND_MISMATCH", Msg: "Lo que devuelves ($" + cents(sum) + ") debe ser igual al total de la devolución ($" + cents(total) + ")."}
			}
		}
		refSum := 0
		for _, rf := range refunds {
			refSum += rf.AmountCents
		}
		if refSum != total {
			return &httpError{Status: http.StatusConflict, Code: "REFUND_EXCEEDS", Msg: "El dinero cobrado por los métodos de pago de esta venta ya no alcanza para esta devolución."}
		}

		// Cash goes out of the open register.
		var sessionID *string
		cashOut := 0
		for _, rf := range refunds {
			if rf.Method == "cash" {
				cashOut += rf.AmountCents
			}
		}
		var sid string
		serr := tx.QueryRow(ctx, `SELECT id FROM cash_sessions WHERE clinic_id=$1 AND closed_at IS NULL FOR UPDATE`, p.ClinicID).Scan(&sid)
		if serr == nil {
			sessionID = &sid
		} else if !errors.Is(serr, pgx.ErrNoRows) {
			return serr
		}
		if cashOut > 0 && sessionID == nil {
			return &httpError{Status: http.StatusConflict, Code: "CASH_CLOSED", Msg: "Abre la caja para devolver efectivo."}
		}

		var folio int
		if err := tx.QueryRow(ctx, `UPDATE clinics SET return_seq = return_seq + 1 WHERE id=$1 RETURNING return_seq`, p.ClinicID).Scan(&folio); err != nil {
			return err
		}
		var retID string
		var createdAt time.Time
		if err := tx.QueryRow(ctx, `
			INSERT INTO sale_returns (clinic_id, sale_id, folio, reason, total_cents, credit_note_pending, session_id, created_by, created_by_name)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id, created_at`,
			p.ClinicID, saleID, folio, in.Reason, total, st.Invoiced, sessionID, p.UserID, p.actorName()).Scan(&retID, &createdAt); err != nil {
			return err
		}
		note := "Devolución #" + itoa(folio) + " de la venta #" + itoa(st.Folio)
		outLines := []map[string]any{}
		for _, pk := range picks {
			if _, err := tx.Exec(ctx, `
				INSERT INTO sale_return_items (return_id, sale_item_id, item_id, professional_id, name, qty, amount_cents, commission_cents, commission_base_cents, restocked)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
				retID, pk.line.ID, pk.line.ItemID, pk.line.ProfessionalID, pk.line.Name, pk.qty, pk.cents, pk.comm, pk.base, pk.restock); err != nil {
				return err
			}
			if pk.restock {
				if err := restockReturned(ctx, tx, p.ClinicID, saleID, *pk.line.ItemID, pk.qty, note, p.actorName()); err != nil {
					return err
				}
			}
			outLines = append(outLines, map[string]any{"name": pk.line.Name, "qty": pk.qty, "amount_cents": pk.cents, "restocked": pk.restock})
		}
		for _, rf := range refunds {
			if _, err := tx.Exec(ctx, `INSERT INTO sale_return_refunds (return_id, method, amount_cents) VALUES ($1,$2,$3)`, retID, rf.Method, rf.AmountCents); err != nil {
				return err
			}
		}
		if cashOut > 0 {
			if _, err := tx.Exec(ctx, `INSERT INTO cash_movements (clinic_id, session_id, kind, amount_cents, concept, created_by, created_by_name) VALUES ($1,$2,'out',$3,$4,$5,$6)`,
				p.ClinicID, *sessionID, cashOut, note, p.UserID, p.actorName()); err != nil {
				return err
			}
		}
		// Card money last: if Mercado Pago refuses, the whole return rolls back and nothing changed.
		for _, rf := range refunds {
			if rf.Method == "mp_point" || rf.Method == "mp_link" {
				if err := s.refundMPPortion(ctx, tx, p.ClinicID, saleID, rf.Method, rf.AmountCents); err != nil {
					return err
				}
			}
		}
		audit(ctx, tx, p.ClinicID, p, "sale_return", "Devolución #"+itoa(folio)+" de la venta #"+itoa(st.Folio)+" por $"+cents(total)+": "+in.Reason,
			map[string]any{"folio": folio, "sale_folio": st.Folio, "total_cents": total, "credit_note_pending": st.Invoiced})
		outRefunds := []map[string]any{}
		for _, rf := range refunds {
			outRefunds = append(outRefunds, map[string]any{"method": rf.Method, "amount_cents": rf.AmountCents})
		}
		out = map[string]any{"return": map[string]any{
			"id": retID, "folio": folio, "sale_id": saleID, "sale_folio": st.Folio, "total_cents": total, "reason": in.Reason,
			"created_at": createdAt, "created_by_name": p.actorName(), "credit_note_pending": st.Invoiced,
			"lines": outLines, "refunds": outRefunds,
		}}
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// restockReturned gives qty back to the lots the sale took it from (what is still outstanding per lot).
func restockReturned(ctx context.Context, tx pgx.Tx, clinicID, saleID, itemID string, qty float64, note, actor string) error {
	rows, err := tx.Query(ctx, `
		SELECT coalesce(lot_id::text, ''), sum(-delta)::float8 FROM stock_movements
		WHERE clinic_id=$1 AND sale_id=$2 AND item_id=$3 AND reason IN ('sale', 'return')
		GROUP BY lot_id ORDER BY min(created_at), lot_id`, clinicID, saleID, itemID)
	if err != nil {
		return err
	}
	type slot struct {
		lot string
		qty float64
	}
	var slots []slot
	for rows.Next() {
		var sl slot
		if err := rows.Scan(&sl.lot, &sl.qty); err != nil {
			rows.Close()
			return err
		}
		if sl.qty > qtyEps {
			slots = append(slots, sl)
		}
	}
	rows.Close()
	if rows.Err() != nil {
		return rows.Err()
	}
	left := qty
	put := func(lot string, n float64) error {
		if lot == "" {
			var err error
			if lot, err = ensureLot(ctx, tx, clinicID, itemID, noLotCode, ""); err != nil {
				return err
			}
		}
		_, err := stockApply(ctx, tx, stockOp{clinicID: clinicID, itemID: itemID, lotID: lot, reason: "return", saleID: saleID, note: note, actor: actor}, n)
		return err
	}
	for _, sl := range slots {
		if left <= qtyEps {
			break
		}
		n := math.Min(left, sl.qty)
		if err := put(sl.lot, n); err != nil {
			return err
		}
		left -= n
	}
	if left > qtyEps {
		return put("", left)
	}
	return nil
}

// refundMPPortion refunds part of the Mercado Pago payments a sale was paid with. It runs inside the return's
// transaction and is the last step, so a refusal from Mercado Pago cancels the whole return.
func (s *Server) refundMPPortion(ctx context.Context, tx pgx.Tx, clinicID, saleID, method string, amount int) error {
	kind := map[string]string{"mp_point": "point", "mp_link": "link"}[method]
	rows, err := tx.Query(ctx, `
		SELECT id, coalesce(mp_payment_id, ''), amount_cents, refunded_cents FROM mp_charges
		WHERE clinic_id=$1 AND sale_id=$2 AND kind=$3 AND status='approved' AND refunded_cents < amount_cents ORDER BY created_at`, clinicID, saleID, kind)
	if err != nil {
		return err
	}
	type ch struct {
		id, pay          string
		amount, refunded int
	}
	var list []ch
	for rows.Next() {
		var c ch
		if err := rows.Scan(&c.id, &c.pay, &c.amount, &c.refunded); err != nil {
			rows.Close()
			return err
		}
		list = append(list, c)
	}
	rows.Close()
	if rows.Err() != nil {
		return rows.Err()
	}
	token, err := s.clinicToken(ctx, clinicID)
	if err != nil {
		return err
	}
	left := amount
	for _, c := range list {
		if left <= 0 {
			break
		}
		take := min(left, c.amount-c.refunded)
		full := c.refunded == 0 && take == c.amount
		var body any
		path := "/v1/orders/" + url.PathEscape(c.id) + "/refund"
		if c.pay == "" && kind == "point" && !full {
			var order struct {
				Transactions struct {
					Payments []struct {
						ID string `json:"id"`
					} `json:"payments"`
				} `json:"transactions"`
			}
			if err := s.mpCall(ctx, token, http.MethodGet, "/v1/orders/"+url.PathEscape(c.id), nil, &order); err == nil && len(order.Transactions.Payments) > 0 {
				c.pay = order.Transactions.Payments[0].ID
			}
		}
		switch kind {
		case "link":
			path = "/v1/payments/" + url.PathEscape(c.pay) + "/refunds"
			if !full {
				body = map[string]any{"amount": float64(take) / 100}
			}
		default:
			if !full {
				if c.pay == "" {
					return &httpError{Status: http.StatusBadGateway, Code: "refund_failed", Msg: "No encontré el pago en Mercado Pago para devolver una parte. Devuélvelo por otro método o cancela la venta completa."}
				}
				body = map[string]any{"transactions": []map[string]string{{"id": c.pay, "amount": cents(take)}}}
			}
		}
		if err := s.mpCall(ctx, token, http.MethodPost, path, body, nil); err != nil {
			var me *mpError
			already := errors.As(err, &me) && full && regexp.MustCompile(`(?i)already.*refund|ya.*reembols|fully refunded`).MatchString(me.Msg)
			if !already {
				msg := "Mercado Pago no pudo devolver el dinero"
				if me != nil && me.Msg != "" {
					msg += ": " + truncate(me.Msg, 160)
				}
				return &httpError{Status: http.StatusBadGateway, Code: "refund_failed", Msg: msg + ". La devolución no se registró."}
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE mp_charges SET refunded_cents = refunded_cents + $2, refunded_at = CASE WHEN refunded_cents + $2 >= amount_cents THEN now() ELSE refunded_at END WHERE id = $1`, c.id, take); err != nil {
			return err
		}
		left -= take
	}
	if left > 0 {
		return &httpError{Status: http.StatusConflict, Code: "refund_failed", Msg: "No encontré cobros aprobados de Mercado Pago suficientes para devolver ese monto."}
	}
	return nil
}
