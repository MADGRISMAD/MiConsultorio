package api

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

// Defaults when the catalog item has no SAT keys: medical services (85121800, E48) and, for products,
// the generic "no existe en el catálogo" key with unit pieza.
const (
	satServiceProduct, satServiceUnit = "85121800", "E48"
	satGenericProduct, satGenericUnit = "01010101", "H87"
)

var regimeRe = regexp.MustCompile(`^\s*(\d{3})`)

// regimeCode pulls the SAT key ("601") out of a text like "601 - General de Ley Personas Morales".
func regimeCode(s string) string {
	if m := regimeRe.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

var payFormByMethod = map[string]string{"cash": "01", "transfer": "03", "card": "04", "mp_point": "04", "mp_link": "04", "other": "99"}

const cfdiNotConfigured = "El timbrado no está configurado en el servidor (faltan las credenciales del proveedor de facturación)."

type invoiceRow struct {
	ID, SaleID, RFC, LegalName, TaxRegime, ZipCode, CfdiUse, Email, Status, FiscalUUID string
	CfdiState, CfdiID, XMLPath, PDFPath                                                string
}

func loadInvoiceRow(ctx context.Context, q queryRower, clinicID, id string) (invoiceRow, error) {
	var x invoiceRow
	err := q.QueryRow(ctx, `
		SELECT id, sale_id, rfc, legal_name, tax_regime, zip_code, cfdi_use, email, status, fiscal_uuid, cfdi_state, cfdi_id, cfdi_xml_path, cfdi_pdf_path
		FROM invoice_requests WHERE clinic_id = $1 AND id = $2`, clinicID, id).
		Scan(&x.ID, &x.SaleID, &x.RFC, &x.LegalName, &x.TaxRegime, &x.ZipCode, &x.CfdiUse, &x.Email, &x.Status, &x.FiscalUUID, &x.CfdiState, &x.CfdiID, &x.XMLPath, &x.PDFPath)
	return x, err
}

// buildCFDI turns a paid sale into the document sent to the PAC. Prices include tax, so each line is
// split into its net amount and the tax; discounts (line and ticket) are applied pro rata.
func buildCFDI(ctx context.Context, q interface {
	queryRower
	rowsQuerier
}, clinicID string, inv invoiceRow, cfg PosSettings) (cfdiDoc, error) {
	var doc cfdiDoc
	var folio, discount, total int
	var onCredit bool
	var status string
	if err := q.QueryRow(ctx, `SELECT folio, discount_cents, total_cents, on_credit, status FROM sales WHERE clinic_id = $1 AND id = $2`, clinicID, inv.SaleID).
		Scan(&folio, &discount, &total, &onCredit, &status); err != nil {
		return doc, err
	}
	if status != "paid" {
		return doc, fail(http.StatusConflict, "Solo se puede timbrar una venta pagada por completo.")
	}
	doc.Folio = itoa(folio)
	doc.Currency = cfg.Currency
	doc.ExpeditionZip = cfg.ZipCode
	doc.Issuer = cfdiParty{RFC: strings.ToUpper(cfg.RFC), Name: strings.ToUpper(cfg.LegalName), Regime: regimeCode(cfg.TaxRegime), Zip: cfg.ZipCode}
	doc.Receiver = cfdiParty{RFC: inv.RFC, Name: strings.ToUpper(inv.LegalName), Regime: regimeCode(inv.TaxRegime), Zip: inv.ZipCode}
	doc.CfdiUse = inv.CfdiUse

	type line struct {
		name, kind, unit, prodCode, unitCode string
		qty, rate                            float64
		unitPrice, lineDisc, total           int
	}
	rows, err := q.Query(ctx, `
		SELECT si.name, si.kind, coalesce(c.unit, ''), coalesce(c.sat_product_code, ''), coalesce(c.sat_unit_code, ''),
		       si.qty::float8, si.tax_rate::float8, si.unit_price_cents, si.discount_cents, si.total_cents
		FROM sale_items si LEFT JOIN catalog_items c ON c.id = si.item_id WHERE si.sale_id = $1 ORDER BY si.name, si.id`, inv.SaleID)
	if err != nil {
		return doc, err
	}
	var lines []line
	lineDiscSum, sub := 0, 0
	for rows.Next() {
		var l line
		if err := rows.Scan(&l.name, &l.kind, &l.unit, &l.prodCode, &l.unitCode, &l.qty, &l.rate, &l.unitPrice, &l.lineDisc, &l.total); err != nil {
			rows.Close()
			return doc, err
		}
		lines = append(lines, l)
		lineDiscSum += l.lineDisc
		sub += l.total
	}
	rows.Close()
	ticketDisc := discount - lineDiscSum
	for _, l := range lines {
		share := l.total
		if sub > 0 && ticketDisc > 0 {
			share = int(roundDiv(int64(l.total)*int64(sub-ticketDisc), int64(sub)))
		}
		grossBefore := l.total + l.lineDisc // unit price x qty, tax included
		disc := grossBefore - share
		f := 1 + l.rate/100
		subtotal := float64(grossBefore) / 100 / f
		discNet := float64(disc) / 100 / f
		it := cfdiItem{Qty: l.qty, Description: l.name, TaxRate: l.rate}
		it.Subtotal, it.Discount = r2(subtotal), r2(discNet)
		it.Base = r2(it.Subtotal - it.Discount)
		it.Tax = r2(it.Base * l.rate / 100)
		it.Total = r2(it.Base + it.Tax)
		it.UnitPrice = it.Subtotal / l.qty
		it.ProductCode, it.UnitCode = l.prodCode, l.unitCode
		if l.kind == "service" {
			if it.ProductCode == "" {
				it.ProductCode = satServiceProduct
			}
			if it.UnitCode == "" {
				it.UnitCode = satServiceUnit
			}
			it.Unit = "Servicio"
		} else {
			if it.ProductCode == "" {
				it.ProductCode = satGenericProduct
			}
			if it.UnitCode == "" {
				it.UnitCode = satGenericUnit
			}
			it.Unit = l.unit
			if it.Unit == "" {
				it.Unit = "Pieza"
			}
		}
		doc.Items = append(doc.Items, it)
	}

	// Payment: a sale settled in abonos is PPD with "por definir"; otherwise PUE with the form that paid most.
	if onCredit {
		doc.PaymentMethod, doc.PaymentForm = "PPD", "99"
		return doc, nil
	}
	doc.PaymentMethod = "PUE"
	prow, err := q.Query(ctx, `SELECT method, sum(amount_cents) FROM sale_payments WHERE sale_id = $1 GROUP BY method ORDER BY 2 DESC, 1`, inv.SaleID)
	if err != nil {
		return doc, err
	}
	defer prow.Close()
	doc.PaymentForm = "99"
	if prow.Next() {
		var m string
		var amt int
		if err := prow.Scan(&m, &amt); err != nil {
			return doc, err
		}
		if code, ok := payFormByMethod[m]; ok {
			doc.PaymentForm = code
		}
	}
	return doc, prow.Err()
}

func validateFiscalData(cfg PosSettings, inv invoiceRow) string {
	switch {
	case cfg.RFC == "" || cfg.LegalName == "" || regimeCode(cfg.TaxRegime) == "" || len(cfg.ZipCode) != 5:
		return "Completa los datos fiscales de tu negocio en Ajustes de cobros (RFC, razón social, régimen fiscal con su clave de 3 dígitos y código postal)."
	case regimeCode(inv.TaxRegime) == "":
		return "Falta el régimen fiscal del receptor con su clave de 3 dígitos (por ejemplo 612 - Personas Físicas con Actividades Empresariales)."
	}
	return ""
}

// cfdiFilePath is where a stamped file lives, relative to UPLOADS_DIR.
func cfdiRelPath(clinicID, fiscalUUID, ext string) string {
	return "cfdi/" + clinicID + "/" + strings.ToLower(fiscalUUID) + "." + ext
}

func (s *Server) saveCFDIFile(rel string, data []byte) error {
	if s.cfg.UploadsDir == "" || !strings.HasPrefix(rel, "cfdi/") || strings.Contains(rel, "..") {
		return errors.New("ruta inválida")
	}
	full := filepath.Join(s.cfg.UploadsDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
		return err
	}
	return os.WriteFile(full, data, 0o600)
}

// stampInvoice builds the CFDI of a pending invoice request, has the PAC stamp it and stores the result.
func (s *Server) stampInvoice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Solicitud no encontrada.")
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
		writeError(w, http.StatusNotFound, "Solicitud no encontrada.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	if inv.Status != "pending" || inv.CfdiState != "" {
		writeError(w, http.StatusConflict, "La solicitud ya fue atendida.")
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
	doc, err := buildCFDI(ctx, s.db, p.ClinicID, inv, cfg)
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	// Claim the request so two clicks cannot stamp the same sale twice.
	tag, err := s.db.Exec(ctx, `UPDATE invoice_requests SET cfdi_state = 'stamping', updated_at = now() WHERE clinic_id = $1 AND id = $2 AND status = 'pending' AND cfdi_state = ''`, p.ClinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusConflict, "La solicitud ya se está timbrando o fue atendida.")
		return
	}
	res, err := prov.Stamp(ctx, doc)
	if err != nil {
		_, _ = s.db.Exec(context.WithoutCancel(ctx), `UPDATE invoice_requests SET cfdi_state = '', updated_at = now() WHERE clinic_id = $1 AND id = $2`, p.ClinicID, id)
		var ce *cfdiError
		if errors.As(err, &ce) {
			writeJSON(w, http.StatusBadGateway, errorBody{Code: "STAMP_FAILED", Message: "No se pudo timbrar: " + ce.Msg})
			return
		}
		logf(r, "cfdi stamp failed for invoice %s: %v", id, err)
		writeJSON(w, http.StatusBadGateway, errorBody{Code: "STAMP_FAILED", Message: "No se pudo contactar al servicio de timbrado. Inténtalo de nuevo."})
		return
	}
	// From here the CFDI exists at the SAT: record it before anything else can fail.
	if _, err := s.db.Exec(context.WithoutCancel(ctx), `
		UPDATE invoice_requests SET status = 'issued', cfdi_state = 'stamped', fiscal_uuid = $3, cfdi_id = $4, stamped_at = now(), updated_at = now()
		WHERE clinic_id = $1 AND id = $2`, p.ClinicID, id, res.UUID, res.ProviderID); err != nil {
		serverError(w, r, err)
		return
	}
	inv.FiscalUUID, inv.CfdiID = res.UUID, res.ProviderID
	warnings := []string{}
	var xml, pdf []byte
	for _, ext := range []string{"xml", "pdf"} {
		data, err := prov.Fetch(ctx, res.ProviderID, ext)
		if err == nil {
			err = s.saveCFDIFile(cfdiRelPath(p.ClinicID, res.UUID, ext), data)
		}
		if err != nil {
			logf(r, "cfdi %s download failed for invoice %s: %v", ext, id, err)
			warnings = append(warnings, "El CFDI se timbró, pero no se pudo guardar el "+strings.ToUpper(ext)+"; descárgalo de nuevo más tarde.")
			continue
		}
		col := map[string]string{"xml": "cfdi_xml_path", "pdf": "cfdi_pdf_path"}[ext]
		if _, err := s.db.Exec(context.WithoutCancel(ctx), `UPDATE invoice_requests SET `+col+` = $3 WHERE clinic_id = $1 AND id = $2`, p.ClinicID, id, cfdiRelPath(p.ClinicID, res.UUID, ext)); err != nil {
			serverError(w, r, err)
			return
		}
		if ext == "xml" {
			xml = data
		} else {
			pdf = data
		}
	}
	emailed := false
	if inv.Email != "" && s.mailEnabled() && (xml != nil || pdf != nil) {
		var att []mail.Attachment
		if xml != nil {
			att = append(att, mail.Attachment{Name: res.UUID + ".xml", ContentType: "application/xml", Data: xml})
		}
		if pdf != nil {
			att = append(att, mail.Attachment{Name: res.UUID + ".pdf", ContentType: "application/pdf", Data: pdf})
		}
		biz := cfg.BusinessName
		if biz == "" {
			biz = cfg.LegalName
		}
		s.sendMail(mail.Message{
			To:      []string{inv.Email},
			Subject: "Tu factura de " + biz + " (" + doc.Folio + ")",
			Text:    "Adjuntamos tu factura (CFDI) con folio fiscal " + res.UUID + ".\n\n" + cfg.LegalName + " · RFC " + cfg.RFC + "\n",
			HTML: layout("Tu factura", `<p>Adjuntamos tu factura (CFDI) con folio fiscal <strong>`+esc(res.UUID)+`</strong>.</p>`+
				`<p style="font-size:12px;color:#7a8b9b">`+esc(cfg.LegalName)+` · RFC `+esc(cfg.RFC)+`</p>`),
			Attachments: att,
		})
		emailed = true
	}
	audit(ctx, s.db, p.ClinicID, p, "cfdi_stamp", "Timbró la factura de la venta #"+doc.Folio+" ("+res.UUID+")", nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "fiscal_uuid": res.UUID, "emailed": emailed, "warnings": warnings})
}

var cfdiMotives = map[string]string{"01": "comprobante emitido con errores con relación", "02": "comprobante emitido con errores sin relación", "03": "no se llevó a cabo la operación", "04": "operación nominativa relacionada en la factura global"}

// cancelCFDI cancels a stamped invoice with the PAC (SAT motives 01 to 04).
func (s *Server) cancelCFDI(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Factura no encontrada.")
		return
	}
	var req struct {
		Motive          string `json:"motive"`
		ReplacementUUID string `json:"replacement_uuid"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.ReplacementUUID = strings.ToUpper(strings.TrimSpace(req.ReplacementUUID))
	if _, ok := cfdiMotives[req.Motive]; !ok {
		writeError(w, http.StatusBadRequest, "El motivo de cancelación debe ser 01, 02, 03 o 04.")
		return
	}
	if req.Motive == "01" && !uuidRe.MatchString(strings.ToLower(req.ReplacementUUID)) {
		writeError(w, http.StatusBadRequest, "El motivo 01 requiere el folio fiscal (UUID) de la factura que sustituye.")
		return
	}
	if req.Motive != "01" {
		req.ReplacementUUID = ""
	}
	prov := s.cfdiProvider()
	if prov == nil {
		writeJSON(w, http.StatusConflict, errorBody{Code: "NOT_CONFIGURED", Message: cfdiNotConfigured})
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
	if inv.CfdiState != "stamped" {
		writeError(w, http.StatusConflict, "Esta factura no está timbrada o ya fue cancelada.")
		return
	}
	if err := prov.Cancel(r.Context(), inv.CfdiID, req.Motive, req.ReplacementUUID); err != nil {
		var ce *cfdiError
		if errors.As(err, &ce) {
			writeJSON(w, http.StatusBadGateway, errorBody{Code: "CANCEL_FAILED", Message: "No se pudo cancelar: " + ce.Msg})
			return
		}
		logf(r, "cfdi cancel failed for invoice %s: %v", id, err)
		writeJSON(w, http.StatusBadGateway, errorBody{Code: "CANCEL_FAILED", Message: "No se pudo contactar al servicio de timbrado. Inténtalo de nuevo."})
		return
	}
	if _, err := s.db.Exec(context.WithoutCancel(r.Context()), `
		UPDATE invoice_requests SET status = 'cancelled', cfdi_state = 'cancelled', cancel_motive = $3, cancel_replacement = $4, cfdi_cancelled_at = now(), updated_at = now()
		WHERE clinic_id = $1 AND id = $2`, p.ClinicID, id, req.Motive, req.ReplacementUUID); err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "cfdi_cancel", "Canceló el CFDI "+inv.FiscalUUID+" (motivo "+req.Motive+")", nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// downloadCFDI serves the stored XML or PDF, fetching it from the PAC again when it was not saved.
func (s *Server) downloadCFDI(ext string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		if inv.CfdiID == "" || (inv.CfdiState != "stamped" && inv.CfdiState != "cancelled") {
			writeError(w, http.StatusNotFound, "Esta factura no se timbró desde el sistema.")
			return
		}
		rel := cfdiRelPath(p.ClinicID, inv.FiscalUUID, ext)
		if !uuidRe.MatchString(strings.ToLower(inv.FiscalUUID)) {
			writeError(w, http.StatusNotFound, "Factura no encontrada.")
			return
		}
		full := filepath.Join(s.cfg.UploadsDir, filepath.FromSlash(rel))
		data, err := os.ReadFile(full)
		if err != nil {
			prov := s.cfdiProvider()
			if prov == nil {
				writeJSON(w, http.StatusConflict, errorBody{Code: "NOT_CONFIGURED", Message: cfdiNotConfigured})
				return
			}
			if data, err = prov.Fetch(r.Context(), inv.CfdiID, ext); err != nil {
				writeError(w, http.StatusBadGateway, "No se pudo obtener el archivo del servicio de timbrado.")
				return
			}
			if err := s.saveCFDIFile(rel, data); err == nil {
				col := map[string]string{"xml": "cfdi_xml_path", "pdf": "cfdi_pdf_path"}[ext]
				_, _ = s.db.Exec(r.Context(), `UPDATE invoice_requests SET `+col+` = $3 WHERE clinic_id = $1 AND id = $2`, p.ClinicID, id, rel)
			}
		}
		ctype := map[string]string{"xml": "application/xml; charset=utf-8", "pdf": "application/pdf"}[ext]
		w.Header().Set("Content-Type", ctype)
		w.Header().Set("Content-Disposition", `attachment; filename="`+inv.FiscalUUID+`.`+ext+`"`)
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(data)
		audit(r.Context(), s.db, p.ClinicID, p, "cfdi_download", "Descargó el "+strings.ToUpper(ext)+" de la factura "+inv.FiscalUUID, nil)
	}
}
