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

// Lots and expiry. catalog_items.stock is the total of the item's stock_lots; every change goes through
// stockApply so both stay equal. Sales use the lot that expires first (FEFO) and never an expired one,
// unless an administrator overrides it with a reason.

const (
	noLotCode = "SIN LOTE"
	qtyEps    = 0.0005
)

func today() string { return time.Now().Format("2006-01-02") }

// stockOp describes one stock change to record against a lot.
type stockOp struct {
	clinicID, itemID, lotID string
	reason                  string
	saleID, encounterID     string // optional
	chargeItemID            string // optional: the pre-account line this consumption belongs to
	note, actor             string
}

// ensureLot returns the lot with that code and expiry, creating it empty when it does not exist.
func ensureLot(ctx context.Context, tx pgx.Tx, clinicID, itemID, code, expires string) (string, error) {
	if code == "" {
		code = noLotCode
	}
	var exp any
	if expires != "" {
		exp = expires
	}
	var id string
	err := tx.QueryRow(ctx, `
		SELECT id FROM stock_lots
		WHERE item_id = $1 AND lower(lot_code) = lower($2) AND expires_on IS NOT DISTINCT FROM $3::date`, itemID, code, exp).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	err = tx.QueryRow(ctx, `INSERT INTO stock_lots (clinic_id, item_id, lot_code, expires_on) VALUES ($1,$2,$3,$4::date) RETURNING id`,
		clinicID, itemID, code, exp).Scan(&id)
	return id, err
}

// stockApply adds delta to a lot and to the item total and records the movement. It returns the new total.
func stockApply(ctx context.Context, tx pgx.Tx, op stockOp, delta float64) (float64, error) {
	var code string
	if err := tx.QueryRow(ctx, `UPDATE stock_lots SET qty = qty + $2 WHERE id = $1 RETURNING lot_code`, op.lotID, delta).Scan(&code); err != nil {
		return 0, err
	}
	var bal float64
	if err := tx.QueryRow(ctx, `UPDATE catalog_items SET stock = stock + $2, updated_at = now() WHERE id = $1 RETURNING stock::float8`, op.itemID, delta).Scan(&bal); err != nil {
		return 0, err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO stock_movements (clinic_id, item_id, delta, reason, sale_id, encounter_id, note, balance, created_by_name, lot_id, lot_code, charge_item_id)
		VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid,NULLIF($6,'')::uuid,$7,$8,$9,$10,$11,NULLIF($12,'')::uuid)`,
		op.clinicID, op.itemID, delta, op.reason, op.saleID, op.encounterID, op.note, bal, op.actor, op.lotID, code, op.chargeItemID)
	return bal, err
}

// seedInitialStock records the opening stock of a new item in its first lot.
func seedInitialStock(ctx context.Context, tx pgx.Tx, clinicID, itemID string, qty float64, code, expires, actor string) error {
	if qty <= 0 {
		return nil
	}
	lot, err := ensureLot(ctx, tx, clinicID, itemID, code, expires)
	if err != nil {
		return err
	}
	// the item row already carries the quantity: only the lot and the movement are added
	var lotCode string
	if err := tx.QueryRow(ctx, `UPDATE stock_lots SET qty = qty + $2 WHERE id = $1 RETURNING lot_code`, lot, qty).Scan(&lotCode); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO stock_movements (clinic_id, item_id, delta, reason, balance, created_by_name, note, lot_id, lot_code)
		VALUES ($1,$2,$3,'initial',$3,$4,'Existencia inicial',$5,$6)`, clinicID, itemID, qty, actor, lot, lotCode)
	return err
}

type consumeResult struct {
	Short   float64 // not covered by sellable lots
	Expired float64 // stock that exists but is expired (and was not allowed)
}

// consumeStock takes qty from the item's lots, earliest expiry first. Expired lots are skipped unless
// allowExpired. What the lots cannot cover goes to a negative "SIN LOTE" lot when allowNegative, and
// is otherwise reported in Short.
func consumeStock(ctx context.Context, tx pgx.Tx, op stockOp, qty float64, allowExpired, allowNegative bool) (consumeResult, error) {
	type lot struct {
		id      string
		qty     float64
		expired bool
	}
	rows, err := tx.Query(ctx, `
		SELECT id, qty::float8, expires_on IS NOT NULL AND expires_on < $2::date
		FROM stock_lots WHERE item_id = $1 AND qty > 0
		ORDER BY expires_on NULLS LAST, created_at, id FOR UPDATE`, op.itemID, today())
	if err != nil {
		return consumeResult{}, err
	}
	var lots []lot
	for rows.Next() {
		var l lot
		if err := rows.Scan(&l.id, &l.qty, &l.expired); err != nil {
			rows.Close()
			return consumeResult{}, err
		}
		lots = append(lots, l)
	}
	rows.Close()
	if rows.Err() != nil {
		return consumeResult{}, rows.Err()
	}
	var res consumeResult
	remaining := qty
	for _, l := range lots {
		if remaining <= qtyEps {
			break
		}
		if l.expired && !allowExpired {
			res.Expired += l.qty
			continue
		}
		take := l.qty
		if take > remaining {
			take = remaining
		}
		o := op
		o.lotID = l.id
		if _, err := stockApply(ctx, tx, o, -take); err != nil {
			return res, err
		}
		remaining -= take
	}
	if remaining > qtyEps && allowNegative {
		id, err := ensureLot(ctx, tx, op.clinicID, op.itemID, noLotCode, "")
		if err != nil {
			return res, err
		}
		o := op
		o.lotID = id
		if _, err := stockApply(ctx, tx, o, -remaining); err != nil {
			return res, err
		}
		remaining = 0
	}
	if remaining > qtyEps {
		res.Short = remaining
	}
	return res, nil
}

// restockSale puts back, lot by lot, everything a sale took (items and service consumables).
func restockSale(ctx context.Context, tx pgx.Tx, clinicID, saleID, note, actor string) error {
	type back struct {
		item, lot string
		qty       float64
	}
	rows, err := tx.Query(ctx, `
		SELECT item_id, coalesce(lot_id::text, ''), -delta::float8 FROM stock_movements
		WHERE clinic_id = $1 AND sale_id = $2 AND reason IN ('sale', 'consumption') AND delta < 0 ORDER BY created_at, id`, clinicID, saleID)
	if err != nil {
		return err
	}
	var backs []back
	for rows.Next() {
		var b back
		if err := rows.Scan(&b.item, &b.lot, &b.qty); err != nil {
			rows.Close()
			return err
		}
		backs = append(backs, b)
	}
	rows.Close()
	if rows.Err() != nil {
		return rows.Err()
	}
	for _, b := range backs {
		if b.lot == "" {
			if b.lot, err = ensureLot(ctx, tx, clinicID, b.item, noLotCode, ""); err != nil {
				return err
			}
		}
		if _, err := stockApply(ctx, tx, stockOp{clinicID: clinicID, itemID: b.item, lotID: b.lot, reason: "void", saleID: saleID, note: note, actor: actor}, b.qty); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Lots of an item and alerts
// ---------------------------------------------------------------------------

type stockLot struct {
	ID        string  `json:"id"`
	LotCode   string  `json:"lot_code"`
	ExpiresOn *string `json:"expires_on"`
	Qty       float64 `json:"qty"`
	Expired   bool    `json:"expired"`
}

func (s *Server) listItemLots(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Artículo no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(), `
		SELECT id, lot_code, to_char(expires_on, 'YYYY-MM-DD'), qty::float8, expires_on IS NOT NULL AND expires_on < $3::date
		FROM stock_lots WHERE clinic_id = $1 AND item_id = $2 AND qty <> 0
		ORDER BY expires_on NULLS LAST, created_at`, p.ClinicID, id, today())
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	list := []stockLot{}
	for rows.Next() {
		var l stockLot
		if err := rows.Scan(&l.ID, &l.LotCode, &l.ExpiresOn, &l.Qty, &l.Expired); err != nil {
			serverError(w, r, err)
			return
		}
		list = append(list, l)
	}
	writeJSON(w, http.StatusOK, map[string]any{"lots": list})
}

type lotAlert struct {
	ItemID    string  `json:"item_id"`
	Name      string  `json:"name"`
	Unit      string  `json:"unit"`
	LotID     string  `json:"lot_id"`
	LotCode   string  `json:"lot_code"`
	ExpiresOn string  `json:"expires_on"`
	Qty       float64 `json:"qty"`
	DaysLeft  int     `json:"days_left"` // negative when already expired
}

type lowAlert struct {
	ItemID   string  `json:"item_id"`
	Name     string  `json:"name"`
	Unit     string  `json:"unit"`
	Stock    float64 `json:"stock"`
	MinStock float64 `json:"min_stock"`
}

// posAlerts lists lots that expire within `days` (default 60), lots already expired and items at or below their minimum.
func (s *Server) posAlerts(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	days := 60
	if v := strings.TrimSpace(r.URL.Query().Get("days")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 365 {
			writeError(w, http.StatusBadRequest, "Los días deben estar entre 1 y 365.")
			return
		}
		days = n
	}
	rows, err := s.db.Query(r.Context(), `
		SELECT c.id, c.name, c.unit, l.id, l.lot_code, to_char(l.expires_on, 'YYYY-MM-DD'), l.qty::float8, (l.expires_on - $2::date)
		FROM stock_lots l JOIN catalog_items c ON c.id = l.item_id
		WHERE l.clinic_id = $1 AND c.track_stock AND c.active AND l.qty > 0 AND l.expires_on IS NOT NULL
		  AND l.expires_on <= $2::date + $3::int
		ORDER BY l.expires_on, lower(c.name)`, p.ClinicID, today(), days)
	if err != nil {
		serverError(w, r, err)
		return
	}
	expiring, expired := []lotAlert{}, []lotAlert{}
	for rows.Next() {
		var a lotAlert
		if err := rows.Scan(&a.ItemID, &a.Name, &a.Unit, &a.LotID, &a.LotCode, &a.ExpiresOn, &a.Qty, &a.DaysLeft); err != nil {
			rows.Close()
			serverError(w, r, err)
			return
		}
		if a.DaysLeft < 0 {
			expired = append(expired, a)
		} else {
			expiring = append(expiring, a)
		}
	}
	rows.Close()
	lrows, err := s.db.Query(r.Context(), `
		SELECT id, name, unit, stock::float8, min_stock::float8 FROM catalog_items
		WHERE clinic_id = $1 AND track_stock AND active AND stock <= min_stock ORDER BY lower(name) LIMIT 200`, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer lrows.Close()
	low := []lowAlert{}
	for lrows.Next() {
		var a lowAlert
		if err := lrows.Scan(&a.ItemID, &a.Name, &a.Unit, &a.Stock, &a.MinStock); err != nil {
			serverError(w, r, err)
			return
		}
		low = append(low, a)
	}
	writeJSON(w, http.StatusOK, map[string]any{"days": days, "expiring": expiring, "expired": expired, "low": low})
}

// ---------------------------------------------------------------------------
// Service consumables
// ---------------------------------------------------------------------------

type consumable struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Unit      string  `json:"unit"`
	Qty       float64 `json:"qty"`
}

func (s *Server) getConsumables(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Servicio no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	list, err := loadConsumables(r.Context(), s.db, p.ClinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"consumables": list})
}

func loadConsumables(ctx context.Context, q rowsQuerier, clinicID, serviceID string) ([]consumable, error) {
	rows, err := q.Query(ctx, `
		SELECT sc.product_id, c.name, c.unit, sc.qty::float8 FROM service_consumables sc JOIN catalog_items c ON c.id = sc.product_id
		WHERE sc.clinic_id = $1 AND sc.service_id = $2 ORDER BY lower(c.name)`, clinicID, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []consumable{}
	for rows.Next() {
		var c consumable
		if err := rows.Scan(&c.ProductID, &c.Name, &c.Unit, &c.Qty); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// setConsumables replaces the supplies a service uses.
func (s *Server) setConsumables(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Servicio no encontrado.")
		return
	}
	var req struct {
		Items []struct {
			ProductID string  `json:"product_id"`
			Qty       float64 `json:"qty"`
		} `json:"items"`
	}
	if !decode(w, r, &req) {
		return
	}
	if len(req.Items) > 40 {
		writeError(w, http.StatusBadRequest, "Un servicio admite hasta 40 consumibles.")
		return
	}
	seen := map[string]bool{}
	for _, it := range req.Items {
		if !validUUID(it.ProductID) || it.Qty <= 0 || it.Qty > 100_000 {
			writeError(w, http.StatusBadRequest, "Un consumible no es válido.")
			return
		}
		if seen[it.ProductID] {
			writeError(w, http.StatusBadRequest, "Un producto está repetido en los consumibles.")
			return
		}
		seen[it.ProductID] = true
	}
	p := principalFrom(r.Context())
	var list []consumable
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		var kind, name string
		err := tx.QueryRow(r.Context(), `SELECT kind, name FROM catalog_items WHERE clinic_id = $1 AND id = $2`, p.ClinicID, id).Scan(&kind, &name)
		if errors.Is(err, pgx.ErrNoRows) {
			return fail(http.StatusNotFound, "Servicio no encontrado.")
		}
		if err != nil {
			return err
		}
		if kind != "service" {
			return fail(http.StatusConflict, "Solo los servicios tienen consumibles.")
		}
		if _, err := tx.Exec(r.Context(), `DELETE FROM service_consumables WHERE service_id = $1`, id); err != nil {
			return err
		}
		for _, it := range req.Items {
			tag, err := tx.Exec(r.Context(), `
				INSERT INTO service_consumables (clinic_id, service_id, product_id, qty)
				SELECT $1, $2, c.id, $4 FROM catalog_items c WHERE c.clinic_id = $1 AND c.id = $3 AND c.kind = 'product'`, p.ClinicID, id, it.ProductID, it.Qty)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return fail(http.StatusBadRequest, "Un consumible no existe en tu catálogo de productos.")
			}
		}
		audit(r.Context(), tx, p.ClinicID, p, "service_consumables", "Actualizó los consumibles de «"+name+"»", nil)
		list, err = loadConsumables(r.Context(), tx, p.ClinicID, id)
		return err
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"consumables": list})
}

// consumeForService takes a sold service's supplies out of stock. A shortage never blocks the sale: what
// could not be taken comes back as warnings.
func consumeForService(ctx context.Context, tx pgx.Tx, clinicID, serviceID, serviceName string, qty float64, saleID, actor string, allowExpired, allowNegative bool) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT sc.product_id, c.name, c.unit, sc.qty::float8 FROM service_consumables sc
		JOIN catalog_items c ON c.id = sc.product_id
		WHERE sc.clinic_id = $1 AND sc.service_id = $2 AND c.track_stock`, clinicID, serviceID)
	if err != nil {
		return nil, err
	}
	type need struct {
		item, name, unit string
		qty              float64
	}
	var needs []need
	for rows.Next() {
		var n need
		if err := rows.Scan(&n.item, &n.name, &n.unit, &n.qty); err != nil {
			rows.Close()
			return nil, err
		}
		needs = append(needs, n)
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	var warns []string
	for _, n := range needs {
		if _, err := tx.Exec(ctx, `SELECT 1 FROM catalog_items WHERE id = $1 FOR UPDATE`, n.item); err != nil {
			return nil, err
		}
		want := n.qty * qty
		res, err := consumeStock(ctx, tx, stockOp{clinicID: clinicID, itemID: n.item, reason: "consumption", saleID: saleID,
			note: "Consumo de servicio " + serviceName, actor: actor}, want, allowExpired, allowNegative)
		if err != nil {
			return nil, err
		}
		if res.Short > qtyEps {
			warns = append(warns, "Faltaron "+qtyStr(res.Short)+" "+n.unit+" de «"+n.name+"» para «"+serviceName+"»"+
				map[bool]string{true: " (las existencias están caducadas)", false: ""}[res.Expired > 0])
		}
	}
	return warns, nil
}

// encounterConsumables records supplies used during a procedure without selling them.
func (s *Server) recordEncounterConsumables(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Atención no encontrada.")
		return
	}
	var req struct {
		Items []struct {
			ItemID string  `json:"item_id"`
			Qty    float64 `json:"qty"`
		} `json:"items"`
	}
	if !decode(w, r, &req) {
		return
	}
	if len(req.Items) == 0 || len(req.Items) > 50 {
		writeError(w, http.StatusBadRequest, "Agrega entre 1 y 50 insumos.")
		return
	}
	for _, it := range req.Items {
		if !validUUID(it.ItemID) || it.Qty <= 0 || it.Qty > 100_000 {
			writeError(w, http.StatusBadRequest, "Un insumo no es válido.")
			return
		}
	}
	p := principalFrom(r.Context())
	out := []map[string]any{}
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		var patientID string
		err := tx.QueryRow(r.Context(), `SELECT patient_id FROM encounters WHERE clinic_id = $1 AND id = $2`, p.ClinicID, id).Scan(&patientID)
		if errors.Is(err, pgx.ErrNoRows) {
			return fail(http.StatusNotFound, "Atención no encontrada.")
		}
		if err != nil {
			return err
		}
		cfg, err := loadPosSettings(r.Context(), tx, p.ClinicID)
		if err != nil {
			return err
		}
		for _, it := range req.Items {
			var name, unit string
			var track bool
			err := tx.QueryRow(r.Context(), `SELECT name, unit, track_stock FROM catalog_items WHERE clinic_id = $1 AND id = $2 AND kind = 'product' FOR UPDATE`, p.ClinicID, it.ItemID).Scan(&name, &unit, &track)
			if errors.Is(err, pgx.ErrNoRows) {
				return fail(http.StatusBadRequest, "Un insumo no existe en tu catálogo de productos.")
			}
			if err != nil {
				return err
			}
			if !track {
				return fail(http.StatusConflict, "«"+name+"» no controla existencias.")
			}
			res, err := consumeStock(r.Context(), tx, stockOp{clinicID: p.ClinicID, itemID: it.ItemID, reason: "consumption", encounterID: id,
				note: "Consumo en procedimiento", actor: p.actorName()}, it.Qty, false, cfg.AllowNegativeStock)
			if err != nil {
				return err
			}
			if res.Short > qtyEps {
				if res.Expired > 0 {
					return &httpError{Status: http.StatusConflict, Code: "LOT_EXPIRED", Msg: "Las existencias de «" + name + "» están caducadas; no se pueden usar."}
				}
				return &httpError{Status: http.StatusConflict, Code: "NO_STOCK", Msg: "No hay existencias suficientes de «" + name + "»."}
			}
			out = append(out, map[string]any{"item_id": it.ItemID, "name": name, "qty": it.Qty, "unit": unit})
		}
		audit(r.Context(), tx, p.ClinicID, p, "encounter_consumables", "Registró el consumo de "+itoa(len(req.Items))+" insumo(s) en un procedimiento", map[string]any{"encounter_id": id})
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"consumed": out})
}

// listEncounterConsumables shows the supplies recorded for an encounter.
func (s *Server) listEncounterConsumables(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Atención no encontrada.")
		return
	}
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(), `
		SELECT c.name, c.unit, -m.delta::float8, m.lot_code, m.created_at
		FROM stock_movements m JOIN catalog_items c ON c.id = m.item_id
		WHERE m.clinic_id = $1 AND m.encounter_id = $2 ORDER BY m.created_at`, p.ClinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		var name, unit, lot string
		var qty float64
		var at time.Time
		if err := rows.Scan(&name, &unit, &qty, &lot, &at); err != nil {
			serverError(w, r, err)
			return
		}
		list = append(list, map[string]any{"name": name, "unit": unit, "qty": qty, "lot_code": lot, "at": at})
	}
	writeJSON(w, http.StatusOK, map[string]any{"consumed": list})
}

// validLotInput checks the lot fields of an entry.
func validLotInput(code, expires string) string {
	if utf8.RuneCountInString(code) > 40 {
		return "El lote es demasiado largo (máximo 40 caracteres)."
	}
	if expires != "" {
		t, err := time.Parse("2006-01-02", expires)
		if err != nil || t.Year() < 2000 || t.Year() > 2100 {
			return "La fecha de caducidad no es válida."
		}
	}
	return ""
}
