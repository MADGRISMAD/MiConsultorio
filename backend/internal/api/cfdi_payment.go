package api

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

// CFDI payment complement (Complemento de Pago 2.0, CfdiType "P"). A sale settled in abonos is stamped as
// PPD with form 99; every abono then needs its own complement that points at the PPD invoice's UUID with
// the installment number, the balance before, the amount paid and the balance left.

// cfdiPaymentTax is the part of the invoice's tax that the abono pays (ImpuestosDR).
type cfdiPaymentTax struct {
	Rate        float64 // percent
	Base, Total float64
}

type cfdiPaymentDoc struct {
	Issuer, Receiver cfdiParty
	ExpeditionZip    string
	Currency         string
	Folio            string
	PaidAt           time.Time
	PaymentForm      string // c_FormaPago of the money received (never 99)
	RelatedUUID      string // the PPD invoice
	RelatedFolio     string
	Installment      int
	Previous, Paid   float64
	Balance          float64
	Taxes            []cfdiPaymentTax
}

type fmPayTax struct {
	Name        string
	Rate        float64
	Total       float64
	Base        float64
	IsRetention bool
}

type fmRelatedDoc struct {
	Uuid                  string
	Folio                 string
	PaymentMethod         string
	PartialityNumber      int
	PreviousBalanceAmount float64
	AmountPaid            float64
	ImpSaldoInsoluto      float64
	TaxObject             string
	Taxes                 []fmTax `json:",omitempty"`
}

type fmPayment struct {
	Date             string
	PaymentForm      string
	Amount           float64
	Currency         string
	RelatedDocuments []fmRelatedDoc
}

// paymentBody builds the Facturama request of a payment complement.
func (f *facturama) paymentBody(doc cfdiPaymentDoc) map[string]any {
	rel := fmRelatedDoc{Uuid: doc.RelatedUUID, Folio: doc.RelatedFolio, PaymentMethod: "PPD", PartialityNumber: doc.Installment,
		PreviousBalanceAmount: r2(doc.Previous), AmountPaid: r2(doc.Paid), ImpSaldoInsoluto: r2(doc.Balance), TaxObject: "01"}
	for _, t := range doc.Taxes {
		rel.TaxObject = "02"
		rel.Taxes = append(rel.Taxes, fmTax{Total: r2(t.Total), Name: "IVA", Base: r2(t.Base), Rate: r6(t.Rate / 100)})
	}
	return map[string]any{
		"Issuer":          map[string]string{"Rfc": doc.Issuer.RFC, "Name": doc.Issuer.Name, "FiscalRegime": doc.Issuer.Regime},
		"Receiver":        map[string]string{"Rfc": doc.Receiver.RFC, "Name": doc.Receiver.Name, "CfdiUse": "CP01", "FiscalRegime": doc.Receiver.Regime, "TaxZipCode": doc.Receiver.Zip},
		"CfdiType":        "P",
		"NameId":          "14",
		"ExpeditionPlace": doc.ExpeditionZip,
		"Folio":           doc.Folio,
		"Complement": map[string]any{"Payments": []fmPayment{{
			Date: doc.PaidAt.Format("2006-01-02T15:04:05"), PaymentForm: doc.PaymentForm, Amount: r2(doc.Paid), Currency: doc.Currency,
			RelatedDocuments: []fmRelatedDoc{rel},
		}}},
	}
}

func (f *facturama) StampPayment(ctx context.Context, doc cfdiPaymentDoc) (cfdiStamped, error) {
	return f.stampRaw(ctx, f.paymentBody(doc))
}

type cbPayRow struct {
	ID        string
	Method    string
	Amount    int
	CreatedAt time.Time
}

// cfdiInstallments numbers the abonos of a sale (NumParcialidad) in the order they were received and gives
// the balance before each one.
func cfdiInstallments(ctx context.Context, q rowsQuerier, saleID string, total int) (rows []cbPayRow, previous map[string]int, err error) {
	r, err := q.Query(ctx, `SELECT id, method, amount_cents, created_at FROM sale_payments WHERE sale_id = $1 ORDER BY created_at, id`, saleID)
	if err != nil {
		return nil, nil, err
	}
	defer r.Close()
	previous = map[string]int{}
	left := total
	for r.Next() {
		var x cbPayRow
		if err := r.Scan(&x.ID, &x.Method, &x.Amount, &x.CreatedAt); err != nil {
			return nil, nil, err
		}
		previous[x.ID] = left
		left -= x.Amount
		rows = append(rows, x)
	}
	return rows, previous, r.Err()
}

type cfdiPaymentView struct {
	PaymentID      string     `json:"payment_id"`
	Method         string     `json:"method"`
	AmountCents    int        `json:"amount_cents"`
	PaidAt         time.Time  `json:"paid_at"`
	Installment    int        `json:"installment"`
	PreviousCents  int        `json:"previous_cents"`
	BalanceCents   int        `json:"balance_cents"`
	CanIssue       bool       `json:"can_issue"`
	Problem        string     `json:"problem,omitempty"`
	ComplementID   *string    `json:"complement_id"`
	ComplementUUID string     `json:"complement_uuid"`
	State          string     `json:"state"`
	StampedAt      *time.Time `json:"stamped_at"`
}

// listPaymentComplements shows each abono of an invoiced sale on account and its complement, if any.
func (s *Server) listPaymentComplements(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Factura no encontrada.")
		return
	}
	p := principalFrom(r.Context())
	inv, err := loadInvoiceRow(r.Context(), s.db, p.ClinicID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Factura no encontrada.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	var total int
	var onCredit bool
	if err := s.db.QueryRow(r.Context(), `SELECT total_cents, on_credit FROM sales WHERE clinic_id = $1 AND id = $2`, p.ClinicID, inv.SaleID).Scan(&total, &onCredit); err != nil {
		serverError(w, r, err)
		return
	}
	out := []cfdiPaymentView{}
	if onCredit {
		pays, previous, err := cfdiInstallments(r.Context(), s.db, inv.SaleID, total)
		if err != nil {
			serverError(w, r, err)
			return
		}
		type comp struct {
			id, uuid, state string
			at              *time.Time
		}
		comps := map[string]comp{}
		rows, err := s.db.Query(r.Context(), `SELECT payment_id::text, id::text, fiscal_uuid, state, stamped_at FROM invoice_payment_complements WHERE clinic_id = $1 AND invoice_id = $2`, p.ClinicID, id)
		if err != nil {
			serverError(w, r, err)
			return
		}
		for rows.Next() {
			var pid string
			var c comp
			if err := rows.Scan(&pid, &c.id, &c.uuid, &c.state, &c.at); err != nil {
				rows.Close()
				serverError(w, r, err)
				return
			}
			comps[pid] = c
		}
		rows.Close()
		for i, x := range pays {
			v := cfdiPaymentView{PaymentID: x.ID, Method: x.Method, AmountCents: x.Amount, PaidAt: x.CreatedAt, Installment: i + 1,
				PreviousCents: previous[x.ID], BalanceCents: previous[x.ID] - x.Amount}
			if c, ok := comps[x.ID]; ok {
				cid := c.id
				v.ComplementID, v.ComplementUUID, v.State, v.StampedAt = &cid, c.uuid, c.state, c.at
			}
			switch {
			case v.ComplementID != nil:
			case inv.CfdiState != "stamped":
				v.Problem = "La factura no está timbrada."
			case payFormByMethod[x.Method] == "99" || payFormByMethod[x.Method] == "":
				v.Problem = "La forma de pago «otro» no es válida para el complemento; el SAT pide una forma de pago concreta."
			default:
				v.CanIssue = true
			}
			out = append(out, v)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"payments": out, "on_credit": onCredit, "stamped": inv.CfdiState == "stamped"})
}

// issuePaymentComplement stamps the complement of one abono.
func (s *Server) issuePaymentComplement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Factura no encontrada.")
		return
	}
	var req struct {
		PaymentID string `json:"payment_id"`
	}
	if !decode(w, r, &req) {
		return
	}
	if !validUUID(req.PaymentID) {
		writeError(w, http.StatusBadRequest, "Elige el abono.")
		return
	}
	prov := s.cfdiProvider()
	if prov == nil {
		writeJSON(w, http.StatusConflict, errorBody{Code: "NOT_CONFIGURED", Message: cfdiNotConfigured})
		return
	}
	p := principalFrom(r.Context())
	ctx := r.Context()
	inv, err := loadInvoiceRow(ctx, s.db, p.ClinicID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Factura no encontrada.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	if inv.CfdiState != "stamped" || inv.FiscalUUID == "" {
		writeError(w, http.StatusConflict, "La factura no está timbrada.")
		return
	}
	var folio, total int
	var onCredit bool
	var saleStatus string
	if err := s.db.QueryRow(ctx, `SELECT folio, total_cents, on_credit, status FROM sales WHERE clinic_id = $1 AND id = $2`, p.ClinicID, inv.SaleID).Scan(&folio, &total, &onCredit, &saleStatus); err != nil {
		serverError(w, r, err)
		return
	}
	if !onCredit || saleStatus == "void" {
		writeError(w, http.StatusConflict, "Solo las ventas a abonos facturadas como PPD llevan complemento de pago.")
		return
	}
	cfg, err := loadPosSettings(ctx, s.db, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if msg := validateFiscalData(cfg, inv); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	pays, previous, err := cfdiInstallments(ctx, s.db, inv.SaleID, total)
	if err != nil {
		serverError(w, r, err)
		return
	}
	var pay cbPayRow
	number := 0
	for i, x := range pays {
		if x.ID == req.PaymentID {
			pay, number = x, i+1
		}
	}
	if number == 0 {
		writeError(w, http.StatusNotFound, "El abono no pertenece a esta factura.")
		return
	}
	form := payFormByMethod[pay.Method]
	if form == "" || form == "99" {
		writeError(w, http.StatusBadRequest, "La forma de pago «otro» no es válida para el complemento; registra el abono con efectivo, transferencia o tarjeta.")
		return
	}

	// the part of each tax that this abono pays
	base, err := buildCFDI(ctx, s.db, p.ClinicID, inv, cfg)
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	frac := float64(pay.Amount) / float64(total)
	byRate := map[float64]*cfdiPaymentTax{}
	var rates []float64
	for _, it := range base.Items {
		if it.TaxRate <= 0 {
			continue
		}
		t := byRate[it.TaxRate]
		if t == nil {
			t = &cfdiPaymentTax{Rate: it.TaxRate}
			byRate[it.TaxRate] = t
			rates = append(rates, it.TaxRate)
		}
		t.Base += it.Base * frac
		t.Total += it.Tax * frac
	}
	doc := cfdiPaymentDoc{
		Issuer: base.Issuer, Receiver: base.Receiver, ExpeditionZip: base.ExpeditionZip, Currency: base.Currency,
		Folio: itoa(folio) + "-P" + itoa(number), PaidAt: pay.CreatedAt.In(cfdiLocation()), PaymentForm: form,
		RelatedUUID: inv.FiscalUUID, RelatedFolio: itoa(folio), Installment: number,
		Previous: float64(previous[pay.ID]) / 100, Paid: float64(pay.Amount) / 100, Balance: float64(previous[pay.ID]-pay.Amount) / 100,
	}
	for _, rate := range rates {
		doc.Taxes = append(doc.Taxes, *byRate[rate])
	}

	// Claim the abono: the unique index makes a second click (or a second user) fail instead of stamping twice.
	// A claim left in 'stamping' for over ten minutes (a crash) can be taken over.
	var compID string
	err = s.db.QueryRow(ctx, `
		INSERT INTO invoice_payment_complements (clinic_id, invoice_id, sale_id, payment_id, installment, previous_cents, paid_cents, balance_cents, related_uuid, email, created_by_name)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (payment_id) DO UPDATE SET created_at = now(), created_by_name = EXCLUDED.created_by_name
			WHERE invoice_payment_complements.state = 'stamping' AND invoice_payment_complements.created_at < now() - interval '10 minutes'
		RETURNING id`,
		p.ClinicID, id, inv.SaleID, pay.ID, number, previous[pay.ID], pay.Amount, previous[pay.ID]-pay.Amount, inv.FiscalUUID, inv.Email, p.actorName()).Scan(&compID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusConflict, "Este abono ya tiene complemento de pago (o se está timbrando).")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	res, err := prov.StampPayment(ctx, doc)
	if err != nil {
		_, _ = s.db.Exec(context.WithoutCancel(ctx), `DELETE FROM invoice_payment_complements WHERE id = $1 AND state = 'stamping'`, compID)
		var ce *cfdiError
		if errors.As(err, &ce) {
			writeJSON(w, http.StatusBadGateway, errorBody{Code: "STAMP_FAILED", Message: "No se pudo timbrar el complemento: " + ce.Msg})
			return
		}
		logf(r, "cfdi payment complement failed for invoice %s: %v", id, err)
		writeJSON(w, http.StatusBadGateway, errorBody{Code: "STAMP_FAILED", Message: "No se pudo contactar al servicio de timbrado. Inténtalo de nuevo."})
		return
	}
	if _, err := s.db.Exec(context.WithoutCancel(ctx), `
		UPDATE invoice_payment_complements SET state = 'stamped', fiscal_uuid = $3, cfdi_id = $4, stamped_at = now() WHERE clinic_id = $1 AND id = $2`,
		p.ClinicID, compID, res.UUID, res.ProviderID); err != nil {
		serverError(w, r, err)
		return
	}
	warnings := []string{}
	var att []mail.Attachment
	for _, ext := range []string{"xml", "pdf"} {
		data, err := prov.Fetch(ctx, res.ProviderID, ext)
		if err == nil {
			err = s.saveCFDIFile(cfdiRelPath(p.ClinicID, res.UUID, ext), data)
		}
		if err != nil {
			logf(r, "cfdi payment %s download failed for complement %s: %v", ext, compID, err)
			warnings = append(warnings, "El complemento se timbró, pero no se pudo guardar el "+strings.ToUpper(ext)+"; descárgalo de nuevo más tarde.")
			continue
		}
		col := map[string]string{"xml": "xml_path", "pdf": "pdf_path"}[ext]
		if _, err := s.db.Exec(context.WithoutCancel(ctx), `UPDATE invoice_payment_complements SET `+col+` = $3 WHERE clinic_id = $1 AND id = $2`, p.ClinicID, compID, cfdiRelPath(p.ClinicID, res.UUID, ext)); err != nil {
			serverError(w, r, err)
			return
		}
		ctype := map[string]string{"xml": "application/xml", "pdf": "application/pdf"}[ext]
		att = append(att, mail.Attachment{Name: res.UUID + "." + ext, ContentType: ctype, Data: data})
	}
	emailed := false
	if inv.Email != "" && s.mailEnabled() && len(att) > 0 {
		biz := cfg.BusinessName
		if biz == "" {
			biz = cfg.LegalName
		}
		s.sendMail(mail.Message{
			To:      []string{inv.Email},
			Subject: "Complemento de pago de " + biz + " (" + doc.Folio + ")",
			Text: "Adjuntamos el complemento de pago (CFDI) del abono " + itoa(number) + " de tu factura " + inv.FiscalUUID + ".\nFolio fiscal del complemento: " + res.UUID +
				"\n\n" + cfg.LegalName + " · RFC " + cfg.RFC + "\n",
			HTML: layout("Complemento de pago", `<p>Adjuntamos el complemento de pago (CFDI) del abono <strong>`+itoa(number)+`</strong> de tu factura <strong>`+esc(inv.FiscalUUID)+`</strong>.</p>`+
				`<p>Folio fiscal del complemento: <strong>`+esc(res.UUID)+`</strong></p>`+
				`<p style="font-size:12px;color:#7a8b9b">`+esc(cfg.LegalName)+` · RFC `+esc(cfg.RFC)+`</p>`),
			Attachments: att,
		})
		emailed = true
		_, _ = s.db.Exec(context.WithoutCancel(ctx), `UPDATE invoice_payment_complements SET emailed = true WHERE clinic_id = $1 AND id = $2`, p.ClinicID, compID)
	}
	audit(ctx, s.db, p.ClinicID, p, "cfdi_payment_complement", "Timbró el complemento de pago "+itoa(number)+" de la venta #"+itoa(folio)+" ("+res.UUID+")", nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "fiscal_uuid": res.UUID, "installment": number, "emailed": emailed, "warnings": warnings})
}

// downloadPaymentComplement serves the stored XML or PDF of a complement (fetching it again when missing).
func (s *Server) downloadPaymentComplement(ext string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, cid := chi.URLParam(r, "id"), chi.URLParam(r, "cid")
		if !validUUID(id) || !validUUID(cid) {
			writeError(w, http.StatusNotFound, "Complemento no encontrado.")
			return
		}
		p := principalFrom(r.Context())
		var uuid, providerID string
		err := s.db.QueryRow(r.Context(), `SELECT fiscal_uuid, cfdi_id FROM invoice_payment_complements WHERE clinic_id = $1 AND invoice_id = $2 AND id = $3 AND state = 'stamped'`,
			p.ClinicID, id, cid).Scan(&uuid, &providerID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !uuidRe.MatchString(strings.ToLower(uuid))) {
			writeError(w, http.StatusNotFound, "Complemento no encontrado.")
			return
		}
		if err != nil {
			serverError(w, r, err)
			return
		}
		rel := cfdiRelPath(p.ClinicID, uuid, ext)
		data, err := os.ReadFile(filepath.Join(s.cfg.UploadsDir, filepath.FromSlash(rel)))
		if err != nil {
			prov := s.cfdiProvider()
			if prov == nil {
				writeJSON(w, http.StatusConflict, errorBody{Code: "NOT_CONFIGURED", Message: cfdiNotConfigured})
				return
			}
			if data, err = prov.Fetch(r.Context(), providerID, ext); err != nil {
				writeError(w, http.StatusBadGateway, "No se pudo obtener el archivo del servicio de timbrado.")
				return
			}
			if err := s.saveCFDIFile(rel, data); err == nil {
				col := map[string]string{"xml": "xml_path", "pdf": "pdf_path"}[ext]
				_, _ = s.db.Exec(r.Context(), `UPDATE invoice_payment_complements SET `+col+` = $3 WHERE clinic_id = $1 AND id = $2`, p.ClinicID, cid, rel)
			}
		}
		w.Header().Set("Content-Type", map[string]string{"xml": "application/xml; charset=utf-8", "pdf": "application/pdf"}[ext])
		w.Header().Set("Content-Disposition", `attachment; filename="`+uuid+`.`+ext+`"`)
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(data)
		audit(r.Context(), s.db, p.ClinicID, p, "cfdi_download", "Descargó el "+strings.ToUpper(ext)+" del complemento de pago "+uuid, nil)
	}
}

// cfdiLocation is the clock the SAT reads on stamped documents (Mexico City); UTC when the zone is unavailable.
func cfdiLocation() *time.Location {
	if loc, err := time.LoadLocation("America/Mexico_City"); err == nil {
		return loc
	}
	return time.UTC
}
