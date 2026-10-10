package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// exportPatient hands over everything the clinic holds about one person (ARCO requests and portability,
// LFPDPPP). Opening it is written to the access log and to the activity log. Only the metadata of the files
// is included, never the bytes or the server's storage keys.
func (s *Server) exportPatient(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	pat, err := loadPatient(r.Context(), s.db, p.ClinicID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	encs, err := s.visibleEncounters(r.Context(), p, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	withheld := 0
	for _, e := range encs {
		if e.Hidden {
			withheld++
		}
	}
	rxs, err := s.prescriptionsFor(r.Context(), p.ClinicID, id, true)
	if err != nil {
		serverError(w, r, err)
		return
	}
	certs, err := s.certificatesOf(r.Context(), p.ClinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	iss, err := s.issuerInfo(r.Context(), p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}

	// Each list is the table row itself as JSON, scoped by clinic and patient.
	sections := []struct{ key, sql string }{
		{"chronic_medications", `SELECT to_jsonb(t) FROM chronic_medications t WHERE clinic_id = $1 AND patient_id = $2 ORDER BY active DESC, name`},
		{"vaccinations", `SELECT to_jsonb(t) FROM vaccinations t WHERE clinic_id = $1 AND patient_id = $2 ORDER BY applied_on DESC, created_at DESC`},
		{"charts", `SELECT to_jsonb(t) FROM patient_charts t WHERE clinic_id = $1 AND patient_id = $2 ORDER BY created_at DESC`},
		{"treatment_plans", `SELECT to_jsonb(t) || jsonb_build_object(
				'items', coalesce((SELECT jsonb_agg(to_jsonb(i) ORDER BY i.phase, i.position) FROM treatment_plan_items i WHERE i.plan_id = t.id AND i.clinic_id = t.clinic_id), '[]'::jsonb),
				'events', coalesce((SELECT jsonb_agg(to_jsonb(e) ORDER BY e.id) FROM treatment_plan_events e WHERE e.plan_id = t.id AND e.clinic_id = t.clinic_id), '[]'::jsonb))
			FROM treatment_plans t WHERE t.clinic_id = $1 AND t.patient_id = $2 ORDER BY t.created_at DESC`},
		{"consents", `SELECT to_jsonb(t) - 'signature_png' || jsonb_build_object('has_signature', t.signature_png <> '')
			FROM consent_signatures t WHERE clinic_id = $1 AND patient_id = $2 ORDER BY signed_at DESC`},
		{"files", `SELECT to_jsonb(t) - 'storage_key' FROM attachments t WHERE clinic_id = $1 AND patient_id = $2 ORDER BY created_at DESC`},
		{"appointments", `SELECT to_jsonb(t) - 'confirm_token' FROM appointments t WHERE clinic_id = $1 AND patient_id = $2 ORDER BY date DESC, start_hour DESC`},
		{"access_log", `SELECT to_jsonb(t) - 'user_id' - 'clinic_id' FROM record_access t WHERE clinic_id = $1 AND patient_id = $2 ORDER BY created_at DESC`},
	}
	out := map[string]any{
		"format": "caresia-patient-export", "version": 1, "generated_at": time.Now().UTC(), "generated_by": p.actorName(),
		"clinic": iss, "patient": pat, "encounters": encs, "encounters_withheld": withheld, "prescriptions": rxs, "certificates": certs,
	}
	for _, sec := range sections {
		list, err := s.jsonRows(r.Context(), sec.sql, p.ClinicID, id)
		if err != nil {
			serverError(w, r, fmt.Errorf("export %s: %w", sec.key, err))
			return
		}
		if sec.key == "appointments" {
			if list, err = openJSONColumn(list, "appointments", "details"); err != nil {
				serverError(w, r, err)
				return
			}
		}
		out[sec.key] = list
	}

	s.logAccess(r.Context(), p.ClinicID, id, p, "export")
	audit(r.Context(), s.db, p.ClinicID, p, "patient_export", p.actorName()+" exportó el expediente de "+pat.Names+" "+pat.LastNames, map[string]any{"patientId": id})

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="expediente-%d.json"`, pat.FileNumber))
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) jsonRows(ctx context.Context, sql string, args ...any) ([]json.RawMessage, error) {
	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, rows.Err()
}

// openJSONColumn decrypts one text column of rows exported with to_jsonb(row) (the row's "id" is part of the
// authenticated data).
func openJSONColumn(list []json.RawMessage, table, column string) ([]json.RawMessage, error) {
	for i, raw := range list {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, err
		}
		id, _ := m["id"].(string)
		v, ok := m[column].(string)
		if !ok {
			continue
		}
		plain, err := decField(table, column, id, v)
		if err != nil {
			return nil, err
		}
		m[column] = plain
		b, err := json.Marshal(m)
		if err != nil {
			return nil, err
		}
		list[i] = b
	}
	return list, nil
}
