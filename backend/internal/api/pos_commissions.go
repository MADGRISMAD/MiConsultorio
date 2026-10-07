package api

import (
	"context"
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Commissions. A rule applies to an item, a category, a professional or everyone; the most specific
// one wins: item > category > professional > general. Among rules of the same kind, one that names the
// professional beats one for everybody, and the newest breaks any remaining tie. The percentage and the
// amount are copied into the sale line when it is sold, so later changes to the rules never alter history.
// The base is what was charged for the line after discounts, without the tax.

type commissionRule struct {
	ID        string    `json:"id"`
	UserID    *string   `json:"user_id"`
	UserName  string    `json:"user_name"`
	ItemID    *string   `json:"item_id"`
	ItemName  string    `json:"item_name"`
	Category  *string   `json:"category"`
	Percent   float64   `json:"percent"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"-"`
}

func (r commissionRule) rank() int {
	tier := 0
	switch {
	case r.ItemID != nil:
		tier = 3
	case r.Category != nil:
		tier = 2
	case r.UserID != nil:
		tier = 1
	}
	rank := tier * 2
	if r.UserID != nil {
		rank++
	}
	return rank
}

// pickCommission returns the percentage that applies to a line (0 when no rule does).
func pickCommission(rules []commissionRule, professionalID, itemID, category string) float64 {
	var best *commissionRule
	for i := range rules {
		r := &rules[i]
		if !r.Active {
			continue
		}
		if r.UserID != nil && (professionalID == "" || *r.UserID != professionalID) {
			continue
		}
		if r.ItemID != nil && (itemID == "" || *r.ItemID != itemID) {
			continue
		}
		if r.Category != nil && (category == "" || !strings.EqualFold(*r.Category, category)) {
			continue
		}
		if best == nil || r.rank() > best.rank() || (r.rank() == best.rank() && r.CreatedAt.After(best.CreatedAt)) {
			best = r
		}
	}
	if best == nil {
		return 0
	}
	return best.Percent
}

func loadCommissionRules(ctx context.Context, q rowsQuerier, clinicID string, onlyActive bool) ([]commissionRule, error) {
	rows, err := q.Query(ctx, `
		SELECT r.id, r.user_id::text, coalesce(u.name, ''), r.item_id::text, coalesce(c.name, ''), r.category, r.percent::float8, r.active, r.created_at
		FROM commission_rules r
		LEFT JOIN users u ON u.id = r.user_id
		LEFT JOIN catalog_items c ON c.id = r.item_id
		WHERE r.clinic_id = $1 AND ($2 = false OR r.active)
		ORDER BY r.created_at, r.id`, clinicID, onlyActive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []commissionRule{}
	for rows.Next() {
		var r commissionRule
		if err := rows.Scan(&r.ID, &r.UserID, &r.UserName, &r.ItemID, &r.ItemName, &r.Category, &r.Percent, &r.Active, &r.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, rows.Err()
}

func (s *Server) listCommissionRules(w http.ResponseWriter, r *http.Request) {
	list, err := loadCommissionRules(r.Context(), s.db, principalFrom(r.Context()).ClinicID, false)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": list})
}

type commissionRuleIn struct {
	UserID   string  `json:"user_id"`
	ItemID   string  `json:"item_id"`
	Category string  `json:"category"`
	Percent  float64 `json:"percent"`
	Active   *bool   `json:"active"`
}

func (in *commissionRuleIn) validate() string {
	in.Category = strings.TrimSpace(in.Category)
	switch {
	case in.UserID != "" && !validUUID(in.UserID):
		return "El profesional no es válido."
	case in.ItemID != "" && !validUUID(in.ItemID):
		return "El artículo no es válido."
	case in.ItemID != "" && in.Category != "":
		return "Elige un artículo o una categoría, no ambos."
	case utf8.RuneCountInString(in.Category) > 60:
		return "La categoría es demasiado larga."
	case in.Percent < 0 || in.Percent > 100:
		return "El porcentaje debe estar entre 0 y 100."
	}
	return ""
}

// checkRuleRefs makes sure the professional and the item belong to the clinic.
func checkRuleRefs(ctx context.Context, tx pgx.Tx, clinicID string, in commissionRuleIn) error {
	if in.UserID != "" {
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE clinic_id = $1 AND id = $2)`, clinicID, in.UserID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return fail(http.StatusBadRequest, "El profesional no existe en este consultorio.")
		}
	}
	if in.ItemID != "" {
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM catalog_items WHERE clinic_id = $1 AND id = $2)`, clinicID, in.ItemID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return fail(http.StatusBadRequest, "El artículo no existe en tu catálogo.")
		}
	}
	return nil
}

func (s *Server) saveCommissionRule(create bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if !create && !validUUID(id) {
			writeError(w, http.StatusNotFound, "Regla no encontrada.")
			return
		}
		var in commissionRuleIn
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
		err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
			if err := checkRuleRefs(r.Context(), tx, p.ClinicID, in); err != nil {
				return err
			}
			if create {
				err := tx.QueryRow(r.Context(), `
					INSERT INTO commission_rules (clinic_id, user_id, item_id, category, percent, active)
					VALUES ($1, NULLIF($2,'')::uuid, NULLIF($3,'')::uuid, NULLIF($4,''), $5, $6) RETURNING id`,
					p.ClinicID, in.UserID, in.ItemID, in.Category, in.Percent, active).Scan(&id)
				if err != nil {
					return err
				}
				audit(r.Context(), tx, p.ClinicID, p, "commission_rule", "Creó una regla de comisión del "+strconv.FormatFloat(in.Percent, 'f', -1, 64)+" %", nil)
				return nil
			}
			tag, err := tx.Exec(r.Context(), `
				UPDATE commission_rules SET user_id = NULLIF($3,'')::uuid, item_id = NULLIF($4,'')::uuid, category = NULLIF($5,''), percent = $6, active = $7
				WHERE clinic_id = $1 AND id = $2`, p.ClinicID, id, in.UserID, in.ItemID, in.Category, in.Percent, active)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return fail(http.StatusNotFound, "Regla no encontrada.")
			}
			audit(r.Context(), tx, p.ClinicID, p, "commission_rule", "Actualizó una regla de comisión", nil)
			return nil
		})
		if err != nil {
			writeFailure(w, r, err)
			return
		}
		list, err := loadCommissionRules(r.Context(), s.db, p.ClinicID, false)
		if err != nil {
			serverError(w, r, err)
			return
		}
		for _, x := range list {
			if x.ID == id {
				status := http.StatusOK
				if create {
					status = http.StatusCreated
				}
				writeJSON(w, status, map[string]any{"rule": x})
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

func (s *Server) deleteCommissionRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Regla no encontrada.")
		return
	}
	p := principalFrom(r.Context())
	tag, err := s.db.Exec(r.Context(), `DELETE FROM commission_rules WHERE clinic_id = $1 AND id = $2`, p.ClinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "Regla no encontrada.")
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "commission_rule", "Eliminó una regla de comisión", nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------------------------------------------------------------------------
// Professionals (who can be credited with a sale)
// ---------------------------------------------------------------------------

func (s *Server) posProfessionals(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(), `
		SELECT id, name, role FROM users WHERE clinic_id = $1 AND NOT disabled AND role IN ('admin', 'doctor') ORDER BY name, id`, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	type pro struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Role string `json:"role"`
	}
	list := []pro{}
	for rows.Next() {
		var x pro
		if err := rows.Scan(&x.ID, &x.Name, &x.Role); err != nil {
			serverError(w, r, err)
			return
		}
		list = append(list, x)
	}
	writeJSON(w, http.StatusOK, map[string]any{"professionals": list})
}

// validProfessional reports whether id is an active member of the clinic.
func validProfessional(ctx context.Context, q queryRower, clinicID, id string) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE clinic_id = $1 AND id = $2 AND NOT disabled)`, clinicID, id).Scan(&ok)
	return ok, err
}

// ---------------------------------------------------------------------------
// Report
// ---------------------------------------------------------------------------

type commissionLine struct {
	Folio          int       `json:"folio"`
	Date           time.Time `json:"date"`
	ProfessionalID *string   `json:"professional_id"`
	Professional   string    `json:"professional"`
	Item           string    `json:"item"`
	BaseCents      int       `json:"base_cents"`
	Percent        float64   `json:"percent"`
	Cents          int       `json:"commission_cents"`
}

type commissionTotal struct {
	ProfessionalID *string `json:"professional_id"`
	Professional   string  `json:"professional"`
	Lines          int     `json:"lines"`
	BaseCents      int     `json:"base_cents"`
	Cents          int     `json:"commission_cents"`
}

// commissionData loads the paid lines of the period; professional filters by user id ("none" = without professional).
func (s *Server) commissionData(r *http.Request) (from, to time.Time, totals []commissionTotal, lines []commissionLine, status int, msg string, err error) {
	p := principalFrom(r.Context())
	q := r.URL.Query()
	from, to, msg = dayRange(q.Get("from"), q.Get("to"))
	if msg != "" {
		return from, to, nil, nil, http.StatusBadRequest, msg, nil
	}
	prof := q.Get("professional")
	where, args := `s.clinic_id = $1 AND s.status = 'paid' AND s.created_at >= $2 AND s.created_at < $3`, []any{p.ClinicID, from, to}
	switch {
	case prof == "none":
		where += ` AND coalesce(si.professional_id, s.professional_id) IS NULL`
	case prof != "":
		if !validUUID(prof) {
			return from, to, nil, nil, http.StatusBadRequest, "El profesional no es válido.", nil
		}
		args = append(args, prof)
		where += ` AND coalesce(si.professional_id, s.professional_id) = $4`
	}
	rows, err := s.db.Query(r.Context(), `
		SELECT s.folio, s.created_at, coalesce(si.professional_id, s.professional_id)::text, coalesce(u.name, ''), si.name,
		       si.commission_pct::float8, si.commission_cents,
		       si.commission_base_cents
		FROM sale_items si JOIN sales s ON s.id = si.sale_id
		LEFT JOIN users u ON u.id = coalesce(si.professional_id, s.professional_id)
		WHERE `+where+` ORDER BY s.created_at, s.folio, si.name`, args...)
	if err != nil {
		return from, to, nil, nil, 0, "", err
	}
	defer rows.Close()
	lines = []commissionLine{}
	index := map[string]int{}
	totals = []commissionTotal{}
	for rows.Next() {
		var l commissionLine
		if err := rows.Scan(&l.Folio, &l.Date, &l.ProfessionalID, &l.Professional, &l.Item, &l.Percent, &l.Cents, &l.BaseCents); err != nil {
			return from, to, nil, nil, 0, "", err
		}
		if l.Professional == "" {
			l.Professional = "Sin profesional"
		}
		lines = append(lines, l)
		key := "none"
		if l.ProfessionalID != nil {
			key = *l.ProfessionalID
		}
		i, ok := index[key]
		if !ok {
			i = len(totals)
			index[key] = i
			totals = append(totals, commissionTotal{ProfessionalID: l.ProfessionalID, Professional: l.Professional})
		}
		totals[i].Lines++
		totals[i].BaseCents += l.BaseCents
		totals[i].Cents += l.Cents
	}
	return from, to, totals, lines, 0, "", rows.Err()
}

func (s *Server) commissionReport(w http.ResponseWriter, r *http.Request) {
	from, to, totals, lines, status, msg, err := s.commissionData(r)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if msg != "" {
		writeError(w, status, msg)
		return
	}
	sum := 0
	for _, t := range totals {
		sum += t.Cents
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"from": from.Format("2006-01-02"), "to": to.AddDate(0, 0, -1).Format("2006-01-02"),
		"totals": totals, "lines": lines, "commission_cents": sum,
	})
}

func (s *Server) commissionCSV(w http.ResponseWriter, r *http.Request) {
	from, to, _, lines, status, msg, err := s.commissionData(r)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if msg != "" {
		writeError(w, status, msg)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="comisiones-`+from.Format("20060102")+`-`+to.AddDate(0, 0, -1).Format("20060102")+`.csv"`)
	_, _ = w.Write([]byte("\xEF\xBB\xBF"))
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"Folio", "Fecha", "Profesional", "Concepto", "Base sin IVA", "Porcentaje", "Comisión"})
	for _, l := range lines {
		_ = cw.Write([]string{strconv.Itoa(l.Folio), l.Date.Local().Format("2006-01-02 15:04"), csvSafe(l.Professional), csvSafe(l.Item),
			cents(l.BaseCents), strconv.FormatFloat(l.Percent, 'f', -1, 64), cents(l.Cents)})
	}
	cw.Flush()
}
