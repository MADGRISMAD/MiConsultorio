package api

import (
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type dayTotal struct {
	Day        string `json:"day"`
	Sales      int    `json:"sales"`
	TotalCents int    `json:"total_cents"`
}

type topItem struct {
	Name        string  `json:"name"`
	Kind        string  `json:"kind"`
	Qty         float64 `json:"qty"`
	TotalCents  int     `json:"total_cents"`
	MarginCents int     `json:"margin_cents"`
}

type userTotal struct {
	Name       string `json:"name"`
	Sales      int    `json:"sales"`
	TotalCents int    `json:"total_cents"`
}

type posReport struct {
	From, To       string        `json:"-"`
	Sales          int           `json:"sales"`
	VoidSales      int           `json:"void_sales"`
	TotalCents     int           `json:"total_cents"`
	TaxCents       int           `json:"tax_cents"`
	DiscountCents  int           `json:"discount_cents"`
	AvgTicketCents int           `json:"avg_ticket_cents"`
	CostCents      int           `json:"cost_cents"`
	MarginCents    int           `json:"margin_cents"`
	ByMethod       []methodTotal `json:"by_method"`
	ByDay          []dayTotal    `json:"by_day"`
	ByUser         []userTotal   `json:"by_user"`
	TopItems       []topItem     `json:"top_items"`
	Inventory      struct {
		Items       int `json:"items"`
		LowStock    int `json:"low_stock"`
		ValueCents  int `json:"value_cents"` // at cost
		RetailCents int `json:"retail_cents"`
	} `json:"inventory"`
}

func (s *Server) posReport(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	from, to, msg := dayRange(r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	ctx := r.Context()
	var rep posReport
	rep.ByMethod, rep.ByDay, rep.ByUser, rep.TopItems = []methodTotal{}, []dayTotal{}, []userTotal{}, []topItem{}

	if err := s.db.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE status='paid'), count(*) FILTER (WHERE status='void'),
		       coalesce(sum(total_cents) FILTER (WHERE status='paid'),0), coalesce(sum(tax_cents) FILTER (WHERE status='paid'),0),
		       coalesce(sum(discount_cents) FILTER (WHERE status='paid'),0)
		FROM sales WHERE clinic_id=$1 AND created_at >= $2 AND created_at < $3`, p.ClinicID, from, to).
		Scan(&rep.Sales, &rep.VoidSales, &rep.TotalCents, &rep.TaxCents, &rep.DiscountCents); err != nil {
		serverError(w, r, err)
		return
	}
	if rep.Sales > 0 {
		rep.AvgTicketCents = rep.TotalCents / rep.Sales
	}

	collect := func(sql string, scan func(pgx.Rows) error) error {
		rows, err := s.db.Query(ctx, sql, p.ClinicID, from, to)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			if err := scan(rows); err != nil {
				return err
			}
		}
		return rows.Err()
	}
	err := errors.Join(
		collect(`SELECT sp.method, coalesce(sum(sp.amount_cents),0), count(*) FROM sale_payments sp JOIN sales s ON s.id=sp.sale_id
			WHERE s.clinic_id=$1 AND sp.created_at>=$2 AND sp.created_at<$3 AND s.status<>'void' GROUP BY 1 ORDER BY 2 DESC`,
			func(rows pgx.Rows) error {
				var m methodTotal
				err := rows.Scan(&m.Method, &m.AmountCents, &m.Count)
				rep.ByMethod = append(rep.ByMethod, m)
				return err
			}),
		collect(`SELECT to_char(created_at, 'YYYY-MM-DD'), count(*), sum(total_cents) FROM sales
			WHERE clinic_id=$1 AND created_at>=$2 AND created_at<$3 AND status='paid' GROUP BY 1 ORDER BY 1`,
			func(rows pgx.Rows) error {
				var d dayTotal
				err := rows.Scan(&d.Day, &d.Sales, &d.TotalCents)
				rep.ByDay = append(rep.ByDay, d)
				return err
			}),
		collect(`SELECT created_by_name, count(*), sum(total_cents) FROM sales
			WHERE clinic_id=$1 AND created_at>=$2 AND created_at<$3 AND status='paid' GROUP BY 1 ORDER BY 3 DESC`,
			func(rows pgx.Rows) error {
				var u userTotal
				err := rows.Scan(&u.Name, &u.Sales, &u.TotalCents)
				rep.ByUser = append(rep.ByUser, u)
				return err
			}),
		collect(`SELECT si.name, si.kind, sum(si.qty)::float8, sum(si.total_cents),
				sum(si.total_cents - round(si.unit_cost_cents * si.qty))::bigint
			FROM sale_items si JOIN sales s ON s.id = si.sale_id
			WHERE s.clinic_id=$1 AND s.created_at>=$2 AND s.created_at<$3 AND s.status='paid'
			GROUP BY si.name, si.kind ORDER BY 4 DESC LIMIT 10`,
			func(rows pgx.Rows) error {
				var t topItem
				var margin int64
				err := rows.Scan(&t.Name, &t.Kind, &t.Qty, &t.TotalCents, &margin)
				t.MarginCents = int(margin)
				rep.TopItems = append(rep.TopItems, t)
				return err
			}),
	)
	if err != nil {
		serverError(w, r, err)
		return
	}
	var cost, margin int64
	if err := s.db.QueryRow(ctx, `
		SELECT coalesce(sum(round(si.unit_cost_cents * si.qty)),0)::bigint, coalesce(sum(si.total_cents - round(si.unit_cost_cents * si.qty)),0)::bigint
		FROM sale_items si JOIN sales s ON s.id = si.sale_id
		WHERE s.clinic_id=$1 AND s.created_at>=$2 AND s.created_at<$3 AND s.status='paid'`, p.ClinicID, from, to).Scan(&cost, &margin); err != nil {
		serverError(w, r, err)
		return
	}
	rep.CostCents, rep.MarginCents = int(cost), int(margin)

	var value, retail int64
	if err := s.db.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE track_stock AND active), count(*) FILTER (WHERE track_stock AND active AND stock <= min_stock),
		       coalesce(sum(round(stock * cost_cents)) FILTER (WHERE track_stock AND active AND stock > 0),0)::bigint,
		       coalesce(sum(round(stock * price_cents)) FILTER (WHERE track_stock AND active AND stock > 0),0)::bigint
		FROM catalog_items WHERE clinic_id=$1`, p.ClinicID).Scan(&rep.Inventory.Items, &rep.Inventory.LowStock, &value, &retail); err != nil {
		serverError(w, r, err)
		return
	}
	rep.Inventory.ValueCents, rep.Inventory.RetailCents = int(value), int(retail)
	writeJSON(w, http.StatusOK, map[string]any{
		"report": rep, "from": from.Format("2006-01-02"), "to": to.AddDate(0, 0, -1).Format("2006-01-02"),
	})
}

// exportSales streams the period's sales as CSV (opens in Excel).
func (s *Server) exportSales(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	from, to, msg := dayRange(r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	rows, err := s.db.Query(r.Context(), `
		SELECT s.folio, s.created_at, s.status, s.customer_name, s.created_by_name, s.subtotal_cents, s.discount_cents, s.tax_cents, s.total_cents,
		       coalesce((SELECT string_agg(method, '+' ORDER BY method) FROM sale_payments WHERE sale_id = s.id), ''), s.balance_cents
		FROM sales s WHERE s.clinic_id=$1 AND s.created_at>=$2 AND s.created_at<$3 ORDER BY s.folio`, p.ClinicID, from, to)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="ventas-`+from.Format("20060102")+`-`+to.AddDate(0, 0, -1).Format("20060102")+`.csv"`)
	_, _ = w.Write([]byte("\xEF\xBB\xBF")) // BOM so Excel reads UTF-8
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"Folio", "Fecha", "Estado", "Cliente", "Cajero", "Subtotal", "Descuento", "IVA incluido", "Total", "Métodos", "Saldo"})
	for rows.Next() {
		var folio, sub, disc, tax, total, balance int
		var at time.Time
		var status, cust, by, methods string
		if err := rows.Scan(&folio, &at, &status, &cust, &by, &sub, &disc, &tax, &total, &methods, &balance); err != nil {
			return
		}
		_ = cw.Write([]string{strconv.Itoa(folio), at.Local().Format("2006-01-02 15:04"), map[string]string{"paid": "Pagada", "open": "Abierta", "void": "Cancelada"}[status],
			csvSafe(cust), csvSafe(by), cents(sub), cents(disc), cents(tax), cents(total), methods, cents(balance)})
	}
	cw.Flush()
}

// csvSafe defuses spreadsheet formulas in user-supplied text.
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// ---------------------------------------------------------------------------
// Invoice requests (facturación)
// ---------------------------------------------------------------------------

type invoiceRequest struct {
	ID         string    `json:"id"`
	SaleID     string    `json:"sale_id"`
	Folio      int       `json:"folio"`
	TotalCents int       `json:"total_cents"`
	RFC        string    `json:"rfc"`
	LegalName  string    `json:"legal_name"`
	TaxRegime  string    `json:"tax_regime"`
	ZipCode    string    `json:"zip_code"`
	CfdiUse    string    `json:"cfdi_use"`
	Email      string    `json:"email"`
	Status     string    `json:"status"`
	FiscalUUID string    `json:"fiscal_uuid"`
	Note       string    `json:"note"`
	CreatedBy  string    `json:"created_by"`
	CreatedAt  time.Time `json:"created_at"`
	// CfdiState is '' for manual requests, then stamped or cancelled when issued through the PAC.
	CfdiState string `json:"cfdi_state"`
}

func (s *Server) listInvoices(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	where, args := "i.clinic_id = $1", []any{p.ClinicID}
	if st := r.URL.Query().Get("status"); st == "pending" || st == "issued" || st == "cancelled" {
		where += " AND i.status = $2"
		args = append(args, st)
	}
	rows, err := s.db.Query(r.Context(), `
		SELECT i.id, i.sale_id, s.folio, s.total_cents, i.rfc, i.legal_name, i.tax_regime, i.zip_code, i.cfdi_use, i.email, i.status, i.fiscal_uuid, i.note, i.created_by_name, i.created_at, i.cfdi_state
		FROM invoice_requests i JOIN sales s ON s.id = i.sale_id WHERE `+where+` ORDER BY i.created_at DESC LIMIT 300`, args...)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	list := []invoiceRequest{}
	for rows.Next() {
		var x invoiceRequest
		if err := rows.Scan(&x.ID, &x.SaleID, &x.Folio, &x.TotalCents, &x.RFC, &x.LegalName, &x.TaxRegime, &x.ZipCode, &x.CfdiUse, &x.Email, &x.Status, &x.FiscalUUID, &x.Note, &x.CreatedBy, &x.CreatedAt, &x.CfdiState); err != nil {
			serverError(w, r, err)
			return
		}
		list = append(list, x)
	}
	writeJSON(w, http.StatusOK, map[string]any{"invoices": list, "stamping": map[string]any{"enabled": s.cfdiProvider() != nil}})
}

var cfdiUses = []string{"G01", "G02", "G03", "I01", "I02", "I03", "I04", "I05", "I06", "I07", "I08", "D01", "D02", "D03", "D04", "D05", "D06", "D07", "D08", "D09", "D10", "S01", "CP01", "CN01"}

func (s *Server) createInvoice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SaleID    string `json:"sale_id"`
		RFC       string `json:"rfc"`
		LegalName string `json:"legal_name"`
		TaxRegime string `json:"tax_regime"`
		ZipCode   string `json:"zip_code"`
		CfdiUse   string `json:"cfdi_use"`
		Email     string `json:"email"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.RFC, req.LegalName = strings.ToUpper(strings.TrimSpace(req.RFC)), strings.TrimSpace(req.LegalName)
	req.TaxRegime, req.ZipCode, req.Email = strings.TrimSpace(req.TaxRegime), strings.TrimSpace(req.ZipCode), strings.TrimSpace(req.Email)
	if req.CfdiUse == "" {
		req.CfdiUse = "G03"
	}
	switch {
	case !validUUID(req.SaleID):
		writeError(w, http.StatusBadRequest, "Elige la venta a facturar.")
		return
	case !rfcRe.MatchString(req.RFC):
		writeError(w, http.StatusBadRequest, "El RFC no es válido.")
		return
	case req.LegalName == "" || utf8.RuneCountInString(req.LegalName) > 200:
		writeError(w, http.StatusBadRequest, "Escribe la razón social tal como aparece en la constancia fiscal.")
		return
	case len(req.ZipCode) != 5 || strings.Trim(req.ZipCode, "0123456789") != "":
		writeError(w, http.StatusBadRequest, "El código postal fiscal debe tener 5 dígitos.")
		return
	case !hasPermission(cfdiUses, req.CfdiUse):
		writeError(w, http.StatusBadRequest, "El uso del CFDI no es válido.")
		return
	case utf8.RuneCountInString(req.TaxRegime) > 80 || utf8.RuneCountInString(req.Email) > 160:
		writeError(w, http.StatusBadRequest, "Uno de los campos es demasiado largo.")
		return
	case req.Email != "" && !strings.Contains(req.Email, "@"):
		writeError(w, http.StatusBadRequest, "El correo no es válido.")
		return
	}
	p := principalFrom(r.Context())
	var id string
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		var status string
		var folio int
		err := tx.QueryRow(r.Context(), `SELECT status, folio FROM sales WHERE clinic_id=$1 AND id=$2`, p.ClinicID, req.SaleID).Scan(&status, &folio)
		if errors.Is(err, pgx.ErrNoRows) {
			return fail(http.StatusNotFound, "Venta no encontrada.")
		}
		if err != nil {
			return err
		}
		if status == "open" {
			return fail(http.StatusConflict, "La venta tiene saldo pendiente; factúrala cuando esté pagada por completo.")
		}
		if status != "paid" {
			return fail(http.StatusConflict, "No se puede facturar una venta cancelada.")
		}
		err = tx.QueryRow(r.Context(), `
			INSERT INTO invoice_requests (clinic_id, sale_id, rfc, legal_name, tax_regime, zip_code, cfdi_use, email, created_by_name)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
			p.ClinicID, req.SaleID, req.RFC, req.LegalName, req.TaxRegime, req.ZipCode, req.CfdiUse, req.Email, p.actorName()).Scan(&id)
		if isUniqueViolation(err) {
			return fail(http.StatusConflict, "Esa venta ya tiene una solicitud de factura.")
		}
		if err != nil {
			return err
		}
		audit(r.Context(), tx, p.ClinicID, p, "invoice_request", "Solicitó factura de la venta #"+itoa(folio)+" para "+req.RFC, nil)
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

// updateInvoice marks a request as issued (with the fiscal folio from the PAC or accountant) or cancelled.
func (s *Server) updateInvoice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Solicitud no encontrada.")
		return
	}
	var req struct {
		Status     string `json:"status"`
		FiscalUUID string `json:"fiscal_uuid"`
		Note       string `json:"note"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.FiscalUUID, req.Note = strings.TrimSpace(req.FiscalUUID), strings.TrimSpace(req.Note)
	if req.Status != "issued" && req.Status != "cancelled" {
		writeError(w, http.StatusBadRequest, "Estado inválido.")
		return
	}
	if req.Status == "issued" && !uuidRe.MatchString(strings.ToLower(req.FiscalUUID)) {
		writeError(w, http.StatusBadRequest, "Escribe el folio fiscal (UUID) de la factura.")
		return
	}
	if utf8.RuneCountInString(req.Note) > 300 {
		writeError(w, http.StatusBadRequest, "La nota es demasiado larga.")
		return
	}
	p := principalFrom(r.Context())
	tag, err := s.db.Exec(r.Context(), `
		UPDATE invoice_requests SET status=$3, fiscal_uuid=$4, note=$5, updated_at=now()
		WHERE clinic_id=$1 AND id=$2 AND status='pending' AND cfdi_state=''`, p.ClinicID, id, req.Status, strings.ToUpper(req.FiscalUUID), req.Note)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusConflict, "La solicitud no existe o ya fue atendida.")
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "invoice_update", "Marcó una solicitud de factura como "+map[string]string{"issued": "emitida", "cancelled": "cancelada"}[req.Status], nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
