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

// Pre-account of a consultation. The professional lists the services and supplies used, marks the supplies
// already spent (their stock leaves right then, once) and sends the account to the register, which charges
// it as a normal sale (see consult_billing_sale.go for how the sale and the pre-account meet).
//
// Plans without cobros keep a plain list of concepts (no catalog, no prices, no stock): useful as a note
// of what was done, but it cannot be sent to the register.

type cbItemIn struct {
	ID             string  `json:"id"` // an existing line (a consumed one is kept as it is)
	CatalogItemID  string  `json:"catalog_item_id"`
	Name           string  `json:"name"`
	Kind           string  `json:"kind"`
	Qty            float64 `json:"qty"`
	UnitPriceCents *int    `json:"unit_price_cents"` // free concepts only; catalog lines take the catalog price
	DiscountCents  int     `json:"discount_cents"`   // administrators only
	Note           string  `json:"note"`
	Consumed       bool    `json:"consumed"`  // the supply was already used up: take it out of stock now
	NoCharge       bool    `json:"no_charge"` // a supply that is used but not billed to the patient
}

type cbChargeIn struct {
	PatientID      string     `json:"patient_id"`
	EncounterID    string     `json:"encounter_id"`
	AppointmentID  string     `json:"appointment_id"`
	ProfessionalID string     `json:"professional_id"`
	Note           string     `json:"note"`
	Items          []cbItemIn `json:"items"`
	Send           bool       `json:"send"`
}

type cbItem struct {
	ID             string     `json:"id"`
	CatalogItemID  *string    `json:"catalog_item_id"`
	Kind           string     `json:"kind"`
	Name           string     `json:"name"`
	Qty            float64    `json:"qty"`
	UnitPriceCents int        `json:"unit_price_cents"`
	TaxRate        float64    `json:"tax_rate"`
	DiscountCents  int        `json:"discount_cents"`
	TotalCents     int        `json:"total_cents"`
	Note           string     `json:"note"`
	Consumed       bool       `json:"consumed"`
	ConsumedAt     *time.Time `json:"consumed_at"`
}

type cbCharge struct {
	ID               string     `json:"id"`
	EncounterID      *string    `json:"encounter_id"`
	PatientID        string     `json:"patient_id"`
	PatientName      string     `json:"patient_name"`
	AppointmentID    *string    `json:"appointment_id"`
	ProfessionalID   *string    `json:"professional_id"`
	ProfessionalName string     `json:"professional_name"`
	Status           string     `json:"status"`
	Note             string     `json:"note"`
	SaleID           *string    `json:"sale_id"`
	CreatedBy        string     `json:"created_by"`
	CancelReason     string     `json:"cancel_reason,omitempty"`
	SentAt           *time.Time `json:"sent_at"`
	ChargedAt        *time.Time `json:"charged_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	TotalCents       int        `json:"total_cents"`
	ItemCount        int        `json:"item_count"`
	Items            []cbItem   `json:"items,omitempty"`
	Warnings         []string   `json:"warnings,omitempty"`
}

const cbChargeCols = `c.id, c.encounter_id::text, c.patient_id, trim(pt.names || ' ' || pt.last_names), c.appointment_id::text, c.professional_id::text,
	coalesce(pu.name, ''), c.status, c.note, c.sale_id::text, c.created_by_name, c.cancel_reason, c.sent_at, c.charged_at, c.created_at, c.updated_at,
	coalesce((SELECT sum(round(i.qty * i.unit_price_cents)::int - i.discount_cents) FROM encounter_charge_items i WHERE i.charge_id = c.id), 0)::int,
	(SELECT count(*) FROM encounter_charge_items i WHERE i.charge_id = c.id)::int`

const cbChargeFrom = ` FROM encounter_charges c JOIN patients pt ON pt.id = c.patient_id LEFT JOIN users pu ON pu.id = c.professional_id`

func cbScanCharge(row pgx.Row) (cbCharge, error) {
	var c cbCharge
	err := row.Scan(&c.ID, &c.EncounterID, &c.PatientID, &c.PatientName, &c.AppointmentID, &c.ProfessionalID, &c.ProfessionalName, &c.Status, &c.Note,
		&c.SaleID, &c.CreatedBy, &c.CancelReason, &c.SentAt, &c.ChargedAt, &c.CreatedAt, &c.UpdatedAt, &c.TotalCents, &c.ItemCount)
	return c, err
}

func cbLoadItems(ctx context.Context, q rowsQuerier, chargeID string) ([]cbItem, error) {
	rows, err := q.Query(ctx, `
		SELECT id, catalog_item_id::text, kind, name, qty::float8, unit_price_cents, tax_rate::float8, discount_cents,
		       round(qty * unit_price_cents)::int - discount_cents, note, consumed, consumed_at
		FROM encounter_charge_items WHERE charge_id = $1 ORDER BY position, id`, chargeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []cbItem{}
	for rows.Next() {
		var it cbItem
		if err := rows.Scan(&it.ID, &it.CatalogItemID, &it.Kind, &it.Name, &it.Qty, &it.UnitPriceCents, &it.TaxRate, &it.DiscountCents, &it.TotalCents, &it.Note, &it.Consumed, &it.ConsumedAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// cbHasCobros reports whether the clinic's plan includes cobros (prices, catalog and stock).
func cbHasCobros(p *Principal) bool {
	if p == nil || p.Billing == nil {
		return false
	}
	plan, ok := planByID(p.Billing.Plan)
	return ok && plan.Cobros
}

// cbCanTouch: the author, the professional it is for, or an administrator.
func cbCanTouch(p *Principal, createdBy, professionalID *string) bool {
	if p.Role == RoleAdmin {
		return true
	}
	return (createdBy != nil && *createdBy == p.UserID) || (professionalID != nil && *professionalID == p.UserID)
}

// cbVisible is the SQL condition (over alias c) for the charges the caller may see.
func cbVisible(p *Principal, args *[]any) string {
	var parts []string
	if hasAnyPermission(p.Permissions, PermAdminHistorials) {
		if p.Role == RoleAdmin {
			parts = append(parts, "true")
		} else {
			*args = append(*args, p.UserID)
			n := itoa(len(*args))
			parts = append(parts, "(c.created_by = $"+n+" OR c.professional_id = $"+n+")")
		}
	}
	if hasAnyPermission(p.Permissions, PermPOS) && cbHasCobros(p) {
		parts = append(parts, "c.status IN ('sent', 'charged')")
	}
	if len(parts) == 0 {
		return "false"
	}
	return "(" + strings.Join(parts, " OR ") + ")"
}

func (s *Server) cbGetOne(ctx context.Context, p *Principal, id string) (cbCharge, error) {
	args := []any{p.ClinicID, id}
	vis := cbVisible(p, &args)
	c, err := cbScanCharge(s.db.QueryRow(ctx, `SELECT `+cbChargeCols+cbChargeFrom+` WHERE c.clinic_id = $1 AND c.id = $2 AND `+vis, args...))
	if err != nil {
		return c, err
	}
	c.Items, err = cbLoadItems(ctx, s.db, c.ID)
	return c, err
}

// ---------------------------------------------------------------------------
// Reads
// ---------------------------------------------------------------------------

func (s *Server) cbList(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	q := r.URL.Query()
	args := []any{p.ClinicID}
	where := "c.clinic_id = $1 AND " + cbVisible(p, &args)
	add := func(cond string, v any) {
		args = append(args, v)
		where += " AND " + strings.ReplaceAll(cond, "?", "$"+itoa(len(args)))
	}
	switch st := q.Get("status"); st {
	case "":
	case "draft", "sent", "charged", "cancelled":
		add("c.status = ?", st)
	default:
		writeError(w, http.StatusBadRequest, "El estado no es válido.")
		return
	}
	for key, col := range map[string]string{"encounter_id": "c.encounter_id", "patient_id": "c.patient_id", "appointment_id": "c.appointment_id"} {
		if v := q.Get(key); v != "" {
			if !validUUID(v) {
				writeError(w, http.StatusBadRequest, "Un filtro no es válido.")
				return
			}
			add(col+" = ?", v)
		}
	}
	order := "c.updated_at DESC"
	if q.Get("status") == "sent" {
		order = "c.sent_at ASC"
	}
	rows, err := s.db.Query(r.Context(), `SELECT `+cbChargeCols+cbChargeFrom+` WHERE `+where+` ORDER BY `+order+` LIMIT 100`, args...)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	list := []cbCharge{}
	for rows.Next() {
		c, err := cbScanCharge(rows)
		if err != nil {
			serverError(w, r, err)
			return
		}
		list = append(list, c)
	}
	if rows.Err() != nil {
		serverError(w, r, rows.Err())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"charges": list})
}

func (s *Server) cbGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Pre-cuenta no encontrada.")
		return
	}
	p := principalFrom(r.Context())
	c, err := s.cbGetOne(r.Context(), p, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Pre-cuenta no encontrada.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	if hasAnyPermission(p.Permissions, PermAdminHistorials) {
		s.logAccess(r.Context(), p.ClinicID, c.PatientID, p, "Consultó una pre-cuenta de consulta")
	}
	writeJSON(w, http.StatusOK, map[string]any{"charge": c})
}

// cbCatalog is what a professional may pick from: active catalog items with their stock and expiry.
func (s *Server) cbCatalog(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	args := []any{p.ClinicID, today()}
	where := "c.clinic_id = $1 AND c.active"
	if k := r.URL.Query().Get("kind"); k == "service" || k == "product" {
		args = append(args, k)
		where += " AND c.kind = $" + itoa(len(args))
	}
	if text := strings.TrimSpace(r.URL.Query().Get("q")); text != "" {
		args = append(args, "%"+escapeLike(text)+"%")
		where += " AND (c.name ILIKE $" + itoa(len(args)) + " OR c.sku ILIKE $" + itoa(len(args)) + ")"
	}
	rows, err := s.db.Query(r.Context(), `
		SELECT c.id, c.kind, c.name, c.unit, c.price_cents, c.tax_rate::float8, c.track_stock, c.stock::float8, c.min_stock::float8,
		       (SELECT to_char(min(l.expires_on), 'YYYY-MM-DD') FROM stock_lots l WHERE l.item_id = c.id AND l.qty > 0),
		       coalesce((SELECT sum(l.qty) FROM stock_lots l WHERE l.item_id = c.id AND l.qty > 0 AND l.expires_on < $2::date), 0)::float8
		FROM catalog_items c WHERE `+where+` ORDER BY c.kind DESC, lower(c.name) LIMIT 60`, args...)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	type item struct {
		ID           string  `json:"id"`
		Kind         string  `json:"kind"`
		Name         string  `json:"name"`
		Unit         string  `json:"unit"`
		PriceCents   int     `json:"price_cents"`
		TaxRate      float64 `json:"tax_rate"`
		TrackStock   bool    `json:"track_stock"`
		Stock        float64 `json:"stock"`
		MinStock     float64 `json:"min_stock"`
		NextExpiry   *string `json:"next_expiry"`
		ExpiredQty   float64 `json:"expired_qty"`
		UsableStock  float64 `json:"usable_stock"` // stock outside expired lots
		StockWarning string  `json:"stock_warning,omitempty"`
	}
	list := []item{}
	for rows.Next() {
		var x item
		if err := rows.Scan(&x.ID, &x.Kind, &x.Name, &x.Unit, &x.PriceCents, &x.TaxRate, &x.TrackStock, &x.Stock, &x.MinStock, &x.NextExpiry, &x.ExpiredQty); err != nil {
			serverError(w, r, err)
			return
		}
		if x.TrackStock {
			x.UsableStock = x.Stock - x.ExpiredQty
			switch {
			case x.UsableStock <= qtyEps && x.ExpiredQty > qtyEps:
				x.StockWarning = "Solo hay existencias caducadas."
			case x.UsableStock <= qtyEps:
				x.StockWarning = "Agotado."
			case x.ExpiredQty > qtyEps:
				x.StockWarning = "Hay lotes caducados; se usarán primero los vigentes."
			case x.Stock <= x.MinStock:
				x.StockWarning = "Existencias por debajo del mínimo."
			}
		}
		list = append(list, x)
	}
	if rows.Err() != nil {
		serverError(w, r, rows.Err())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

// ---------------------------------------------------------------------------
// Writes
// ---------------------------------------------------------------------------

func (in *cbChargeIn) clean() string {
	in.Note = strings.TrimSpace(in.Note)
	for _, f := range []struct{ v, msg string }{
		{in.EncounterID, "La consulta no es válida."}, {in.AppointmentID, "La cita no es válida."}, {in.ProfessionalID, "El profesional no es válido."},
	} {
		if f.v != "" && !validUUID(f.v) {
			return f.msg
		}
	}
	if utf8.RuneCountInString(in.Note) > 300 {
		return "La nota es demasiado larga."
	}
	if len(in.Items) == 0 || len(in.Items) > 60 {
		return "Agrega entre 1 y 60 conceptos."
	}
	for i := range in.Items {
		it := &in.Items[i]
		n := "Concepto " + itoa(i+1) + ": "
		it.Name, it.Note = strings.TrimSpace(it.Name), strings.TrimSpace(it.Note)
		switch {
		case it.ID != "" && !validUUID(it.ID):
			return n + "la línea no es válida."
		case it.CatalogItemID != "" && !validUUID(it.CatalogItemID):
			return n + "el artículo no es válido."
		case it.CatalogItemID == "" && (it.Name == "" || utf8.RuneCountInString(it.Name) > 160):
			return n + "escribe un nombre."
		case it.Qty <= 0 || it.Qty > 100_000:
			return n + "la cantidad no es válida."
		case utf8.RuneCountInString(it.Note) > 200:
			return n + "la nota es demasiado larga."
		case it.DiscountCents < 0 || !okCents(it.DiscountCents):
			return n + "el descuento no es válido."
		case it.UnitPriceCents != nil && !okCents(*it.UnitPriceCents):
			return n + "el precio no es válido."
		case it.Kind != "" && it.Kind != "service" && it.Kind != "product":
			return n + "el tipo no es válido."
		}
	}
	return ""
}

// cbSave creates (chargeID == "") or replaces the lines of a pre-account inside tx.
func (s *Server) cbSave(ctx context.Context, tx pgx.Tx, p *Principal, chargeID string, in cbChargeIn) (string, []string, error) {
	cobros := cbHasCobros(p)
	var warnings []string
	var encounterID, createdBy, proID *string
	var patientID string
	if chargeID == "" {
		patientID = in.PatientID
		if !validUUID(patientID) {
			return "", nil, fail(http.StatusBadRequest, "Indica el paciente.")
		}
		if err := tx.QueryRow(ctx, `SELECT id::text FROM patients WHERE clinic_id = $1 AND id = $2`, p.ClinicID, patientID).Scan(&patientID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return "", nil, fail(http.StatusBadRequest, "El paciente no existe en este consultorio.")
			}
			return "", nil, err
		}
		pro := in.ProfessionalID
		if pro == "" && (p.Role == RoleDoctor || p.Role == RoleAdmin) {
			pro = p.UserID
		}
		if pro != "" {
			ok, err := validProfessional(ctx, tx, p.ClinicID, pro)
			if err != nil {
				return "", nil, err
			}
			if !ok {
				return "", nil, fail(http.StatusBadRequest, "El profesional no existe en este consultorio.")
			}
			proID = &pro
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO encounter_charges (clinic_id, patient_id, professional_id, note, created_by, created_by_name)
			VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`, p.ClinicID, patientID, proID, in.Note, p.UserID, p.actorName()).Scan(&chargeID); err != nil {
			return "", nil, err
		}
	} else {
		var status string
		err := tx.QueryRow(ctx, `SELECT status, patient_id::text, encounter_id::text, created_by::text, professional_id::text FROM encounter_charges WHERE clinic_id = $1 AND id = $2 FOR UPDATE`,
			p.ClinicID, chargeID).Scan(&status, &patientID, &encounterID, &createdBy, &proID)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil, fail(http.StatusNotFound, "Pre-cuenta no encontrada.")
		}
		if err != nil {
			return "", nil, err
		}
		if !cbCanTouch(p, createdBy, proID) {
			return "", nil, fail(http.StatusForbidden, "Esta pre-cuenta es de otro profesional.")
		}
		if status != "draft" && status != "sent" {
			return "", nil, &httpError{Status: http.StatusConflict, Code: "CHARGE_CLOSED", Msg: "La pre-cuenta ya fue cobrada o cancelada."}
		}
		if in.PatientID != "" && in.PatientID != patientID {
			return "", nil, fail(http.StatusBadRequest, "No se puede cambiar el paciente de una pre-cuenta.")
		}
		if _, err := tx.Exec(ctx, `UPDATE encounter_charges SET note = $3, updated_at = now() WHERE clinic_id = $1 AND id = $2`, p.ClinicID, chargeID, in.Note); err != nil {
			return "", nil, err
		}
	}

	// link to the encounter (once) and the visit
	if in.EncounterID != "" && (encounterID == nil || *encounterID != in.EncounterID) {
		if encounterID != nil {
			return "", nil, fail(http.StatusConflict, "La pre-cuenta ya está ligada a otra consulta.")
		}
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM encounters WHERE clinic_id = $1 AND id = $2 AND patient_id = $3)`, p.ClinicID, in.EncounterID, patientID).Scan(&ok); err != nil {
			return "", nil, err
		}
		if !ok {
			return "", nil, fail(http.StatusBadRequest, "La consulta no existe para este paciente.")
		}
		if _, err := tx.Exec(ctx, `UPDATE encounter_charges SET encounter_id = $3 WHERE clinic_id = $1 AND id = $2`, p.ClinicID, chargeID, in.EncounterID); err != nil {
			return "", nil, err
		}
		// supplies consumed before the note existed now belong to it
		if _, err := tx.Exec(ctx, `
			UPDATE stock_movements SET encounter_id = $3 WHERE clinic_id = $1 AND encounter_id IS NULL
			  AND charge_item_id IN (SELECT id FROM encounter_charge_items WHERE charge_id = $2)`, p.ClinicID, chargeID, in.EncounterID); err != nil {
			return "", nil, err
		}
		encounterID = &in.EncounterID
	}
	if in.AppointmentID != "" {
		tag, err := tx.Exec(ctx, `UPDATE encounter_charges SET appointment_id = $3 WHERE clinic_id = $1 AND id = $2
			AND EXISTS (SELECT 1 FROM appointments a WHERE a.clinic_id = $1 AND a.id = $3)`, p.ClinicID, chargeID, in.AppointmentID)
		if err != nil {
			return "", nil, err
		}
		if tag.RowsAffected() == 0 {
			return "", nil, fail(http.StatusBadRequest, "La cita no existe en este consultorio.")
		}
	}

	// existing lines: consumed ones are permanent
	type old struct {
		itemID *string
		name   string
	}
	kept := map[string]old{}
	rows, err := tx.Query(ctx, `SELECT id::text, catalog_item_id::text, name FROM encounter_charge_items WHERE charge_id = $1 AND consumed`, chargeID)
	if err != nil {
		return "", nil, err
	}
	for rows.Next() {
		var id string
		var o old
		if err := rows.Scan(&id, &o.itemID, &o.name); err != nil {
			rows.Close()
			return "", nil, err
		}
		kept[id] = o
	}
	rows.Close()
	if rows.Err() != nil {
		return "", nil, rows.Err()
	}
	present := map[string]bool{}
	for _, it := range in.Items {
		if it.ID != "" && kept[it.ID].name != "" {
			present[it.ID] = true
		}
	}
	for id, o := range kept {
		if !present[id] {
			return "", nil, &httpError{Status: http.StatusConflict, Code: "CONSUMED_LOCKED", Msg: "«" + o.name + "» ya se descontó del inventario y no se puede quitar de la pre-cuenta."}
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM encounter_charge_items WHERE charge_id = $1 AND NOT consumed`, chargeID); err != nil {
		return "", nil, err
	}

	var cfg PosSettings
	if cobros {
		var err error
		if cfg, err = loadPosSettings(ctx, tx, p.ClinicID); err != nil {
			return "", nil, err
		}
	}
	for i, it := range in.Items {
		if present[it.ID] {
			if _, err := tx.Exec(ctx, `UPDATE encounter_charge_items SET position = $2, note = $3 WHERE id = $1`, it.ID, i, it.Note); err != nil {
				return "", nil, err
			}
			continue
		}
		kind, name, price, tax := it.Kind, it.Name, 0, 0.0
		var catalogID any
		var track bool
		if it.CatalogItemID != "" {
			if !cobros {
				return "", nil, &httpError{Status: http.StatusForbidden, Code: "PLAN_REQUIRED", Msg: "El catálogo con precios e insumos viene con los planes Crecimiento y Pro."}
			}
			err := tx.QueryRow(ctx, `SELECT kind, name, price_cents, tax_rate::float8, track_stock FROM catalog_items WHERE clinic_id = $1 AND id = $2 AND active`,
				p.ClinicID, it.CatalogItemID).Scan(&kind, &name, &price, &tax, &track)
			if errors.Is(err, pgx.ErrNoRows) {
				return "", nil, fail(http.StatusBadRequest, "Concepto "+itoa(i+1)+": el artículo no existe en tu catálogo.")
			}
			if err != nil {
				return "", nil, err
			}
			catalogID = it.CatalogItemID
		} else {
			if kind == "" {
				kind = "service"
			}
			if cobros {
				tax = cfg.DefaultTaxRate
				if it.UnitPriceCents != nil {
					price = *it.UnitPriceCents
				}
			}
		}
		discount := 0
		if p.Role == RoleAdmin && cobros {
			discount = it.DiscountCents
		}
		if it.NoCharge {
			price, discount = 0, 0
		}
		if gross := int(roundDiv(int64(price)*int64(it.Qty*1000), 1000)); discount > gross {
			return "", nil, fail(http.StatusBadRequest, "Concepto "+itoa(i+1)+": el descuento supera el importe.")
		}
		if it.Consumed {
			switch {
			case !cobros:
				return "", nil, &httpError{Status: http.StatusForbidden, Code: "PLAN_REQUIRED", Msg: "Descontar insumos del inventario viene con los planes Crecimiento y Pro."}
			case it.CatalogItemID == "" || kind != "product":
				return "", nil, fail(http.StatusBadRequest, "Concepto "+itoa(i+1)+": solo los productos del catálogo se descuentan del inventario.")
			case !track:
				return "", nil, fail(http.StatusConflict, "«"+name+"» no controla existencias.")
			}
		}
		var lineID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO encounter_charge_items (charge_id, clinic_id, catalog_item_id, kind, name, qty, unit_price_cents, tax_rate, discount_cents, note, position)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`,
			chargeID, p.ClinicID, catalogID, kind, name, it.Qty, price, tax, discount, it.Note, i).Scan(&lineID); err != nil {
			return "", nil, err
		}
		if !it.Consumed {
			continue
		}
		var low, minStock float64
		encID := ""
		if encounterID != nil {
			encID = *encounterID
		}
		res, err := consumeStock(ctx, tx, stockOp{clinicID: p.ClinicID, itemID: it.CatalogItemID, reason: "consumption", encounterID: encID, chargeItemID: lineID,
			note: "Insumo de la pre-cuenta de la consulta", actor: p.actorName()}, it.Qty, false, cfg.AllowNegativeStock)
		if err != nil {
			return "", nil, err
		}
		if res.Short > qtyEps {
			if res.Expired > 0 {
				return "", nil, &httpError{Status: http.StatusConflict, Code: "LOT_EXPIRED", Msg: "Las existencias de «" + name + "» están caducadas; no se pueden usar."}
			}
			return "", nil, &httpError{Status: http.StatusConflict, Code: "NO_STOCK", Msg: "No hay existencias suficientes de «" + name + "»."}
		}
		if _, err := tx.Exec(ctx, `UPDATE encounter_charge_items SET consumed = true, consumed_at = now() WHERE id = $1`, lineID); err != nil {
			return "", nil, err
		}
		if err := tx.QueryRow(ctx, `SELECT stock::float8, min_stock::float8 FROM catalog_items WHERE id = $1`, it.CatalogItemID).Scan(&low, &minStock); err != nil {
			return "", nil, err
		}
		if low <= minStock {
			warnings = append(warnings, "Quedan "+trimQty(low)+" de «"+name+"» (mínimo "+trimQty(minStock)+").")
		}
	}
	if in.Send {
		if err := cbMarkSent(ctx, tx, p, chargeID); err != nil {
			return "", nil, err
		}
	} else if _, err := tx.Exec(ctx, `UPDATE encounter_charges SET updated_at = now() WHERE id = $1`, chargeID); err != nil {
		return "", nil, err
	}
	audit(ctx, tx, p.ClinicID, p, "consult_charge_save", "Guardó la pre-cuenta de una consulta ("+itoa(len(in.Items))+" conceptos)", map[string]any{"charge_id": chargeID, "patient_id": patientID})
	return chargeID, warnings, nil
}

func trimQty(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// cbMarkSent moves a draft to the register's queue; a sent one stays as it is.
func cbMarkSent(ctx context.Context, tx pgx.Tx, p *Principal, chargeID string) error {
	if !cbHasCobros(p) {
		return &httpError{Status: http.StatusForbidden, Code: "PLAN_REQUIRED", Msg: "Enviar a caja viene con los planes Crecimiento y Pro."}
	}
	var billable int
	if err := tx.QueryRow(ctx, `SELECT coalesce(sum(round(qty * unit_price_cents)::int - discount_cents), 0)::int FROM encounter_charge_items WHERE charge_id = $1`, chargeID).Scan(&billable); err != nil {
		return err
	}
	if billable <= 0 {
		return fail(http.StatusBadRequest, "No hay nada que cobrar: agrega al menos un concepto con precio.")
	}
	if _, err := tx.Exec(ctx, `UPDATE encounter_charges SET status = 'sent', sent_at = coalesce(sent_at, now()), updated_at = now() WHERE id = $1 AND status IN ('draft', 'sent')`, chargeID); err != nil {
		return err
	}
	audit(ctx, tx, p.ClinicID, p, "consult_charge_send", "Envió una pre-cuenta de consulta a caja por $"+cents(billable), map[string]any{"charge_id": chargeID})
	return nil
}

func (s *Server) cbWrite(w http.ResponseWriter, r *http.Request, chargeID string, status int) {
	var in cbChargeIn
	if !decode(w, r, &in) {
		return
	}
	if msg := in.clean(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	p := principalFrom(r.Context())
	var warnings []string
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		var err error
		chargeID, warnings, err = s.cbSave(r.Context(), tx, p, chargeID, in)
		return err
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	c, err := s.cbGetOne(r.Context(), p, chargeID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	c.Warnings = warnings
	writeJSON(w, status, map[string]any{"charge": c})
}

func (s *Server) cbCreate(w http.ResponseWriter, r *http.Request) {
	s.cbWrite(w, r, "", http.StatusCreated)
}

func (s *Server) cbUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Pre-cuenta no encontrada.")
		return
	}
	s.cbWrite(w, r, id, http.StatusOK)
}

// cbSend and cbCancel share the lock-check-update shape.
func (s *Server) cbTransition(w http.ResponseWriter, r *http.Request, to string) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Pre-cuenta no encontrada.")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if to == "cancelled" && r.ContentLength != 0 {
		if !decode(w, r, &req) {
			return
		}
		req.Reason = strings.TrimSpace(req.Reason)
		if utf8.RuneCountInString(req.Reason) > 200 {
			writeError(w, http.StatusBadRequest, "El motivo es demasiado largo.")
			return
		}
	}
	p := principalFrom(r.Context())
	kept := 0
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		var status string
		var createdBy, proID *string
		err := tx.QueryRow(r.Context(), `SELECT status, created_by::text, professional_id::text FROM encounter_charges WHERE clinic_id = $1 AND id = $2 FOR UPDATE`, p.ClinicID, id).Scan(&status, &createdBy, &proID)
		if errors.Is(err, pgx.ErrNoRows) {
			return fail(http.StatusNotFound, "Pre-cuenta no encontrada.")
		}
		if err != nil {
			return err
		}
		if !cbCanTouch(p, createdBy, proID) {
			return fail(http.StatusForbidden, "Esta pre-cuenta es de otro profesional.")
		}
		if status != "draft" && status != "sent" {
			return &httpError{Status: http.StatusConflict, Code: "CHARGE_CLOSED", Msg: "La pre-cuenta ya fue cobrada o cancelada."}
		}
		if to == "sent" {
			return cbMarkSent(r.Context(), tx, p, id)
		}
		if err := tx.QueryRow(r.Context(), `SELECT count(*)::int FROM encounter_charge_items WHERE charge_id = $1 AND consumed`, id).Scan(&kept); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `UPDATE encounter_charges SET status = 'cancelled', cancel_reason = $3, cancelled_at = now(), updated_at = now() WHERE clinic_id = $1 AND id = $2`, p.ClinicID, id, req.Reason); err != nil {
			return err
		}
		audit(r.Context(), tx, p.ClinicID, p, "consult_charge_cancel", "Canceló una pre-cuenta de consulta", map[string]any{"charge_id": id, "consumed_lines_kept": kept})
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	c, err := s.cbGetOne(r.Context(), p, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	// supplies already used stay out of stock: they were physically spent
	writeJSON(w, http.StatusOK, map[string]any{"charge": c, "consumed_kept": kept})
}
