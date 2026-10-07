package api

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Laboratory orders and structured results. Results are append-only: a wrong value is corrected by capturing a new
// one that points at it (supersedes_id); the old one stays in the history. Orders are cancelled with a reason, never
// deleted. Free text is sealed at rest like the rest of the clinical record (see encfields.go).

type labResult struct {
	ID            string    `json:"id"`
	OrderID       string    `json:"order_id"`
	Panel         string    `json:"panel"`
	Analyte       string    `json:"analyte"`
	ValueNum      *float64  `json:"value_num"`
	ValueText     string    `json:"value_text"`
	Unit          string    `json:"unit"`
	RefLow        *float64  `json:"ref_low"`
	RefHigh       *float64  `json:"ref_high"`
	RefSource     string    `json:"ref_source"`
	Flag          string    `json:"flag"`
	ResultedAt    time.Time `json:"resulted_at"`
	Notes         string    `json:"notes"`
	SupersedesID  *string   `json:"supersedes_id"`
	SupersededBy  *string   `json:"superseded_by"`
	CreatedByName string    `json:"created_by_name"`
	CreatedAt     time.Time `json:"created_at"`
}

type labOrder struct {
	ID            string      `json:"id"`
	PatientID     string      `json:"patient_id"`
	EncounterID   *string     `json:"encounter_id"`
	Title         string      `json:"title"`
	Status        string      `json:"status"`
	OrderedAt     time.Time   `json:"ordered_at"`
	OrderedByName string      `json:"ordered_by_name"`
	LabName       string      `json:"lab_name"`
	Notes         string      `json:"notes"`
	AttachmentID  *string     `json:"attachment_id"`
	CancelReason  string      `json:"cancel_reason"`
	CancelledAt   *time.Time  `json:"cancelled_at"`
	CancelledBy   string      `json:"cancelled_by"`
	CreatedAt     time.Time   `json:"created_at"`
	Results       []labResult `json:"results"`
}

const labOrderCols = `id::text, patient_id::text, encounter_id::text, title, status, ordered_at, ordered_by_name, lab_name, notes,
	attachment_id::text, cancel_reason, cancelled_at, cancelled_by, created_at`

const labResultCols = `r.id::text, r.order_id::text, r.panel, r.analyte, r.value_num::float8, r.value_text, r.unit, r.ref_low::float8, r.ref_high::float8,
	r.ref_source, r.flag, r.resulted_at, r.notes, r.supersedes_id::text, (SELECT s.id::text FROM lab_results s WHERE s.supersedes_id = r.id),
	r.created_by_name, r.created_at`

func scanLabOrder(row pgx.Row) (labOrder, error) {
	var o labOrder
	err := row.Scan(&o.ID, &o.PatientID, &o.EncounterID, &o.Title, &o.Status, &o.OrderedAt, &o.OrderedByName, &o.LabName, &o.Notes,
		&o.AttachmentID, &o.CancelReason, &o.CancelledAt, &o.CancelledBy, &o.CreatedAt)
	if err != nil {
		return o, err
	}
	if o.Notes, err = decField("lab_orders", "notes", o.ID, o.Notes); err != nil {
		return o, err
	}
	o.Results = []labResult{}
	return o, nil
}

func scanLabResult(row pgx.Row) (labResult, error) {
	var x labResult
	err := row.Scan(&x.ID, &x.OrderID, &x.Panel, &x.Analyte, &x.ValueNum, &x.ValueText, &x.Unit, &x.RefLow, &x.RefHigh, &x.RefSource, &x.Flag,
		&x.ResultedAt, &x.Notes, &x.SupersedesID, &x.SupersededBy, &x.CreatedByName, &x.CreatedAt)
	if err != nil {
		return x, err
	}
	x.Notes, err = decField("lab_results", "notes", x.ID, x.Notes)
	return x, err
}

// labFlag compares a numeric value with the range printed for it. Without any bound there is nothing to compare.
func labFlag(value float64, low, high *float64) string {
	switch {
	case low == nil && high == nil:
		return "na"
	case low != nil && value < *low:
		return "bajo"
	case high != nil && value > *high:
		return "alto"
	}
	return "normal"
}

type labResultIn struct {
	Panel        string     `json:"panel"`
	Analyte      string     `json:"analyte"`
	ValueNum     *float64   `json:"value_num"`
	ValueText    string     `json:"value_text"`
	Unit         string     `json:"unit"`
	RefLow       *float64   `json:"ref_low"`
	RefHigh      *float64   `json:"ref_high"`
	RefSource    string     `json:"ref_source"`
	Flag         string     `json:"flag"`
	ResultedAt   *time.Time `json:"resulted_at"`
	Notes        string     `json:"notes"`
	SupersedesID string     `json:"supersedes_id"`
}

type labOrderIn struct {
	Title        string        `json:"title"`
	EncounterID  string        `json:"encounter_id"`
	OrderedAt    *time.Time    `json:"ordered_at"`
	LabName      string        `json:"lab_name"`
	Notes        string        `json:"notes"`
	AttachmentID string        `json:"attachment_id"`
	Results      []labResultIn `json:"results"`
	Complete     bool          `json:"complete"`
}

func labTooLong(s string, n int) bool { return utf8.RuneCountInString(s) > n }

func labFinite(v *float64) bool {
	return v == nil || (!math.IsNaN(*v) && !math.IsInf(*v, 0) && math.Abs(*v) < 1e12)
}

// labPatient loads the patient of this clinic the route is about, answering 404 itself.
func (s *Server) labPatient(w http.ResponseWriter, r *http.Request) (id, sex string, archived, ok bool) {
	id = chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	var at *time.Time
	err := s.db.QueryRow(r.Context(), `SELECT sex, archived_at FROM patients WHERE clinic_id=$1 AND id=$2`, p.ClinicID, id).Scan(&sex, &at)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	return id, sex, at != nil, true
}

func (s *Server) labCatalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"version": labCatalogVersion, "notice": labCatalogNotice, "panels": labCatalogPanels})
}

func (s *Server) labLoadResults(ctx context.Context, q rowsQuerier, clinicID, where string, args ...any) ([]labResult, error) {
	rows, err := q.Query(ctx, `SELECT `+labResultCols+` FROM lab_results r WHERE r.clinic_id=$1 AND `+where+` ORDER BY r.resulted_at ASC, r.created_at ASC, r.analyte`,
		append([]any{clinicID}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []labResult{}
	for rows.Next() {
		x, err := scanLabResult(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Server) listLabOrders(w http.ResponseWriter, r *http.Request) {
	id, _, _, ok := s.labPatient(w, r)
	if !ok {
		return
	}
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(), `SELECT `+labOrderCols+` FROM lab_orders WHERE clinic_id=$1 AND patient_id=$2 ORDER BY ordered_at DESC, created_at DESC LIMIT 300`, p.ClinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	orders := []labOrder{}
	idx := map[string]int{}
	for rows.Next() {
		o, err := scanLabOrder(rows)
		if err != nil {
			rows.Close()
			serverError(w, r, err)
			return
		}
		idx[o.ID] = len(orders)
		orders = append(orders, o)
	}
	rows.Close()
	if rows.Err() != nil {
		serverError(w, r, rows.Err())
		return
	}
	results, err := s.labLoadResults(r.Context(), s.db, p.ClinicID, `r.patient_id=$2`, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	for _, x := range results {
		if i, found := idx[x.OrderID]; found {
			orders[i].Results = append(orders[i].Results, x)
		}
	}
	s.logAccess(r.Context(), p.ClinicID, id, p, "view")
	writeJSON(w, http.StatusOK, map[string]any{"orders": orders})
}

func (s *Server) labOrderWithResults(ctx context.Context, q rowsQuerier, clinicID, id string) (labOrder, error) {
	o, err := scanLabOrder(q.QueryRow(ctx, `SELECT `+labOrderCols+` FROM lab_orders WHERE clinic_id=$1 AND id=$2`, clinicID, id))
	if err != nil {
		return o, err
	}
	o.Results, err = s.labLoadResults(ctx, q, clinicID, `r.order_id=$2`, id)
	return o, err
}

func (s *Server) getLabOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p := principalFrom(r.Context())
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Orden no encontrada.")
		return
	}
	o, err := s.labOrderWithResults(r.Context(), s.db, p.ClinicID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Orden no encontrada.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	s.logAccess(r.Context(), p.ClinicID, o.PatientID, p, "view")
	writeJSON(w, http.StatusOK, map[string]any{"order": o})
}

// labCheckLinks verifies that an encounter and an attachment belong to this patient and clinic.
func (s *Server) labCheckLinks(ctx context.Context, q rowsQuerier, clinicID, patientID, encounterID, attachmentID string) error {
	if encounterID != "" {
		var found bool
		if !validUUID(encounterID) {
			return fail(http.StatusBadRequest, "La consulta indicada no es válida.")
		}
		if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM encounters WHERE clinic_id=$1 AND patient_id=$2 AND id=$3)`, clinicID, patientID, encounterID).Scan(&found); err != nil {
			return err
		}
		if !found {
			return fail(http.StatusBadRequest, "La consulta indicada no pertenece a este paciente.")
		}
	}
	if attachmentID != "" {
		var found bool
		if !validUUID(attachmentID) {
			return fail(http.StatusBadRequest, "El archivo indicado no es válido.")
		}
		if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM attachments WHERE clinic_id=$1 AND patient_id=$2 AND id=$3)`, clinicID, patientID, attachmentID).Scan(&found); err != nil {
			return err
		}
		if !found {
			return fail(http.StatusBadRequest, "El archivo indicado no pertenece a este paciente.")
		}
	}
	return nil
}

func labCleanOrder(in *labOrderIn) string {
	in.Title, in.LabName, in.Notes = strings.TrimSpace(in.Title), strings.TrimSpace(in.LabName), strings.TrimSpace(in.Notes)
	switch {
	case in.Title == "" || labTooLong(in.Title, 160):
		return "Escribe el nombre del estudio (máximo 160 caracteres)."
	case labTooLong(in.LabName, 120):
		return "El nombre del laboratorio es demasiado largo."
	case labTooLong(in.Notes, 1000):
		return "Las notas son demasiado largas."
	}
	return ""
}

func (s *Server) createLabOrder(w http.ResponseWriter, r *http.Request) {
	id, sex, archived, ok := s.labPatient(w, r)
	if !ok {
		return
	}
	var in labOrderIn
	if !decode(w, r, &in) {
		return
	}
	if msg := labCleanOrder(&in); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if archived {
		writeError(w, http.StatusConflict, "El expediente está archivado: no se pueden agregar órdenes.")
		return
	}
	ordered := time.Now()
	if in.OrderedAt != nil {
		ordered = *in.OrderedAt
		if ordered.After(time.Now().Add(24*time.Hour)) || ordered.Year() < 1900 {
			writeError(w, http.StatusBadRequest, "La fecha de la orden no es válida.")
			return
		}
	}
	p := principalFrom(r.Context())
	var out labOrder
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		if err := s.labCheckLinks(r.Context(), tx, p.ClinicID, id, in.EncounterID, in.AttachmentID); err != nil {
			return err
		}
		orderID := newRowID()
		notes, err := encField("lab_orders", "notes", orderID, in.Notes)
		if err != nil {
			return err
		}
		var enc, att any
		if in.EncounterID != "" {
			enc = in.EncounterID
		}
		if in.AttachmentID != "" {
			att = in.AttachmentID
		}
		if _, err := tx.Exec(r.Context(), `INSERT INTO lab_orders (id, clinic_id, patient_id, encounter_id, title, ordered_at, ordered_by_name, lab_name, notes, attachment_id)
			VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, orderID, p.ClinicID, id, enc, in.Title, ordered, p.actorName(), in.LabName, notes, att); err != nil {
			return err
		}
		if len(in.Results) > 0 {
			if err := s.labInsertResults(r.Context(), tx, p, orderID, id, sex, in.Results); err != nil {
				return err
			}
			status := "parcial"
			if in.Complete {
				status = "completo"
			}
			if _, err := tx.Exec(r.Context(), `UPDATE lab_orders SET status=$3, updated_at=now() WHERE clinic_id=$1 AND id=$2`, p.ClinicID, orderID, status); err != nil {
				return err
			}
		}
		out, err = s.labOrderWithResults(r.Context(), tx, p.ClinicID, orderID)
		return err
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "lab_order_create", "Registró una orden de laboratorio", map[string]any{"patient": id, "order": out.ID, "results": len(out.Results)})
	writeJSON(w, http.StatusCreated, map[string]any{"order": out})
}

// labLoadOrderForWrite finds an order of this clinic that can still receive changes.
func (s *Server) labOrderForWrite(w http.ResponseWriter, r *http.Request) (o labOrder, sex string, ok bool) {
	id := chi.URLParam(r, "id")
	p := principalFrom(r.Context())
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Orden no encontrada.")
		return
	}
	o, err := scanLabOrder(s.db.QueryRow(r.Context(), `SELECT `+labOrderCols+` FROM lab_orders WHERE clinic_id=$1 AND id=$2`, p.ClinicID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Orden no encontrada.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	var archived *time.Time
	if err := s.db.QueryRow(r.Context(), `SELECT sex, archived_at FROM patients WHERE clinic_id=$1 AND id=$2`, p.ClinicID, o.PatientID).Scan(&sex, &archived); err != nil {
		serverError(w, r, err)
		return
	}
	if o.Status == "cancelado" {
		writeError(w, http.StatusConflict, "La orden está cancelada y ya no admite cambios.")
		return
	}
	if archived != nil {
		writeError(w, http.StatusConflict, "El expediente está archivado: no se pueden modificar órdenes.")
		return
	}
	return o, sex, true
}

func (s *Server) updateLabOrder(w http.ResponseWriter, r *http.Request) {
	var in labOrderIn
	if !decode(w, r, &in) {
		return
	}
	if msg := labCleanOrder(&in); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	o, _, ok := s.labOrderForWrite(w, r)
	if !ok {
		return
	}
	p := principalFrom(r.Context())
	if err := s.labCheckLinks(r.Context(), s.db, p.ClinicID, o.PatientID, in.EncounterID, in.AttachmentID); err != nil {
		writeFailure(w, r, err)
		return
	}
	notes, err := encField("lab_orders", "notes", o.ID, in.Notes)
	if err != nil {
		serverError(w, r, err)
		return
	}
	var att any
	if in.AttachmentID != "" {
		att = in.AttachmentID
	}
	if _, err := s.db.Exec(r.Context(), `UPDATE lab_orders SET title=$3, lab_name=$4, notes=$5, attachment_id=$6, updated_at=now() WHERE clinic_id=$1 AND id=$2`,
		p.ClinicID, o.ID, in.Title, in.LabName, notes, att); err != nil {
		serverError(w, r, err)
		return
	}
	out, err := s.labOrderWithResults(r.Context(), s.db, p.ClinicID, o.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "lab_order_update", "Editó los datos de una orden de laboratorio", map[string]any{"patient": o.PatientID, "order": o.ID})
	writeJSON(w, http.StatusOK, map[string]any{"order": out})
}

func (s *Server) setLabOrderStatus(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if in.Status != "parcial" && in.Status != "completo" && in.Status != "cancelado" {
		writeError(w, http.StatusBadRequest, "El estado no es válido.")
		return
	}
	if in.Status == "cancelado" && (in.Reason == "" || labTooLong(in.Reason, 300)) {
		writeError(w, http.StatusBadRequest, "Escribe el motivo de la cancelación.")
		return
	}
	o, _, ok := s.labOrderForWrite(w, r)
	if !ok {
		return
	}
	p := principalFrom(r.Context())
	switch {
	case in.Status == o.Status:
		writeError(w, http.StatusConflict, "La orden ya tiene ese estado.")
		return
	case in.Status == "parcial" && o.Status != "solicitado":
		writeError(w, http.StatusConflict, "Una orden con resultados no puede volver a parcial.")
		return
	}
	if in.Status != "cancelado" {
		var n int
		if err := s.db.QueryRow(r.Context(), `SELECT count(*) FROM lab_results WHERE clinic_id=$1 AND order_id=$2`, p.ClinicID, o.ID).Scan(&n); err != nil {
			serverError(w, r, err)
			return
		}
		if n == 0 {
			writeError(w, http.StatusConflict, "Captura al menos un resultado antes de cambiar el estado.")
			return
		}
	}
	var err error
	if in.Status == "cancelado" {
		_, err = s.db.Exec(r.Context(), `UPDATE lab_orders SET status='cancelado', cancel_reason=$3, cancelled_at=now(), cancelled_by=$4, updated_at=now() WHERE clinic_id=$1 AND id=$2`,
			p.ClinicID, o.ID, in.Reason, p.actorName())
	} else {
		_, err = s.db.Exec(r.Context(), `UPDATE lab_orders SET status=$3, updated_at=now() WHERE clinic_id=$1 AND id=$2`, p.ClinicID, o.ID, in.Status)
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	out, err := s.labOrderWithResults(r.Context(), s.db, p.ClinicID, o.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "lab_order_status", "Cambió el estado de una orden de laboratorio a "+in.Status, map[string]any{"patient": o.PatientID, "order": o.ID, "status": in.Status})
	writeJSON(w, http.StatusOK, map[string]any{"order": out})
}

func (s *Server) addLabResults(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Results  []labResultIn `json:"results"`
		Complete bool          `json:"complete"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Results) == 0 {
		writeError(w, http.StatusBadRequest, "Captura al menos un resultado.")
		return
	}
	o, sex, ok := s.labOrderForWrite(w, r)
	if !ok {
		return
	}
	p := principalFrom(r.Context())
	var out labOrder
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		if err := s.labInsertResults(r.Context(), tx, p, o.ID, o.PatientID, sex, in.Results); err != nil {
			return err
		}
		status := o.Status
		if o.Status == "solicitado" {
			status = "parcial"
		}
		if in.Complete {
			status = "completo"
		}
		if _, err := tx.Exec(r.Context(), `UPDATE lab_orders SET status=$3, updated_at=now() WHERE clinic_id=$1 AND id=$2`, p.ClinicID, o.ID, status); err != nil {
			return err
		}
		var err error
		out, err = s.labOrderWithResults(r.Context(), tx, p.ClinicID, o.ID)
		return err
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "lab_results_add", "Capturó resultados de laboratorio", map[string]any{"patient": o.PatientID, "order": o.ID, "count": len(in.Results)})
	writeJSON(w, http.StatusCreated, map[string]any{"order": out})
}

// labInsertResults validates and appends results to an order, computing every flag on the server.
func (s *Server) labInsertResults(ctx context.Context, tx pgx.Tx, p *Principal, orderID, patientID, sex string, list []labResultIn) error {
	if len(list) > 120 {
		return fail(http.StatusBadRequest, "Máximo 120 resultados por captura.")
	}
	now := time.Now()
	for _, in := range list {
		in.Panel, in.Analyte, in.Unit = strings.TrimSpace(in.Panel), strings.TrimSpace(in.Analyte), strings.TrimSpace(in.Unit)
		in.ValueText, in.Notes = strings.TrimSpace(in.ValueText), strings.TrimSpace(in.Notes)
		switch {
		case in.Analyte == "" || labTooLong(in.Analyte, 120):
			return fail(http.StatusBadRequest, "Cada resultado necesita el nombre del análisis (máximo 120 caracteres).")
		case labTooLong(in.Panel, 120) || labTooLong(in.Unit, 30) || labTooLong(in.ValueText, 200) || labTooLong(in.Notes, 500):
			return fail(http.StatusBadRequest, "Alguno de los textos de «"+in.Analyte+"» es demasiado largo.")
		case !labFinite(in.ValueNum) || !labFinite(in.RefLow) || !labFinite(in.RefHigh):
			return fail(http.StatusBadRequest, "Los valores de «"+in.Analyte+"» no son válidos.")
		case in.ValueNum == nil && in.ValueText == "":
			return fail(http.StatusBadRequest, "Captura el valor de «"+in.Analyte+"».")
		case in.RefLow != nil && in.RefHigh != nil && *in.RefLow > *in.RefHigh:
			return fail(http.StatusBadRequest, "El rango de referencia de «"+in.Analyte+"» está invertido.")
		case in.RefSource != "" && in.RefSource != "laboratorio" && in.RefSource != "catalogo":
			return fail(http.StatusBadRequest, "El origen del rango de «"+in.Analyte+"» no es válido.")
		}
		resulted := now
		if in.ResultedAt != nil {
			resulted = *in.ResultedAt
			if resulted.After(now.Add(24*time.Hour)) || resulted.Year() < 1900 {
				return fail(http.StatusBadRequest, "La fecha del resultado de «"+in.Analyte+"» no es válida.")
			}
		}
		// The range typed on the result always prevails; the catalog only fills a missing one.
		low, high, source := in.RefLow, in.RefHigh, in.RefSource
		if low != nil || high != nil {
			if source == "" {
				source = "laboratorio"
			}
		} else if in.ValueNum != nil {
			if cl, ch, unit, found := labCatalogRef(in.Panel, in.Analyte, sex); found && (cl != nil || ch != nil) {
				low, high, source = cl, ch, "catalogo"
				if in.Unit == "" {
					in.Unit = unit
				}
			} else {
				source = ""
			}
		} else {
			source = ""
		}
		flag := "na"
		if in.ValueNum != nil {
			flag = labFlag(*in.ValueNum, low, high)
			if in.Flag == "critico" && (flag == "bajo" || flag == "alto") {
				flag = "critico"
			}
		} else if in.Flag == "normal" || in.Flag == "anormal" || in.Flag == "critico" {
			flag = in.Flag
		}

		var supersedes any
		if in.SupersedesID != "" {
			if !validUUID(in.SupersedesID) {
				return fail(http.StatusBadRequest, "El resultado a corregir no es válido.")
			}
			var oid, panel, analyte string
			err := tx.QueryRow(ctx, `SELECT order_id::text, panel, analyte FROM lab_results WHERE clinic_id=$1 AND id=$2`, p.ClinicID, in.SupersedesID).Scan(&oid, &panel, &analyte)
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && (oid != orderID || !strings.EqualFold(analyte, in.Analyte))) {
				return fail(http.StatusBadRequest, "El resultado a corregir no existe en esta orden o es de otro análisis.")
			}
			if err != nil {
				return err
			}
			if in.Notes == "" {
				return fail(http.StatusBadRequest, "Escribe el motivo de la corrección de «"+in.Analyte+"».")
			}
			supersedes = in.SupersedesID
		}

		id := newRowID()
		notes, err := encField("lab_results", "notes", id, in.Notes)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO lab_results (id, clinic_id, patient_id, order_id, panel, analyte, value_num, value_text, unit, ref_low, ref_high, ref_source,
				flag, resulted_at, notes, supersedes_id, created_by_name)
			VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
			id, p.ClinicID, patientID, orderID, in.Panel, in.Analyte, in.ValueNum, in.ValueText, in.Unit, low, high, source, flag, resulted, notes, supersedes, p.actorName()); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return fail(http.StatusConflict, "«"+in.Analyte+"» ya fue corregido: corrige la versión más reciente.")
			}
			return err
		}
	}
	return nil
}

type labTrendPoint struct {
	ID         string    `json:"id"`
	OrderID    string    `json:"order_id"`
	ResultedAt time.Time `json:"resulted_at"`
	Value      float64   `json:"value"`
	Unit       string    `json:"unit"`
	RefLow     *float64  `json:"ref_low"`
	RefHigh    *float64  `json:"ref_high"`
	RefSource  string    `json:"ref_source"`
	Flag       string    `json:"flag"`
	LabName    string    `json:"lab_name"`
}

type labAnalyteSummary struct {
	Analyte string    `json:"analyte"`
	Unit    string    `json:"unit"`
	Count   int       `json:"count"`
	LastAt  time.Time `json:"last_at"`
}

// labTrends lists the numeric history of one analyte (current versions of non-cancelled orders), plus the analytes that have any.
func (s *Server) labTrends(w http.ResponseWriter, r *http.Request) {
	id, _, _, ok := s.labPatient(w, r)
	if !ok {
		return
	}
	p := principalFrom(r.Context())
	const current = `r.clinic_id=$1 AND r.patient_id=$2 AND r.value_num IS NOT NULL AND o.status <> 'cancelado'
		AND NOT EXISTS (SELECT 1 FROM lab_results s WHERE s.supersedes_id = r.id)`
	rows, err := s.db.Query(r.Context(), `SELECT min(r.analyte), coalesce((array_agg(r.unit ORDER BY r.resulted_at DESC))[1], ''), count(*)::int, max(r.resulted_at)
		FROM lab_results r JOIN lab_orders o ON o.id = r.order_id WHERE `+current+` GROUP BY lower(r.analyte) ORDER BY lower(min(r.analyte)) LIMIT 500`, p.ClinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	analytes := []labAnalyteSummary{}
	for rows.Next() {
		var a labAnalyteSummary
		if err := rows.Scan(&a.Analyte, &a.Unit, &a.Count, &a.LastAt); err != nil {
			rows.Close()
			serverError(w, r, err)
			return
		}
		analytes = append(analytes, a)
	}
	rows.Close()
	if rows.Err() != nil {
		serverError(w, r, rows.Err())
		return
	}
	analyte := strings.TrimSpace(r.URL.Query().Get("analyte"))
	points := []labTrendPoint{}
	if analyte != "" {
		prow, err := s.db.Query(r.Context(), `SELECT r.id::text, r.order_id::text, r.resulted_at, r.value_num::float8, r.unit, r.ref_low::float8, r.ref_high::float8, r.ref_source, r.flag, o.lab_name
			FROM lab_results r JOIN lab_orders o ON o.id = r.order_id WHERE `+current+` AND lower(r.analyte) = lower($3)
			ORDER BY r.resulted_at ASC, r.created_at ASC LIMIT 1000`, p.ClinicID, id, analyte)
		if err != nil {
			serverError(w, r, err)
			return
		}
		defer prow.Close()
		for prow.Next() {
			var x labTrendPoint
			if err := prow.Scan(&x.ID, &x.OrderID, &x.ResultedAt, &x.Value, &x.Unit, &x.RefLow, &x.RefHigh, &x.RefSource, &x.Flag, &x.LabName); err != nil {
				serverError(w, r, err)
				return
			}
			points = append(points, x)
		}
		if prow.Err() != nil {
			serverError(w, r, prow.Err())
			return
		}
	}
	s.logAccess(r.Context(), p.ClinicID, id, p, "view")
	writeJSON(w, http.StatusOK, map[string]any{"analyte": analyte, "points": points, "analytes": analytes})
}
