package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type cashMovement struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	AmountCents int       `json:"amount_cents"`
	Concept     string    `json:"concept"`
	By          string    `json:"by"`
	CreatedAt   time.Time `json:"created_at"`
}

type methodTotal struct {
	Method      string `json:"method"`
	AmountCents int    `json:"amount_cents"`
	Count       int    `json:"count"`
}

type cashSession struct {
	ID            string     `json:"id"`
	OpenedBy      string     `json:"opened_by"`
	OpenedAt      time.Time  `json:"opened_at"`
	OpeningCents  int        `json:"opening_cents"`
	ClosedBy      string     `json:"closed_by"`
	ClosedAt      *time.Time `json:"closed_at"`
	CountedCents  *int       `json:"counted_cents"`
	ExpectedCents int        `json:"expected_cents"`
	DiffCents     *int       `json:"diff_cents"`
	Note          string     `json:"note"`
	// computed
	Sales       int            `json:"sales"`
	SalesCents  int            `json:"sales_cents"`
	CashInCents int            `json:"cash_in_cents"` // cash payments net of change
	MovesIn     int            `json:"moves_in_cents"`
	MovesOut    int            `json:"moves_out_cents"`
	ByMethod    []methodTotal  `json:"by_method"`
	Movements   []cashMovement `json:"movements"`
}

// summarizeSession computes what the register should hold. Voided sales are left out. Money counts in the
// register that was open when it was received, so abonos land in today's register, not the sale's.
func summarizeSession(ctx context.Context, q interface {
	queryRower
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, clinicID, id string) (cashSession, error) {
	var c cashSession
	var expected *int
	err := q.QueryRow(ctx, `
		SELECT id, opened_by_name, opened_at, opening_cents, closed_by_name, closed_at, counted_cents, expected_cents, note
		FROM cash_sessions WHERE clinic_id = $1 AND id = $2`, clinicID, id).
		Scan(&c.ID, &c.OpenedBy, &c.OpenedAt, &c.OpeningCents, &c.ClosedBy, &c.ClosedAt, &c.CountedCents, &expected, &c.Note)
	if err != nil {
		return c, err
	}
	if err := q.QueryRow(ctx, `SELECT count(*), coalesce(sum(total_cents),0) FROM sales WHERE session_id = $1 AND status IN ('paid', 'open')`, id).Scan(&c.Sales, &c.SalesCents); err != nil {
		return c, err
	}
	rows, err := q.Query(ctx, `
		SELECT sp.method, coalesce(sum(sp.amount_cents),0), count(*)
		FROM sale_payments sp JOIN sales s ON s.id = sp.sale_id
		WHERE sp.session_id = $1 AND s.status <> 'void' GROUP BY sp.method ORDER BY sp.method`, id)
	if err != nil {
		return c, err
	}
	c.ByMethod = []methodTotal{}
	for rows.Next() {
		var m methodTotal
		if err := rows.Scan(&m.Method, &m.AmountCents, &m.Count); err != nil {
			rows.Close()
			return c, err
		}
		if m.Method == "cash" {
			c.CashInCents = m.AmountCents
		}
		c.ByMethod = append(c.ByMethod, m)
	}
	rows.Close()
	mrows, err := q.Query(ctx, `SELECT id, kind, amount_cents, concept, created_by_name, created_at FROM cash_movements WHERE session_id = $1 ORDER BY created_at`, id)
	if err != nil {
		return c, err
	}
	c.Movements = []cashMovement{}
	for mrows.Next() {
		var m cashMovement
		if err := mrows.Scan(&m.ID, &m.Kind, &m.AmountCents, &m.Concept, &m.By, &m.CreatedAt); err != nil {
			mrows.Close()
			return c, err
		}
		if m.Kind == "in" {
			c.MovesIn += m.AmountCents
		} else {
			c.MovesOut += m.AmountCents
		}
		c.Movements = append(c.Movements, m)
	}
	mrows.Close()
	c.ExpectedCents = c.OpeningCents + c.CashInCents + c.MovesIn - c.MovesOut
	if c.ClosedAt != nil && expected != nil {
		c.ExpectedCents = *expected // frozen at closing time
	}
	if c.CountedCents != nil {
		d := *c.CountedCents - c.ExpectedCents
		c.DiffCents = &d
	}
	return c, nil
}

func (s *Server) currentCash(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	var id string
	err := s.db.QueryRow(r.Context(), `SELECT id FROM cash_sessions WHERE clinic_id = $1 AND closed_at IS NULL`, p.ClinicID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusOK, map[string]any{"session": nil})
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	c, err := summarizeSession(r.Context(), s.db, p.ClinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"session": c})
}

func (s *Server) openCash(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OpeningCents int `json:"opening_cents"`
	}
	if !decode(w, r, &req) {
		return
	}
	if !okCents(req.OpeningCents) {
		writeError(w, http.StatusBadRequest, "El fondo inicial no es válido.")
		return
	}
	p := principalFrom(r.Context())
	var id string
	err := s.db.QueryRow(r.Context(), `
		INSERT INTO cash_sessions (clinic_id, opened_by, opened_by_name, opening_cents) VALUES ($1,$2,$3,$4) RETURNING id`,
		p.ClinicID, p.UserID, p.actorName(), req.OpeningCents).Scan(&id)
	if isUniqueViolation(err) {
		writeError(w, http.StatusConflict, "Ya hay una caja abierta.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "cash_open", "Abrió la caja con $"+cents(req.OpeningCents), nil)
	c, err := summarizeSession(r.Context(), s.db, p.ClinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"session": c})
}

func (s *Server) cashMovementCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind        string `json:"kind"`
		AmountCents int    `json:"amount_cents"`
		Concept     string `json:"concept"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.Concept = strings.TrimSpace(req.Concept)
	switch {
	case req.Kind != "in" && req.Kind != "out":
		writeError(w, http.StatusBadRequest, "Indica si es entrada o salida.")
		return
	case req.AmountCents <= 0 || !okCents(req.AmountCents):
		writeError(w, http.StatusBadRequest, "El monto no es válido.")
		return
	case req.Concept == "" || utf8.RuneCountInString(req.Concept) > 120:
		writeError(w, http.StatusBadRequest, "Escribe el concepto (máximo 120 caracteres).")
		return
	}
	p := principalFrom(r.Context())
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		var sid string
		err := tx.QueryRow(r.Context(), `SELECT id FROM cash_sessions WHERE clinic_id = $1 AND closed_at IS NULL FOR UPDATE`, p.ClinicID).Scan(&sid)
		if errors.Is(err, pgx.ErrNoRows) {
			return &httpError{Status: http.StatusConflict, Code: "CASH_CLOSED", Msg: "Abre la caja primero."}
		}
		if err != nil {
			return err
		}
		if req.Kind == "out" {
			c, err := summarizeSession(r.Context(), tx, p.ClinicID, sid)
			if err != nil {
				return err
			}
			if req.AmountCents > c.ExpectedCents {
				return fail(http.StatusConflict, "No hay tanto efectivo en la caja (hay $"+cents(c.ExpectedCents)+").")
			}
		}
		if _, err := tx.Exec(r.Context(), `
			INSERT INTO cash_movements (clinic_id, session_id, kind, amount_cents, concept, created_by, created_by_name) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			p.ClinicID, sid, req.Kind, req.AmountCents, req.Concept, p.UserID, p.actorName()); err != nil {
			return err
		}
		label := map[string]string{"in": "Entrada", "out": "Salida"}[req.Kind]
		audit(r.Context(), tx, p.ClinicID, p, "cash_movement", label+" de caja por $"+cents(req.AmountCents)+": "+req.Concept, nil)
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

func (s *Server) closeCash(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CountedCents int    `json:"counted_cents"`
		Note         string `json:"note"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.Note = strings.TrimSpace(req.Note)
	if !okCents(req.CountedCents) {
		writeError(w, http.StatusBadRequest, "El efectivo contado no es válido.")
		return
	}
	if utf8.RuneCountInString(req.Note) > 300 {
		writeError(w, http.StatusBadRequest, "La nota es demasiado larga.")
		return
	}
	p := principalFrom(r.Context())
	var out cashSession
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		var sid string
		err := tx.QueryRow(r.Context(), `SELECT id FROM cash_sessions WHERE clinic_id = $1 AND closed_at IS NULL FOR UPDATE`, p.ClinicID).Scan(&sid)
		if errors.Is(err, pgx.ErrNoRows) {
			return fail(http.StatusConflict, "No hay una caja abierta.")
		}
		if err != nil {
			return err
		}
		c, err := summarizeSession(r.Context(), tx, p.ClinicID, sid)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `
			UPDATE cash_sessions SET closed_at = now(), closed_by = $3, closed_by_name = $4, counted_cents = $5, expected_cents = $6, note = $7
			WHERE clinic_id = $1 AND id = $2`, p.ClinicID, sid, p.UserID, p.actorName(), req.CountedCents, c.ExpectedCents, req.Note); err != nil {
			return err
		}
		out, err = summarizeSession(r.Context(), tx, p.ClinicID, sid)
		if err != nil {
			return err
		}
		audit(r.Context(), tx, p.ClinicID, p, "cash_close", "Cerró la caja: esperado $"+cents(c.ExpectedCents)+", contado $"+cents(req.CountedCents), nil)
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"session": out})
}

func (s *Server) listCashSessions(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(), `
		SELECT id FROM cash_sessions WHERE clinic_id = $1 AND closed_at IS NOT NULL ORDER BY opened_at DESC LIMIT 60`, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		serverError(w, r, err)
		return
	}
	list := make([]cashSession, 0, len(ids))
	for _, id := range ids {
		c, err := summarizeSession(r.Context(), s.db, p.ClinicID, id)
		if err != nil {
			serverError(w, r, err)
			return
		}
		c.Movements = nil
		list = append(list, c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": list})
}

func (s *Server) getCashSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Corte no encontrado.")
		return
	}
	c, err := summarizeSession(r.Context(), s.db, principalFrom(r.Context()).ClinicID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Corte no encontrado.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"session": c})
}
