package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

var rfcRe = regexp.MustCompile(`^[A-ZÑ&]{3,4}[0-9]{6}[A-Z0-9]{3}$`)

// catalogItem is a service (consultation, procedure) or a product (medicine, supplies) that can be sold.
type catalogItem struct {
	ID         string  `json:"id" db:"id"`
	Kind       string  `json:"kind" db:"kind"`
	Name       string  `json:"name" db:"name"`
	SKU        string  `json:"sku" db:"sku"`
	Barcode    string  `json:"barcode" db:"barcode"`
	Category   string  `json:"category" db:"category"`
	PriceCents int     `json:"price_cents" db:"price_cents"`
	CostCents  int     `json:"cost_cents" db:"cost_cents"`
	TaxRate    float64 `json:"tax_rate" db:"tax_rate"`
	TrackStock bool    `json:"track_stock" db:"track_stock"`
	Stock      float64 `json:"stock" db:"stock"`
	MinStock   float64 `json:"min_stock" db:"min_stock"`
	Unit       string  `json:"unit" db:"unit"`
	Active     bool    `json:"active" db:"active"`
}

const catalogCols = `id, kind, name, sku, barcode, category, price_cents, cost_cents, tax_rate::float8 AS tax_rate,
	track_stock, stock::float8 AS stock, min_stock::float8 AS min_stock, unit, active`

type catalogInput struct {
	Kind       string  `json:"kind"`
	Name       string  `json:"name"`
	SKU        string  `json:"sku"`
	Barcode    string  `json:"barcode"`
	Category   string  `json:"category"`
	PriceCents int     `json:"price_cents"`
	CostCents  int     `json:"cost_cents"`
	TaxRate    float64 `json:"tax_rate"`
	TrackStock bool    `json:"track_stock"`
	Stock      float64 `json:"stock"` // initial stock (create only)
	MinStock   float64 `json:"min_stock"`
	Unit       string  `json:"unit"`
	Active     *bool   `json:"active"`
}

func (in *catalogInput) validate() string {
	in.Name, in.SKU, in.Barcode = strings.TrimSpace(in.Name), strings.TrimSpace(in.SKU), strings.TrimSpace(in.Barcode)
	in.Category, in.Unit = strings.TrimSpace(in.Category), strings.TrimSpace(in.Unit)
	if in.Unit == "" {
		in.Unit = "pza"
	}
	switch {
	case in.Kind != "service" && in.Kind != "product":
		return "El tipo debe ser servicio o producto."
	case in.Name == "" || utf8.RuneCountInString(in.Name) > 160:
		return "El nombre es obligatorio (máximo 160 caracteres)."
	case utf8.RuneCountInString(in.SKU) > 60 || utf8.RuneCountInString(in.Barcode) > 60 || utf8.RuneCountInString(in.Category) > 60 || utf8.RuneCountInString(in.Unit) > 20:
		return "Uno de los campos es demasiado largo."
	case !okCents(in.PriceCents) || !okCents(in.CostCents):
		return "El precio o el costo no son válidos."
	case in.TaxRate < 0 || in.TaxRate > 100:
		return "El IVA debe estar entre 0 y 100."
	case in.Stock < 0 || in.Stock > 1_000_000 || in.MinStock < 0 || in.MinStock > 1_000_000:
		return "La existencia no es válida."
	}
	if in.Kind == "service" {
		in.TrackStock, in.Stock, in.MinStock = false, 0, 0
	}
	return ""
}

func (s *Server) listCatalog(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	q := r.URL.Query()
	where, args := []string{"clinic_id = $1"}, []any{p.ClinicID}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.ReplaceAll(cond, "?", "$"+strconv.Itoa(len(args))))
	}
	if k := q.Get("kind"); k == "service" || k == "product" {
		add("kind = ?", k)
	}
	if q.Get("active") == "1" {
		where = append(where, "active")
	}
	if q.Get("low") == "1" {
		where = append(where, "track_stock AND stock <= min_stock")
	}
	if cat := strings.TrimSpace(q.Get("category")); cat != "" {
		add("category = ?", cat)
	}
	if text := strings.TrimSpace(q.Get("q")); text != "" {
		args = append(args, "%"+escapeLike(text)+"%", text)
		n := len(args)
		where = append(where, fmt.Sprintf("(name ILIKE $%d OR sku ILIKE $%d OR barcode = $%d)", n-1, n-1, n))
	}
	rows, err := s.db.Query(r.Context(),
		`SELECT `+catalogCols+` FROM catalog_items WHERE `+strings.Join(where, " AND ")+` ORDER BY kind DESC, lower(name) LIMIT 2000`, args...)
	if err != nil {
		serverError(w, r, err)
		return
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[catalogItem])
	if err != nil {
		serverError(w, r, err)
		return
	}
	cats, _ := s.categories(r.Context(), p.ClinicID)
	writeJSON(w, http.StatusOK, map[string]any{"items": list, "categories": cats})
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (s *Server) categories(ctx context.Context, clinicID string) ([]string, error) {
	rows, err := s.db.Query(ctx, `SELECT DISTINCT category FROM catalog_items WHERE clinic_id = $1 AND category <> '' ORDER BY 1`, clinicID)
	if err != nil {
		return []string{}, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if out == nil {
		out = []string{}
	}
	return out, err
}

func (s *Server) createCatalogItem(w http.ResponseWriter, r *http.Request) {
	var in catalogInput
	if !decode(w, r, &in) {
		return
	}
	if msg := in.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	p := principalFrom(r.Context())
	var item catalogItem
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `
			INSERT INTO catalog_items (clinic_id, kind, name, sku, barcode, category, price_cents, cost_cents, tax_rate, track_stock, stock, min_stock, unit)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING `+catalogCols,
			p.ClinicID, in.Kind, in.Name, in.SKU, in.Barcode, in.Category, in.PriceCents, in.CostCents, in.TaxRate, in.TrackStock, in.Stock, in.MinStock, in.Unit)
		if err != nil {
			return err
		}
		if item, err = pgx.CollectOneRow(rows, pgx.RowToStructByName[catalogItem]); err != nil {
			return err
		}
		if in.TrackStock && in.Stock > 0 {
			if _, err := tx.Exec(r.Context(), `
				INSERT INTO stock_movements (clinic_id, item_id, delta, reason, balance, created_by_name, note)
				VALUES ($1,$2,$3,'initial',$3,$4,'Existencia inicial')`, p.ClinicID, item.ID, in.Stock, p.actorName()); err != nil {
				return err
			}
		}
		return nil
	})
	if isUniqueViolation(err) {
		writeError(w, http.StatusConflict, "Ya existe un artículo con ese código o código de barras.")
		return
	}
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"item": item})
}

func (s *Server) updateCatalogItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Artículo no encontrado.")
		return
	}
	var in catalogInput
	if !decode(w, r, &in) {
		return
	}
	if msg := in.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	p := principalFrom(r.Context())
	// stock is changed only through stock movements, never by editing the item
	rows, err := s.db.Query(r.Context(), `
		UPDATE catalog_items SET kind=$3, name=$4, sku=$5, barcode=$6, category=$7, price_cents=$8, cost_cents=$9, tax_rate=$10,
		       track_stock=$11, min_stock=$12, unit=$13, active=$14, updated_at=now()
		WHERE clinic_id=$1 AND id=$2 RETURNING `+catalogCols,
		p.ClinicID, id, in.Kind, in.Name, in.SKU, in.Barcode, in.Category, in.PriceCents, in.CostCents, in.TaxRate, in.TrackStock, in.MinStock, in.Unit, active)
	if err != nil {
		serverError(w, r, err)
		return
	}
	item, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[catalogItem])
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Artículo no encontrado.")
		return
	}
	if isUniqueViolation(err) {
		writeError(w, http.StatusConflict, "Ya existe un artículo con ese código o código de barras.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

// deleteCatalogItem removes an item nobody has sold yet; otherwise it is archived so history stays readable.
func (s *Server) deleteCatalogItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Artículo no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	var archived bool
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		var used bool
		if err := tx.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM sale_items WHERE item_id = $1)`, id).Scan(&used); err != nil {
			return err
		}
		var tag int64
		if used {
			archived = true
			ct, err := tx.Exec(r.Context(), `UPDATE catalog_items SET active=false, updated_at=now() WHERE clinic_id=$1 AND id=$2`, p.ClinicID, id)
			if err != nil {
				return err
			}
			tag = ct.RowsAffected()
		} else {
			ct, err := tx.Exec(r.Context(), `DELETE FROM catalog_items WHERE clinic_id=$1 AND id=$2`, p.ClinicID, id)
			if err != nil {
				return err
			}
			tag = ct.RowsAffected()
		}
		if tag == 0 {
			return fail(http.StatusNotFound, "Artículo no encontrado.")
		}
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"archived": archived})
}

// ---------------------------------------------------------------------------
// Stock
// ---------------------------------------------------------------------------

type stockMovement struct {
	ID        string  `json:"id" db:"id"`
	Delta     float64 `json:"delta" db:"delta"`
	Reason    string  `json:"reason" db:"reason"`
	Note      string  `json:"note" db:"note"`
	Balance   float64 `json:"balance" db:"balance"`
	By        string  `json:"by" db:"created_by_name"`
	CreatedAt string  `json:"created_at" db:"created_at"`
}

var stockReasons = []string{"purchase", "adjustment", "loss"}

type stockInput struct {
	Delta  float64 `json:"delta"`
	Reason string  `json:"reason"`
	Note   string  `json:"note"`
	// SetTo, when present, sets the count to an exact value (physical inventory); Delta is ignored.
	SetTo *float64 `json:"set_to"`
}

func (s *Server) adjustStock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Artículo no encontrado.")
		return
	}
	var in stockInput
	if !decode(w, r, &in) {
		return
	}
	in.Note = strings.TrimSpace(in.Note)
	if !hasPermission(stockReasons, in.Reason) {
		writeError(w, http.StatusBadRequest, "Motivo inválido.")
		return
	}
	if utf8.RuneCountInString(in.Note) > 200 {
		writeError(w, http.StatusBadRequest, "La nota es demasiado larga.")
		return
	}
	p := principalFrom(r.Context())
	var balance float64
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		var cur float64
		var track bool
		err := tx.QueryRow(r.Context(), `SELECT stock::float8, track_stock FROM catalog_items WHERE clinic_id=$1 AND id=$2 FOR UPDATE`, p.ClinicID, id).Scan(&cur, &track)
		if errors.Is(err, pgx.ErrNoRows) {
			return fail(http.StatusNotFound, "Artículo no encontrado.")
		}
		if err != nil {
			return err
		}
		if !track {
			return fail(http.StatusConflict, "Este artículo no controla existencias. Actívalo en su ficha.")
		}
		delta := in.Delta
		if in.SetTo != nil {
			if *in.SetTo < 0 || *in.SetTo > 1_000_000 {
				return fail(http.StatusBadRequest, "La existencia no es válida.")
			}
			delta = *in.SetTo - cur
			in.Reason = "adjustment"
		}
		if delta == 0 || delta > 1_000_000 || delta < -1_000_000 {
			return fail(http.StatusBadRequest, "La cantidad no es válida.")
		}
		if (in.Reason == "purchase" && delta < 0) || (in.Reason == "loss" && delta > 0) {
			return fail(http.StatusBadRequest, "La cantidad no corresponde al motivo.")
		}
		balance = cur + delta
		if balance < 0 {
			return fail(http.StatusConflict, "La existencia no puede quedar en negativo.")
		}
		if _, err := tx.Exec(r.Context(), `UPDATE catalog_items SET stock=$3, updated_at=now() WHERE clinic_id=$1 AND id=$2`, p.ClinicID, id, balance); err != nil {
			return err
		}
		_, err = tx.Exec(r.Context(), `
			INSERT INTO stock_movements (clinic_id, item_id, delta, reason, note, balance, created_by_name) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			p.ClinicID, id, delta, in.Reason, in.Note, balance, p.actorName())
		return err
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stock": balance})
}

func (s *Server) listStockMovements(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Artículo no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(), `
		SELECT id, delta::float8 AS delta, reason, note, balance::float8 AS balance, created_by_name, to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SSOF') AS created_at
		FROM stock_movements WHERE clinic_id=$1 AND item_id=$2 ORDER BY created_at DESC LIMIT 200`, p.ClinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[stockMovement])
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"movements": list})
}

// importCatalog creates many items at once (spreadsheet paste, or the magic inventory once confirmed).
// Items whose SKU or barcode already exist are skipped and reported.
func (s *Server) importCatalog(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items []catalogInput `json:"items"`
	}
	if !decode(w, r, &req) {
		return
	}
	if len(req.Items) == 0 || len(req.Items) > 500 {
		writeError(w, http.StatusBadRequest, "Envía entre 1 y 500 artículos.")
		return
	}
	for i := range req.Items {
		if msg := req.Items[i].validate(); msg != "" {
			writeError(w, http.StatusBadRequest, "Fila "+itoa(i+1)+": "+msg)
			return
		}
	}
	p := principalFrom(r.Context())
	created, skipped := 0, []string{}
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		for _, in := range req.Items {
			var id string
			err := tx.QueryRow(r.Context(), `
				INSERT INTO catalog_items (clinic_id, kind, name, sku, barcode, category, price_cents, cost_cents, tax_rate, track_stock, stock, min_stock, unit)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
				ON CONFLICT DO NOTHING RETURNING id`,
				p.ClinicID, in.Kind, in.Name, in.SKU, in.Barcode, in.Category, in.PriceCents, in.CostCents, in.TaxRate, in.TrackStock, in.Stock, in.MinStock, in.Unit).Scan(&id)
			if errors.Is(err, pgx.ErrNoRows) {
				skipped = append(skipped, in.Name)
				continue
			}
			if err != nil {
				return err
			}
			created++
			if in.TrackStock && in.Stock > 0 {
				if _, err := tx.Exec(r.Context(), `
					INSERT INTO stock_movements (clinic_id, item_id, delta, reason, balance, created_by_name, note)
					VALUES ($1,$2,$3,'initial',$3,$4,'Existencia inicial')`, p.ClinicID, id, in.Stock, p.actorName()); err != nil {
					return err
				}
			}
		}
		audit(r.Context(), tx, p.ClinicID, p, "catalog_import", "Importó "+itoa(created)+" artículos al catálogo", nil)
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"created": created, "skipped": skipped})
}
