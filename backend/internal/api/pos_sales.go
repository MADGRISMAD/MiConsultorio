package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// ---------------------------------------------------------------------------
// Request / response shapes
// ---------------------------------------------------------------------------

type saleLineIn struct {
	ItemID         string   `json:"item_id"`
	Name           string   `json:"name"` // for free-form lines (no catalog item)
	Qty            float64  `json:"qty"`
	UnitPriceCents *int     `json:"unit_price_cents"` // override; defaults to the catalog price
	DiscountCents  int      `json:"discount_cents"`
	TaxRate        *float64 `json:"tax_rate"` // free-form lines only
}

type salePaymentIn struct {
	Method        string `json:"method"`
	AmountCents   int    `json:"amount_cents"`
	ReceivedCents *int   `json:"received_cents"` // cash handed over; change is computed
	Reference     string `json:"reference"`
	IntentID      string `json:"intent_id"` // an approved Mercado Pago Point payment
}

type saleIn struct {
	Lines         []saleLineIn    `json:"lines"`
	DiscountCents int             `json:"discount_cents"` // on the whole ticket
	CustomerName  string          `json:"customer_name"`
	CustomerCURP  string          `json:"customer_curp"`
	AppointmentID string          `json:"appointment_id"`
	Note          string          `json:"note"`
	Payments      []salePaymentIn `json:"payments"`
}

type saleLine struct {
	ID             string  `json:"id"`
	ItemID         *string `json:"item_id"`
	Kind           string  `json:"kind"`
	Name           string  `json:"name"`
	Qty            float64 `json:"qty"`
	UnitPriceCents int     `json:"unit_price_cents"`
	TaxRate        float64 `json:"tax_rate"`
	DiscountCents  int     `json:"discount_cents"`
	TotalCents     int     `json:"total_cents"`
}

type salePayment struct {
	Method        string `json:"method"`
	AmountCents   int    `json:"amount_cents"`
	ReceivedCents *int   `json:"received_cents"`
	ChangeCents   int    `json:"change_cents"`
	Reference     string `json:"reference"`
}

type sale struct {
	ID            string        `json:"id"`
	Folio         int           `json:"folio"`
	CustomerName  string        `json:"customer_name"`
	CustomerCURP  string        `json:"customer_curp"`
	Note          string        `json:"note"`
	SubtotalCents int           `json:"subtotal_cents"`
	DiscountCents int           `json:"discount_cents"`
	TaxCents      int           `json:"tax_cents"`
	TotalCents    int           `json:"total_cents"`
	Status        string        `json:"status"`
	VoidReason    string        `json:"void_reason"`
	VoidedBy      string        `json:"voided_by"`
	CreatedBy     string        `json:"created_by"`
	CreatedAt     time.Time     `json:"created_at"`
	Lines         []saleLine    `json:"lines,omitempty"`
	Payments      []salePayment `json:"payments,omitempty"`
	Invoice       *string       `json:"invoice_status,omitempty"`
}

// ---------------------------------------------------------------------------
// Create a sale
// ---------------------------------------------------------------------------

// A line after pricing.
type pricedLine struct {
	in         saleLineIn
	itemID     *string
	kind, name string
	unitPrice  int
	unitCost   int
	taxRate    float64
	total      int // after the line discount
	track      bool
}

func (s *Server) createSale(w http.ResponseWriter, r *http.Request) {
	var in saleIn
	if !decode(w, r, &in) {
		return
	}
	p := principalFrom(r.Context())
	in.CustomerName, in.CustomerCURP = strings.TrimSpace(in.CustomerName), normalizeCURP(in.CustomerCURP)
	in.Note = strings.TrimSpace(in.Note)
	if len(in.Lines) == 0 || len(in.Lines) > 100 {
		writeError(w, http.StatusBadRequest, "Agrega entre 1 y 100 conceptos a la venta.")
		return
	}
	if utf8.RuneCountInString(in.CustomerName) > 200 || utf8.RuneCountInString(in.Note) > 300 {
		writeError(w, http.StatusBadRequest, "Uno de los campos es demasiado largo.")
		return
	}
	if in.AppointmentID != "" && !validUUID(in.AppointmentID) {
		writeError(w, http.StatusBadRequest, "La cita no es válida.")
		return
	}

	var out sale
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		cfg, err := loadPosSettings(r.Context(), tx, p.ClinicID)
		if err != nil {
			return err
		}
		// Serialize sales of one clinic: folio numbering and stock both depend on it.
		var folio int
		if err := tx.QueryRow(r.Context(), `UPDATE clinics SET sale_seq = sale_seq + 1 WHERE id = $1 RETURNING sale_seq`, p.ClinicID).Scan(&folio); err != nil {
			return err
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

		// ---- price the lines ----
		lines := make([]pricedLine, 0, len(in.Lines))
		for i, l := range in.Lines {
			if l.Qty <= 0 || l.Qty > 100_000 {
				return fail(http.StatusBadRequest, "Concepto "+itoa(i+1)+": la cantidad no es válida.")
			}
			if l.DiscountCents < 0 || !okCents(l.DiscountCents) {
				return fail(http.StatusBadRequest, "Concepto "+itoa(i+1)+": el descuento no es válido.")
			}
			pl := pricedLine{in: l, taxRate: cfg.DefaultTaxRate}
			if l.ItemID != "" {
				if !validUUID(l.ItemID) {
					return fail(http.StatusBadRequest, "Concepto "+itoa(i+1)+": artículo inválido.")
				}
				var stock float64
				var price, cost int
				err := tx.QueryRow(r.Context(), `
					SELECT kind, name, price_cents, cost_cents, tax_rate::float8, track_stock, stock::float8
					FROM catalog_items WHERE clinic_id=$1 AND id=$2 AND active FOR UPDATE`, p.ClinicID, l.ItemID).
					Scan(&pl.kind, &pl.name, &price, &cost, &pl.taxRate, &pl.track, &stock)
				if errors.Is(err, pgx.ErrNoRows) {
					return fail(http.StatusConflict, "Concepto "+itoa(i+1)+": el artículo ya no está disponible.")
				}
				if err != nil {
					return err
				}
				id := l.ItemID
				pl.itemID, pl.unitPrice, pl.unitCost = &id, price, cost
				if pl.track && !cfg.AllowNegativeStock && stock < l.Qty {
					return &httpError{Status: http.StatusConflict, Code: "NO_STOCK", Msg: "No hay existencias suficientes de «" + pl.name + "» (quedan " + strconv.FormatFloat(stock, 'f', -1, 64) + ")."}
				}
			} else {
				pl.name, pl.kind = strings.TrimSpace(l.Name), "service"
				if pl.name == "" || utf8.RuneCountInString(pl.name) > 160 {
					return fail(http.StatusBadRequest, "Concepto "+itoa(i+1)+": escribe un nombre.")
				}
				if l.UnitPriceCents == nil {
					return fail(http.StatusBadRequest, "Concepto "+itoa(i+1)+": escribe un precio.")
				}
				if l.TaxRate != nil {
					if *l.TaxRate < 0 || *l.TaxRate > 100 {
						return fail(http.StatusBadRequest, "Concepto "+itoa(i+1)+": IVA inválido.")
					}
					pl.taxRate = *l.TaxRate
				}
			}
			if l.UnitPriceCents != nil {
				if !okCents(*l.UnitPriceCents) {
					return fail(http.StatusBadRequest, "Concepto "+itoa(i+1)+": el precio no es válido.")
				}
				pl.unitPrice = *l.UnitPriceCents
			}
			gross := roundDiv(int64(pl.unitPrice)*int64(l.Qty*1000), 1000)
			if int64(l.DiscountCents) > gross {
				return fail(http.StatusBadRequest, "Concepto "+itoa(i+1)+": el descuento supera el importe.")
			}
			pl.total = int(gross) - l.DiscountCents
			lines = append(lines, pl)
		}

		var gross, lineDisc int
		for _, l := range lines {
			gross += l.total + l.in.DiscountCents
			lineDisc += l.in.DiscountCents
		}
		if in.DiscountCents < 0 || in.DiscountCents > gross-lineDisc {
			return fail(http.StatusBadRequest, "El descuento supera el total de la venta.")
		}
		if (lineDisc > 0 || in.DiscountCents > 0) && !cfg.AllowDiscounts {
			return fail(http.StatusForbidden, "Los descuentos están desactivados en los ajustes de cobros.")
		}
		totalDisc := lineDisc + in.DiscountCents
		if gross > 0 && cfg.MaxDiscountPct < 100 && int64(totalDisc)*100 > int64(gross)*int64(cfg.MaxDiscountPct) {
			return fail(http.StatusForbidden, "El descuento supera el máximo permitido ("+itoa(cfg.MaxDiscountPct)+" %).")
		}
		total := gross - totalDisc
		if total <= 0 {
			return fail(http.StatusBadRequest, "El total de la venta debe ser mayor a cero.")
		}
		// Prices already include tax; split it out of what was actually charged (ticket discount spread pro rata).
		tax := 0
		for _, l := range lines {
			share := l.total
			if sub := gross - lineDisc; sub > 0 && in.DiscountCents > 0 {
				share = int(roundDiv(int64(l.total)*int64(sub-in.DiscountCents), int64(sub)))
			}
			tax += int(roundDiv(int64(share)*int64(l.taxRate*100), int64(10000)+int64(l.taxRate*100)))
		}

		// ---- payments ----
		if len(in.Payments) == 0 || len(in.Payments) > 6 {
			return fail(http.StatusBadRequest, "Indica cómo se pagó la venta.")
		}
		paid := 0
		pays := make([]salePayment, 0, len(in.Payments))
		usedIntents := []string{}
		for _, pa := range in.Payments {
			if !hasPermission(cfg.Methods, pa.Method) {
				return fail(http.StatusBadRequest, "El método de pago «"+pa.Method+"» no está activo en tu negocio.")
			}
			if pa.AmountCents <= 0 || !okCents(pa.AmountCents) {
				return fail(http.StatusBadRequest, "El monto de un pago no es válido.")
			}
			sp := salePayment{Method: pa.Method, AmountCents: pa.AmountCents, Reference: strings.TrimSpace(pa.Reference)}
			if utf8.RuneCountInString(sp.Reference) > 80 {
				return fail(http.StatusBadRequest, "La referencia es demasiado larga.")
			}
			if pa.Method == "cash" && pa.ReceivedCents != nil {
				if *pa.ReceivedCents < pa.AmountCents || !okCents(*pa.ReceivedCents) {
					return fail(http.StatusBadRequest, "El efectivo recibido es menor al monto.")
				}
				sp.ReceivedCents, sp.ChangeCents = pa.ReceivedCents, *pa.ReceivedCents-pa.AmountCents
			}
			if pa.Method == "mp_point" || pa.Method == "mp_link" {
				ref, err := claimCharge(r.Context(), tx, p.ClinicID, map[string]string{"mp_point": "point", "mp_link": "link"}[pa.Method], pa.IntentID, pa.AmountCents)
				if err != nil {
					return err
				}
				sp.Reference = ref
				usedIntents = append(usedIntents, pa.IntentID)
			}
			paid += pa.AmountCents
			pays = append(pays, sp)
		}
		if paid != total {
			return fail(http.StatusBadRequest, "Los pagos ($"+cents(paid)+") no suman el total ($"+cents(total)+").")
		}

		// ---- write ----
		var saleID string
		var createdAt time.Time
		appt := any(nil)
		if in.AppointmentID != "" {
			appt = in.AppointmentID
		}
		if err := tx.QueryRow(r.Context(), `
			INSERT INTO sales (clinic_id, folio, session_id, customer_name, customer_curp, appointment_id, note,
			                   subtotal_cents, discount_cents, tax_cents, total_cents, created_by, created_by_name)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING id, created_at`,
			p.ClinicID, folio, sessionID, in.CustomerName, in.CustomerCURP, appt, in.Note,
			gross, totalDisc, tax, total, p.UserID, p.actorName()).Scan(&saleID, &createdAt); err != nil {
			return err
		}
		out = sale{ID: saleID, Folio: folio, CustomerName: in.CustomerName, CustomerCURP: in.CustomerCURP, Note: in.Note,
			SubtotalCents: gross, DiscountCents: totalDisc, TaxCents: tax, TotalCents: total, Status: "paid", CreatedBy: p.actorName(), CreatedAt: createdAt}
		for _, l := range lines {
			var lineID string
			if err := tx.QueryRow(r.Context(), `
				INSERT INTO sale_items (sale_id, item_id, kind, name, qty, unit_price_cents, unit_cost_cents, tax_rate, discount_cents, total_cents)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
				saleID, l.itemID, l.kind, l.name, l.in.Qty, l.unitPrice, l.unitCost, l.taxRate, l.in.DiscountCents, l.total).Scan(&lineID); err != nil {
				return err
			}
			out.Lines = append(out.Lines, saleLine{ID: lineID, ItemID: l.itemID, Kind: l.kind, Name: l.name, Qty: l.in.Qty,
				UnitPriceCents: l.unitPrice, TaxRate: l.taxRate, DiscountCents: l.in.DiscountCents, TotalCents: l.total})
			if l.track && l.itemID != nil {
				var bal float64
				if err := tx.QueryRow(r.Context(), `UPDATE catalog_items SET stock = stock - $2, updated_at = now() WHERE id = $1 RETURNING stock::float8`, *l.itemID, l.in.Qty).Scan(&bal); err != nil {
					return err
				}
				if _, err := tx.Exec(r.Context(), `
					INSERT INTO stock_movements (clinic_id, item_id, delta, reason, sale_id, balance, created_by_name, note)
					VALUES ($1,$2,$3,'sale',$4,$5,$6,$7)`, p.ClinicID, *l.itemID, -l.in.Qty, saleID, bal, p.actorName(), "Venta #"+itoa(folio)); err != nil {
					return err
				}
			}
		}
		for _, sp := range pays {
			if _, err := tx.Exec(r.Context(), `
				INSERT INTO sale_payments (sale_id, method, amount_cents, received_cents, change_cents, reference) VALUES ($1,$2,$3,$4,$5,$6)`,
				saleID, sp.Method, sp.AmountCents, sp.ReceivedCents, sp.ChangeCents, sp.Reference); err != nil {
				return err
			}
		}
		out.Payments = pays
		for _, id := range usedIntents {
			if _, err := tx.Exec(r.Context(), `UPDATE mp_charges SET sale_id = $2 WHERE id = $1`, id, saleID); err != nil {
				return err
			}
		}
		audit(r.Context(), tx, p.ClinicID, p, "sale", "Cobró la venta #"+itoa(folio)+" por $"+cents(total), map[string]any{"folio": folio, "total_cents": total})
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"sale": out})
}

func cents(n int) string {
	neg := ""
	if n < 0 {
		neg, n = "-", -n
	}
	return neg + strconv.Itoa(n/100) + "." + strings.Repeat("0", 2-len(strconv.Itoa(n%100))) + strconv.Itoa(n%100)
}

// ---------------------------------------------------------------------------
// List / read / void
// ---------------------------------------------------------------------------

func (s *Server) listSales(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	q := r.URL.Query()
	from, to, msg := dayRange(q.Get("from"), q.Get("to"))
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	where, args := []string{"s.clinic_id = $1", "s.created_at >= $2", "s.created_at < $3"}, []any{p.ClinicID, from, to}
	if st := q.Get("status"); st == "paid" || st == "void" {
		args = append(args, st)
		where = append(where, "s.status = $"+itoa(len(args)))
	}
	if text := strings.TrimSpace(q.Get("q")); text != "" {
		args = append(args, "%"+escapeLike(text)+"%", text)
		n := len(args)
		where = append(where, "(s.customer_name ILIKE $"+itoa(n-1)+" OR s.customer_curp ILIKE $"+itoa(n-1)+" OR s.folio::text = $"+itoa(n)+")")
	}
	if q.Get("invoiceable") == "1" {
		where = append(where, "s.status = 'paid' AND NOT EXISTS (SELECT 1 FROM invoice_requests i WHERE i.sale_id = s.id AND i.status <> 'cancelled')")
	}
	limit := 200
	if v, err := strconv.Atoi(q.Get("limit")); err == nil && v > 0 && v <= 1000 {
		limit = v
	}
	rows, err := s.db.Query(r.Context(), `
		SELECT s.id, s.folio, s.customer_name, s.customer_curp, s.note, s.subtotal_cents, s.discount_cents, s.tax_cents, s.total_cents,
		       s.status, s.void_reason, s.voided_by_name, s.created_by_name, s.created_at,
		       (SELECT i.status FROM invoice_requests i WHERE i.sale_id = s.id AND i.status <> 'cancelled' LIMIT 1)
		FROM sales s WHERE `+strings.Join(where, " AND ")+` ORDER BY s.created_at DESC LIMIT `+itoa(limit), args...)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	list := []sale{}
	for rows.Next() {
		var x sale
		if err := rows.Scan(&x.ID, &x.Folio, &x.CustomerName, &x.CustomerCURP, &x.Note, &x.SubtotalCents, &x.DiscountCents, &x.TaxCents, &x.TotalCents,
			&x.Status, &x.VoidReason, &x.VoidedBy, &x.CreatedBy, &x.CreatedAt, &x.Invoice); err != nil {
			serverError(w, r, err)
			return
		}
		list = append(list, x)
	}
	if rows.Err() != nil {
		serverError(w, r, rows.Err())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sales": list})
}

func loadSale(ctx context.Context, q interface {
	queryRower
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, clinicID, id string) (sale, error) {
	var x sale
	err := q.QueryRow(ctx, `
		SELECT s.id, s.folio, s.customer_name, s.customer_curp, s.note, s.subtotal_cents, s.discount_cents, s.tax_cents, s.total_cents,
		       s.status, s.void_reason, s.voided_by_name, s.created_by_name, s.created_at,
		       (SELECT i.status FROM invoice_requests i WHERE i.sale_id = s.id AND i.status <> 'cancelled' LIMIT 1)
		FROM sales s WHERE s.clinic_id = $1 AND s.id = $2`, clinicID, id).
		Scan(&x.ID, &x.Folio, &x.CustomerName, &x.CustomerCURP, &x.Note, &x.SubtotalCents, &x.DiscountCents, &x.TaxCents, &x.TotalCents,
			&x.Status, &x.VoidReason, &x.VoidedBy, &x.CreatedBy, &x.CreatedAt, &x.Invoice)
	if err != nil {
		return x, err
	}
	rows, err := q.Query(ctx, `SELECT id, item_id::text, kind, name, qty::float8, unit_price_cents, tax_rate::float8, discount_cents, total_cents FROM sale_items WHERE sale_id = $1 ORDER BY name`, id)
	if err != nil {
		return x, err
	}
	for rows.Next() {
		var l saleLine
		if err := rows.Scan(&l.ID, &l.ItemID, &l.Kind, &l.Name, &l.Qty, &l.UnitPriceCents, &l.TaxRate, &l.DiscountCents, &l.TotalCents); err != nil {
			rows.Close()
			return x, err
		}
		x.Lines = append(x.Lines, l)
	}
	rows.Close()
	prows, err := q.Query(ctx, `SELECT method, amount_cents, received_cents, change_cents, reference FROM sale_payments WHERE sale_id = $1 ORDER BY created_at, id`, id)
	if err != nil {
		return x, err
	}
	for prows.Next() {
		var sp salePayment
		if err := prows.Scan(&sp.Method, &sp.AmountCents, &sp.ReceivedCents, &sp.ChangeCents, &sp.Reference); err != nil {
			prows.Close()
			return x, err
		}
		x.Payments = append(x.Payments, sp)
	}
	prows.Close()
	return x, nil
}

func (s *Server) getSale(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Venta no encontrada.")
		return
	}
	x, err := loadSale(r.Context(), s.db, principalFrom(r.Context()).ClinicID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Venta no encontrada.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sale": x})
}

// voidSale cancels a sale, returns its stock and records why. Cash taken by a voided sale
// is excluded from the register's expected cash while its session is still open.
func (s *Server) voidSale(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Venta no encontrada.")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" || utf8.RuneCountInString(req.Reason) > 200 {
		writeError(w, http.StatusBadRequest, "Escribe el motivo de la cancelación (máximo 200 caracteres).")
		return
	}
	p := principalFrom(r.Context())
	// Card money goes back first (idempotent); if Mercado Pago refuses, the sale stays as it was.
	var st string
	if err := s.db.QueryRow(r.Context(), `SELECT status FROM sales WHERE clinic_id=$1 AND id=$2`, p.ClinicID, id).Scan(&st); err == nil && st == "paid" {
		var invoiced bool
		_ = s.db.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM invoice_requests WHERE sale_id=$1 AND status='issued')`, id).Scan(&invoiced)
		if !invoiced {
			if err := s.refundCharges(r.Context(), p.ClinicID, id); err != nil {
				writeFailure(w, r, err)
				return
			}
		}
	}
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		var status string
		var folio int
		err := tx.QueryRow(r.Context(), `SELECT status, folio FROM sales WHERE clinic_id=$1 AND id=$2 FOR UPDATE`, p.ClinicID, id).Scan(&status, &folio)
		if errors.Is(err, pgx.ErrNoRows) {
			return fail(http.StatusNotFound, "Venta no encontrada.")
		}
		if err != nil {
			return err
		}
		if status == "void" {
			return fail(http.StatusConflict, "La venta ya estaba cancelada.")
		}
		var invoiced bool
		if err := tx.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM invoice_requests WHERE sale_id=$1 AND status='issued')`, id).Scan(&invoiced); err != nil {
			return err
		}
		if invoiced {
			return fail(http.StatusConflict, "La venta ya tiene una factura emitida; cancela primero la factura.")
		}
		if _, err := tx.Exec(r.Context(), `UPDATE sales SET status='void', void_reason=$3, voided_at=now(), voided_by_name=$4 WHERE clinic_id=$1 AND id=$2`,
			p.ClinicID, id, req.Reason, p.actorName()); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `UPDATE invoice_requests SET status='cancelled', updated_at=now() WHERE sale_id=$1 AND status='pending'`, id); err != nil {
			return err
		}
		rows, err := tx.Query(r.Context(), `
			SELECT si.item_id, si.qty::float8 FROM sale_items si JOIN catalog_items c ON c.id = si.item_id
			WHERE si.sale_id = $1 AND c.track_stock`, id)
		if err != nil {
			return err
		}
		type back struct {
			item string
			qty  float64
		}
		var backs []back
		for rows.Next() {
			var b back
			if err := rows.Scan(&b.item, &b.qty); err != nil {
				rows.Close()
				return err
			}
			backs = append(backs, b)
		}
		rows.Close()
		for _, b := range backs {
			var bal float64
			if err := tx.QueryRow(r.Context(), `UPDATE catalog_items SET stock = stock + $2, updated_at=now() WHERE id=$1 RETURNING stock::float8`, b.item, b.qty).Scan(&bal); err != nil {
				return err
			}
			if _, err := tx.Exec(r.Context(), `
				INSERT INTO stock_movements (clinic_id, item_id, delta, reason, sale_id, balance, created_by_name, note)
				VALUES ($1,$2,$3,'void',$4,$5,$6,$7)`, p.ClinicID, b.item, b.qty, id, bal, p.actorName(), "Cancelación de la venta #"+itoa(folio)); err != nil {
				return err
			}
		}
		audit(r.Context(), tx, p.ClinicID, p, "sale_void", "Canceló la venta #"+itoa(folio)+": "+req.Reason, map[string]any{"folio": folio})
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
