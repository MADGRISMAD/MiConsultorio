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
	TaxRate        *float64 `json:"tax_rate"`        // free-form lines only
	ProfessionalID string   `json:"professional_id"` // credited with this line; defaults to the sale's professional
}

type salePaymentIn struct {
	Method        string `json:"method"`
	AmountCents   int    `json:"amount_cents"`
	ReceivedCents *int   `json:"received_cents"` // cash handed over; change is computed
	Reference     string `json:"reference"`
	IntentID      string `json:"intent_id"` // an approved Mercado Pago Point payment
}

type saleIn struct {
	Lines          []saleLineIn    `json:"lines"`
	DiscountCents  int             `json:"discount_cents"` // on the whole ticket
	CustomerName   string          `json:"customer_name"`
	CustomerCURP   string          `json:"customer_curp"`
	PatientID      string          `json:"patient_id"`
	ProfessionalID string          `json:"professional_id"`
	AppointmentID  string          `json:"appointment_id"`
	PlanItemIDs    []string        `json:"plan_item_ids"` // treatment plan items this sale charges
	Note           string          `json:"note"`
	Payments       []salePaymentIn `json:"payments"`
	// OnAccount leaves what the payments do not cover as a balance to collect later (abonos).
	OnAccount bool `json:"on_account"`
	// AllowExpired sells expired lots anyway. Administrators only, with a reason.
	AllowExpired  bool   `json:"allow_expired"`
	ExpiredReason string `json:"expired_reason"`
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
	Method        string    `json:"method"`
	AmountCents   int       `json:"amount_cents"`
	ReceivedCents *int      `json:"received_cents"`
	ChangeCents   int       `json:"change_cents"`
	Reference     string    `json:"reference"`
	CreatedAt     time.Time `json:"created_at"`
}

type sale struct {
	ID               string        `json:"id"`
	Folio            int           `json:"folio"`
	CustomerName     string        `json:"customer_name"`
	CustomerCURP     string        `json:"customer_curp"`
	Note             string        `json:"note"`
	SubtotalCents    int           `json:"subtotal_cents"`
	DiscountCents    int           `json:"discount_cents"`
	TaxCents         int           `json:"tax_cents"`
	TotalCents       int           `json:"total_cents"`
	PaidCents        int           `json:"paid_cents"`
	BalanceCents     int           `json:"balance_cents"`
	OnCredit         bool          `json:"on_credit"`
	Status           string        `json:"status"`
	VoidReason       string        `json:"void_reason"`
	VoidedBy         string        `json:"voided_by"`
	CreatedBy        string        `json:"created_by"`
	CreatedAt        time.Time     `json:"created_at"`
	ProfessionalID   *string       `json:"professional_id"`
	ProfessionalName string        `json:"professional_name"`
	PatientID        *string       `json:"patient_id"`
	AppointmentID    *string       `json:"appointment_id"`
	Lines            []saleLine    `json:"lines,omitempty"`
	Payments         []salePayment `json:"payments,omitempty"`
	Invoice          *string       `json:"invoice_status,omitempty"`
	Warnings         []string      `json:"warnings,omitempty"`
}

// ---------------------------------------------------------------------------
// Create a sale
// ---------------------------------------------------------------------------

// A line after pricing.
type pricedLine struct {
	in         saleLineIn
	itemID     *string
	kind, name string
	category   string
	unitPrice  int
	unitCost   int
	taxRate    float64
	total      int // after the line discount
	taxPart    int // tax contained in what is charged for the line (ticket discount included)
	sharePart  int // what is charged for the line after the ticket discount
	track      bool
}

// parsedPayments are validated payments ready to be stored.
type parsedPayments struct {
	list    []salePayment
	intents []string
	paid    int
}

// parsePayments validates payments against the enabled methods and claims Mercado Pago charges.
func parsePayments(ctx context.Context, tx pgx.Tx, clinicID string, cfg PosSettings, ins []salePaymentIn) (parsedPayments, error) {
	var out parsedPayments
	for _, pa := range ins {
		if !hasPermission(cfg.Methods, pa.Method) {
			return out, fail(http.StatusBadRequest, "El método de pago «"+pa.Method+"» no está activo en tu negocio.")
		}
		if pa.AmountCents <= 0 || !okCents(pa.AmountCents) {
			return out, fail(http.StatusBadRequest, "El monto de un pago no es válido.")
		}
		sp := salePayment{Method: pa.Method, AmountCents: pa.AmountCents, Reference: strings.TrimSpace(pa.Reference)}
		if utf8.RuneCountInString(sp.Reference) > 80 {
			return out, fail(http.StatusBadRequest, "La referencia es demasiado larga.")
		}
		if pa.Method == "cash" && pa.ReceivedCents != nil {
			if *pa.ReceivedCents < pa.AmountCents || !okCents(*pa.ReceivedCents) {
				return out, fail(http.StatusBadRequest, "El efectivo recibido es menor al monto.")
			}
			sp.ReceivedCents, sp.ChangeCents = pa.ReceivedCents, *pa.ReceivedCents-pa.AmountCents
		}
		if pa.Method == "mp_point" || pa.Method == "mp_link" {
			ref, err := claimCharge(ctx, tx, clinicID, map[string]string{"mp_point": "point", "mp_link": "link"}[pa.Method], pa.IntentID, pa.AmountCents)
			if err != nil {
				return out, err
			}
			sp.Reference = ref
			out.intents = append(out.intents, pa.IntentID)
		}
		out.paid += pa.AmountCents
		out.list = append(out.list, sp)
	}
	return out, nil
}

func (s *Server) createSale(w http.ResponseWriter, r *http.Request) {
	var in saleIn
	if !decode(w, r, &in) {
		return
	}
	p := principalFrom(r.Context())
	in.CustomerName, in.CustomerCURP = strings.TrimSpace(in.CustomerName), normalizeCURP(in.CustomerCURP)
	in.Note, in.ExpiredReason = strings.TrimSpace(in.Note), strings.TrimSpace(in.ExpiredReason)
	if len(in.Lines) == 0 || len(in.Lines) > 100 {
		writeError(w, http.StatusBadRequest, "Agrega entre 1 y 100 conceptos a la venta.")
		return
	}
	if utf8.RuneCountInString(in.CustomerName) > 200 || utf8.RuneCountInString(in.Note) > 300 {
		writeError(w, http.StatusBadRequest, "Uno de los campos es demasiado largo.")
		return
	}
	for _, f := range []struct{ v, msg string }{
		{in.AppointmentID, "La cita no es válida."}, {in.PatientID, "El paciente no es válido."}, {in.ProfessionalID, "El profesional no es válido."},
	} {
		if f.v != "" && !validUUID(f.v) {
			writeError(w, http.StatusBadRequest, f.msg)
			return
		}
	}
	if len(in.PlanItemIDs) > 100 {
		writeError(w, http.StatusBadRequest, "Demasiados conceptos del plan.")
		return
	}
	for _, id := range in.PlanItemIDs {
		if !validUUID(id) {
			writeError(w, http.StatusBadRequest, "Un concepto del plan no es válido.")
			return
		}
	}
	if in.AllowExpired {
		if p.Role != RoleAdmin {
			writeError(w, http.StatusForbidden, "Solo un administrador puede vender lotes caducados.")
			return
		}
		if in.ExpiredReason == "" || utf8.RuneCountInString(in.ExpiredReason) > 200 {
			writeError(w, http.StatusBadRequest, "Escribe el motivo para vender lotes caducados (máximo 200 caracteres).")
			return
		}
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

		// ---- who, and which visit ----
		professionalID, patientID := in.ProfessionalID, in.PatientID
		if in.AppointmentID != "" {
			var apPatient, apPro *string
			err := tx.QueryRow(r.Context(), `SELECT patient_id::text, professional_id::text FROM appointments WHERE clinic_id = $1 AND id = $2 FOR UPDATE`,
				p.ClinicID, in.AppointmentID).Scan(&apPatient, &apPro)
			if errors.Is(err, pgx.ErrNoRows) {
				return fail(http.StatusBadRequest, "La cita no existe en este consultorio.")
			}
			if err != nil {
				return err
			}
			if patientID == "" && apPatient != nil {
				patientID = *apPatient
			}
			if professionalID == "" && apPro != nil {
				professionalID = *apPro
			}
		}
		if patientID != "" {
			var names, last string
			err := tx.QueryRow(r.Context(), `SELECT names, last_names FROM patients WHERE clinic_id = $1 AND id = $2`, p.ClinicID, patientID).Scan(&names, &last)
			if errors.Is(err, pgx.ErrNoRows) {
				return fail(http.StatusBadRequest, "El paciente no existe en este consultorio.")
			}
			if err != nil {
				return err
			}
			if in.CustomerName == "" {
				in.CustomerName = strings.TrimSpace(names + " " + last)
			}
		}
		checked := map[string]bool{}
		checkPro := func(id string) error {
			if id == "" || checked[id] {
				return nil
			}
			ok, err := validProfessional(r.Context(), tx, p.ClinicID, id)
			if err != nil {
				return err
			}
			if !ok {
				return fail(http.StatusBadRequest, "El profesional no existe en este consultorio.")
			}
			checked[id] = true
			return nil
		}
		if err := checkPro(professionalID); err != nil {
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
			if l.ProfessionalID != "" && !validUUID(l.ProfessionalID) {
				return fail(http.StatusBadRequest, "Concepto "+itoa(i+1)+": el profesional no es válido.")
			}
			if err := checkPro(l.ProfessionalID); err != nil {
				return err
			}
			pl := pricedLine{in: l, taxRate: cfg.DefaultTaxRate}
			if l.ItemID != "" {
				if !validUUID(l.ItemID) {
					return fail(http.StatusBadRequest, "Concepto "+itoa(i+1)+": artículo inválido.")
				}
				var stock float64
				var price, cost int
				err := tx.QueryRow(r.Context(), `
					SELECT kind, name, category, price_cents, cost_cents, tax_rate::float8, track_stock, stock::float8
					FROM catalog_items WHERE clinic_id=$1 AND id=$2 AND active FOR UPDATE`, p.ClinicID, l.ItemID).
					Scan(&pl.kind, &pl.name, &pl.category, &price, &cost, &pl.taxRate, &pl.track, &stock)
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
		for i := range lines {
			l := &lines[i]
			l.sharePart = l.total
			if sub := gross - lineDisc; sub > 0 && in.DiscountCents > 0 {
				l.sharePart = int(roundDiv(int64(l.total)*int64(sub-in.DiscountCents), int64(sub)))
			}
			l.taxPart = int(roundDiv(int64(l.sharePart)*int64(l.taxRate*100), int64(10000)+int64(l.taxRate*100)))
			tax += l.taxPart
		}

		// ---- payments ----
		if len(in.Payments) > 6 || (len(in.Payments) == 0 && !in.OnAccount) {
			return fail(http.StatusBadRequest, "Indica cómo se pagó la venta.")
		}
		pp, err := parsePayments(r.Context(), tx, p.ClinicID, cfg, in.Payments)
		if err != nil {
			return err
		}
		status, balance := "paid", 0
		switch {
		case in.OnAccount:
			if pp.paid > total {
				return fail(http.StatusBadRequest, "Los pagos ($"+cents(pp.paid)+") superan el total ($"+cents(total)+").")
			}
			if in.CustomerName == "" && patientID == "" {
				return fail(http.StatusBadRequest, "Para cobrar a abonos indica el paciente o el nombre del cliente.")
			}
			if pp.paid < total {
				status, balance = "open", total-pp.paid
			}
		case pp.paid != total:
			return fail(http.StatusBadRequest, "Los pagos ($"+cents(pp.paid)+") no suman el total ($"+cents(total)+").")
		}

		// ---- write ----
		var saleID string
		var createdAt time.Time
		nilIf := func(v string) any {
			if v == "" {
				return nil
			}
			return v
		}
		if err := tx.QueryRow(r.Context(), `
			INSERT INTO sales (clinic_id, folio, session_id, customer_name, customer_curp, appointment_id, note,
			                   subtotal_cents, discount_cents, tax_cents, total_cents, created_by, created_by_name,
			                   professional_id, patient_id, status, balance_cents, on_credit)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18) RETURNING id, created_at`,
			p.ClinicID, folio, sessionID, in.CustomerName, in.CustomerCURP, nilIf(in.AppointmentID), in.Note,
			gross, totalDisc, tax, total, p.UserID, p.actorName(),
			nilIf(professionalID), nilIf(patientID), status, balance, in.OnAccount && status == "open").Scan(&saleID, &createdAt); err != nil {
			return err
		}
		out = sale{ID: saleID, Folio: folio, CustomerName: in.CustomerName, CustomerCURP: in.CustomerCURP, Note: in.Note,
			SubtotalCents: gross, DiscountCents: totalDisc, TaxCents: tax, TotalCents: total, PaidCents: pp.paid, BalanceCents: balance,
			OnCredit: in.OnAccount && status == "open", Status: status, CreatedBy: p.actorName(), CreatedAt: createdAt}
		if professionalID != "" {
			out.ProfessionalID = &professionalID
		}
		if patientID != "" {
			out.PatientID = &patientID
		}
		if in.AppointmentID != "" {
			out.AppointmentID = &in.AppointmentID
		}
		rules, err := loadCommissionRules(r.Context(), tx, p.ClinicID, true)
		if err != nil {
			return err
		}
		for _, l := range lines {
			linePro := l.in.ProfessionalID
			if linePro == "" {
				linePro = professionalID
			}
			itemID := ""
			if l.itemID != nil {
				itemID = *l.itemID
			}
			pct := pickCommission(rules, linePro, itemID, l.category)
			base := l.sharePart - l.taxPart
			commission := int(roundDiv(int64(base)*int64(pct*100), 10000))
			var lineID string
			if err := tx.QueryRow(r.Context(), `
				INSERT INTO sale_items (sale_id, item_id, kind, name, qty, unit_price_cents, unit_cost_cents, tax_rate, discount_cents, total_cents,
				                        professional_id, commission_pct, commission_base_cents, commission_cents)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING id`,
				saleID, l.itemID, l.kind, l.name, l.in.Qty, l.unitPrice, l.unitCost, l.taxRate, l.in.DiscountCents, l.total,
				nilIf(l.in.ProfessionalID), pct, base, commission).Scan(&lineID); err != nil {
				return err
			}
			out.Lines = append(out.Lines, saleLine{ID: lineID, ItemID: l.itemID, Kind: l.kind, Name: l.name, Qty: l.in.Qty,
				UnitPriceCents: l.unitPrice, TaxRate: l.taxRate, DiscountCents: l.in.DiscountCents, TotalCents: l.total})
			if l.track && l.itemID != nil {
				res, err := consumeStock(r.Context(), tx, stockOp{clinicID: p.ClinicID, itemID: *l.itemID, reason: "sale", saleID: saleID,
					note: "Venta #" + itoa(folio), actor: p.actorName()}, l.in.Qty, in.AllowExpired, cfg.AllowNegativeStock)
				if err != nil {
					return err
				}
				if res.Short > qtyEps {
					if res.Expired > 0 {
						return &httpError{Status: http.StatusConflict, Code: "LOT_EXPIRED", Msg: "Las existencias disponibles de «" + l.name + "» están caducadas y no se pueden vender."}
					}
					return &httpError{Status: http.StatusConflict, Code: "NO_STOCK", Msg: "No hay existencias suficientes de «" + l.name + "»."}
				}
			}
			if l.kind == "service" && l.itemID != nil {
				warns, err := consumeForService(r.Context(), tx, p.ClinicID, *l.itemID, l.name, l.in.Qty, saleID, p.actorName(), in.AllowExpired, cfg.AllowNegativeStock)
				if err != nil {
					return err
				}
				out.Warnings = append(out.Warnings, warns...)
			}
		}
		for i := range pp.list {
			sp := &pp.list[i]
			if err := tx.QueryRow(r.Context(), `
				INSERT INTO sale_payments (sale_id, method, amount_cents, received_cents, change_cents, reference, session_id) VALUES ($1,$2,$3,$4,$5,$6,$7)
				RETURNING created_at`,
				saleID, sp.Method, sp.AmountCents, sp.ReceivedCents, sp.ChangeCents, sp.Reference, sessionID).Scan(&sp.CreatedAt); err != nil {
				return err
			}
		}
		out.Payments = pp.list
		for _, id := range pp.intents {
			if _, err := tx.Exec(r.Context(), `UPDATE mp_charges SET sale_id = $2 WHERE id = $1`, id, saleID); err != nil {
				return err
			}
		}
		if in.AppointmentID != "" {
			// the visit is done once it is charged; a visit that already has a live sale keeps pointing at it
			if _, err := tx.Exec(r.Context(), `
				UPDATE appointments a SET status = 'completed', finished_at = coalesce(a.finished_at, now()),
				       sale_id = CASE WHEN a.sale_id IS NULL OR EXISTS (SELECT 1 FROM sales o WHERE o.id = a.sale_id AND o.status = 'void') THEN $3::uuid ELSE a.sale_id END
				WHERE a.clinic_id = $1 AND a.id = $2`, p.ClinicID, in.AppointmentID, saleID); err != nil {
				return err
			}
		}
		if len(in.PlanItemIDs) > 0 {
			markPlanItems(r.Context(), tx, p.ClinicID, saleID, in.PlanItemIDs)
		}
		if in.AllowExpired {
			audit(r.Context(), tx, p.ClinicID, p, "sale_expired_override", "Vendió con lotes caducados autorizados en la venta #"+itoa(folio)+": "+in.ExpiredReason, map[string]any{"folio": folio})
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

// markPlanItems links treatment plan items to the sale that charges them. The plan tables belong to another
// module and may not exist (or may have changed), so a failure here never blocks the sale.
func markPlanItems(ctx context.Context, tx pgx.Tx, clinicID, saleID string, ids []string) {
	sp, err := tx.Begin(ctx)
	if err != nil {
		return
	}
	if _, err := sp.Exec(ctx, `UPDATE treatment_plan_items SET sale_id = $2 WHERE clinic_id = $1 AND id = ANY($3::uuid[]) AND status <> 'cancelled'`,
		clinicID, saleID, ids); err != nil {
		_ = sp.Rollback(ctx)
		return
	}
	_ = sp.Commit(ctx)
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

const saleSelect = `
	SELECT s.id, s.folio, s.customer_name, s.customer_curp, s.note, s.subtotal_cents, s.discount_cents, s.tax_cents, s.total_cents,
	       s.balance_cents, s.on_credit, s.status, s.void_reason, s.voided_by_name, s.created_by_name, s.created_at,
	       s.professional_id::text, coalesce(pu.name, ''), s.patient_id::text, s.appointment_id::text,
	       (SELECT coalesce(sum(amount_cents), 0) FROM sale_payments WHERE sale_id = s.id)::int,
	       (SELECT i.status FROM invoice_requests i WHERE i.sale_id = s.id AND i.status <> 'cancelled' LIMIT 1)
	FROM sales s LEFT JOIN users pu ON pu.id = s.professional_id`

func scanSale(row pgx.Row, x *sale) error {
	return row.Scan(&x.ID, &x.Folio, &x.CustomerName, &x.CustomerCURP, &x.Note, &x.SubtotalCents, &x.DiscountCents, &x.TaxCents, &x.TotalCents,
		&x.BalanceCents, &x.OnCredit, &x.Status, &x.VoidReason, &x.VoidedBy, &x.CreatedBy, &x.CreatedAt,
		&x.ProfessionalID, &x.ProfessionalName, &x.PatientID, &x.AppointmentID, &x.PaidCents, &x.Invoice)
}

func (s *Server) listSales(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	q := r.URL.Query()
	from, to, msg := dayRange(q.Get("from"), q.Get("to"))
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	where, args := []string{"s.clinic_id = $1", "s.created_at >= $2", "s.created_at < $3"}, []any{p.ClinicID, from, to}
	if st := q.Get("status"); st == "paid" || st == "void" || st == "open" {
		args = append(args, st)
		where = append(where, "s.status = $"+itoa(len(args)))
	}
	if id := q.Get("patient_id"); validUUID(id) {
		args = append(args, id)
		where = append(where, "s.patient_id = $"+itoa(len(args)))
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
	rows, err := s.db.Query(r.Context(), saleSelect+` WHERE `+strings.Join(where, " AND ")+` ORDER BY s.created_at DESC LIMIT `+itoa(limit), args...)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	list := []sale{}
	for rows.Next() {
		var x sale
		if err := scanSale(rows, &x); err != nil {
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
	if err := scanSale(q.QueryRow(ctx, saleSelect+` WHERE s.clinic_id = $1 AND s.id = $2`, clinicID, id), &x); err != nil {
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
	prows, err := q.Query(ctx, `SELECT method, amount_cents, received_cents, change_cents, reference, created_at FROM sale_payments WHERE sale_id = $1 ORDER BY created_at, id`, id)
	if err != nil {
		return x, err
	}
	for prows.Next() {
		var sp salePayment
		if err := prows.Scan(&sp.Method, &sp.AmountCents, &sp.ReceivedCents, &sp.ChangeCents, &sp.Reference, &sp.CreatedAt); err != nil {
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

// voidSale cancels a sale, returns its stock (to the lots it came from) and records why. Cash taken by a
// voided sale is excluded from the register's expected cash while its session is still open. For a sale on
// account only what was actually paid is given back: the response says how much and by which method, since
// cash, card and transfer refunds are made by hand (Mercado Pago ones are returned automatically).
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
	if err := s.db.QueryRow(r.Context(), `SELECT status FROM sales WHERE clinic_id=$1 AND id=$2`, p.ClinicID, id).Scan(&st); err == nil && (st == "paid" || st == "open") {
		var invoiced bool
		_ = s.db.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM invoice_requests WHERE sale_id=$1 AND status='issued')`, id).Scan(&invoiced)
		if !invoiced {
			if err := s.refundCharges(r.Context(), p.ClinicID, id); err != nil {
				writeFailure(w, r, err)
				return
			}
		}
	}
	var refunds []methodTotal
	refundTotal := 0
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
		if _, err := tx.Exec(r.Context(), `UPDATE sales SET status='void', balance_cents=0, void_reason=$3, voided_at=now(), voided_by_name=$4 WHERE clinic_id=$1 AND id=$2`,
			p.ClinicID, id, req.Reason, p.actorName()); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `UPDATE invoice_requests SET status='cancelled', updated_at=now() WHERE sale_id=$1 AND status='pending'`, id); err != nil {
			return err
		}
		if err := restockSale(r.Context(), tx, p.ClinicID, id, "Cancelación de la venta #"+itoa(folio), p.actorName()); err != nil {
			return err
		}
		rows, err := tx.Query(r.Context(), `SELECT method, sum(amount_cents)::int, count(*) FROM sale_payments WHERE sale_id = $1 GROUP BY method ORDER BY method`, id)
		if err != nil {
			return err
		}
		for rows.Next() {
			var m methodTotal
			if err := rows.Scan(&m.Method, &m.AmountCents, &m.Count); err != nil {
				rows.Close()
				return err
			}
			refunds = append(refunds, m)
			refundTotal += m.AmountCents
		}
		rows.Close()
		audit(r.Context(), tx, p.ClinicID, p, "sale_void", "Canceló la venta #"+itoa(folio)+": "+req.Reason, map[string]any{"folio": folio, "paid_cents": refundTotal})
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	if refunds == nil {
		refunds = []methodTotal{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "refund_cents": refundTotal, "refund_by_method": refunds})
}
