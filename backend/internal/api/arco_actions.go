package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

// arcoMutate runs fn on the locked request of the caller's clinic and answers with the updated request.
// fn returns a message to refuse with 409.
func (s *Server) arcoMutate(w http.ResponseWriter, r *http.Request, fn func(ctx context.Context, tx pgx.Tx, a *arcoRequest, p *Principal) (string, error)) {
	ctx := r.Context()
	p := principalFrom(ctx)
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Solicitud no encontrada.")
		return
	}
	var refused string
	err := inTx(ctx, s.db, func(tx pgx.Tx) error {
		a, err := arcoScan(tx.QueryRow(ctx, `SELECT `+arcoCols+arcoFrom+` WHERE r.clinic_id = $1 AND r.id = $2 FOR UPDATE OF r`, p.ClinicID, id))
		if err != nil {
			return err
		}
		a.derive(s.arcoToday(ctx, p.ClinicID))
		if refused, err = fn(ctx, tx, &a, p); err != nil || refused != "" {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE arco_requests SET updated_at = now() WHERE id = $1`, id)
		return err
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		writeError(w, http.StatusNotFound, "Solicitud no encontrada.")
	case err != nil:
		serverError(w, r, err)
	case refused != "":
		writeJSON(w, http.StatusConflict, errorBody{Code: "NOT_ALLOWED", Message: refused})
	default:
		a, err := s.arcoLoad(ctx, p.ClinicID, id)
		if err != nil {
			serverError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"request": a})
	}
}

// arcoMailRequester sends a plain notice to the person who made the request. It never fails the action.
func (s *Server) arcoMailRequester(to, clinicName, subject, body string) {
	if to == "" {
		return
	}
	s.sendMail(mail.Message{
		To:      []string{to},
		Subject: subject,
		Text:    body + "\n\n" + clinicName + "\n",
		HTML:    layout(subject, "<p>"+strings.ReplaceAll(esc(body), "\n", "<br>")+"</p><p><strong>"+esc(clinicName)+"</strong></p>"),
	})
}

func (s *Server) arcoClinicName(ctx context.Context, clinicID string) string {
	var n string
	_ = s.db.QueryRow(ctx, `SELECT name FROM clinics WHERE id = $1`, clinicID).Scan(&n)
	return n
}

func (s *Server) arcoPatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PatientID        *string `json:"patient_id"`
		IdentityVerified *bool   `json:"identity_verified"`
		IdentityMethod   *string `json:"identity_method"`
		RequesterName    *string `json:"requester_name"`
		RequesterEmail   *string `json:"requester_email"`
		RequesterPhone   *string `json:"requester_phone"`
		HandledByName    *string `json:"handled_by_name"`
	}
	if !decode(w, r, &req) {
		return
	}
	s.arcoMutate(w, r, func(ctx context.Context, tx pgx.Tx, a *arcoRequest, p *Principal) (string, error) {
		fail := func(m string) (string, error) { return m, nil }
		if req.PatientID != nil && *req.PatientID != "" && !arcoPatientOK(ctx, tx, p.ClinicID, *req.PatientID) {
			return fail("Paciente no encontrado.")
		}
		if req.RequesterName != nil {
			v, ok := arcoCleanText(arcoOneLine(*req.RequesterName), 3, 120)
			if !ok {
				return fail("El nombre debe tener entre 3 y 120 caracteres.")
			}
			a.RequesterName = v
		}
		if req.RequesterEmail != nil {
			v := strings.TrimSpace(*req.RequesterEmail)
			if v != "" && !validEmail(v) {
				return fail("El correo no es válido.")
			}
			a.RequesterEmail = v
		}
		if req.RequesterPhone != nil {
			v := arcoOneLine(*req.RequesterPhone)
			if len(v) > 30 {
				return fail("El teléfono es demasiado largo.")
			}
			a.RequesterPhone = v
		}
		if req.HandledByName != nil {
			a.HandledByName = truncate(arcoOneLine(*req.HandledByName), 120)
		}
		method := a.IdentityMethod
		if req.IdentityMethod != nil {
			method = truncate(arcoOneLine(*req.IdentityMethod), 200)
		}
		verified := a.IdentityVerified
		if req.IdentityVerified != nil {
			verified = *req.IdentityVerified
		}
		if verified && method == "" {
			return fail("Indica cómo se verificó la identidad (por ejemplo: identificación oficial mostrada en el consultorio).")
		}
		if _, err := tx.Exec(ctx, `UPDATE arco_requests SET requester_name = $3, requester_email = $4, requester_phone = $5, handled_by_name = $6,
			identity_method = $7, identity_verified = $8,
			identity_verified_at = CASE WHEN $8 AND NOT identity_verified THEN now() WHEN NOT $8 THEN NULL ELSE identity_verified_at END
			WHERE clinic_id = $1 AND id = $2`,
			p.ClinicID, a.ID, a.RequesterName, a.RequesterEmail, a.RequesterPhone, a.HandledByName, method, verified); err != nil {
			return "", err
		}
		if verified != a.IdentityVerified || (verified && method != a.IdentityMethod) {
			msg := "Identidad no verificada"
			if verified {
				msg = "Identidad verificada: " + method
			}
			if err := arcoEvent(ctx, tx, p.ClinicID, a.ID, "identity", p.actorName(), msg); err != nil {
				return "", err
			}
			audit(ctx, tx, p.ClinicID, p, "arco_identity", msg+" ("+a.Folio+")", map[string]any{"arco_id": a.ID})
		}
		cur := ""
		if a.PatientID != nil {
			cur = *a.PatientID
		}
		if req.PatientID != nil && *req.PatientID != cur {
			var target any
			msg := "Se desvinculó el paciente"
			if *req.PatientID != "" {
				target, msg = *req.PatientID, "Se vinculó el expediente de un paciente"
			}
			if _, err := tx.Exec(ctx, `UPDATE arco_requests SET patient_id = $3 WHERE clinic_id = $1 AND id = $2`, p.ClinicID, a.ID, target); err != nil {
				return "", err
			}
			if err := arcoEvent(ctx, tx, p.ClinicID, a.ID, "link", p.actorName(), msg); err != nil {
				return "", err
			}
		}
		return "", nil
	})
}

func (s *Server) arcoSetStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Status    string `json:"status"`
		Note      string `json:"note"`
		SendEmail bool   `json:"send_email"`
	}
	if !decode(w, r, &req) {
		return
	}
	note, noteOK := arcoCleanText(req.Note, 0, 1000)
	if req.Status != "recibida" && req.Status != "en_revision" && req.Status != "requiere_info" {
		writeError(w, http.StatusBadRequest, "Estado no válido. Para cerrar la solicitud usa «Responder».")
		return
	}
	if !noteOK || (req.Status == "requiere_info" && note == "") {
		writeError(w, http.StatusBadRequest, "Escribe qué información o documento se necesita (máximo 1000 caracteres).")
		return
	}
	s.arcoMutate(w, r, func(ctx context.Context, tx pgx.Tx, a *arcoRequest, p *Principal) (string, error) {
		if !a.Open {
			return "La solicitud ya está cerrada.", nil
		}
		if _, err := tx.Exec(ctx, `UPDATE arco_requests SET status = $3 WHERE clinic_id = $1 AND id = $2`, p.ClinicID, a.ID, req.Status); err != nil {
			return "", err
		}
		msg := "Estado: " + arcoStatusLabels[req.Status]
		if note != "" {
			msg += ". " + note
		}
		if err := arcoEvent(ctx, tx, p.ClinicID, a.ID, "status", p.actorName(), msg); err != nil {
			return "", err
		}
		audit(ctx, tx, p.ClinicID, p, "arco_status", a.Folio+": "+arcoStatusLabels[req.Status], map[string]any{"arco_id": a.ID})
		if req.SendEmail && req.Status == "requiere_info" {
			s.arcoMailRequester(a.RequesterEmail, s.arcoClinicName(ctx, p.ClinicID), "Necesitamos más información de tu solicitud "+a.Folio,
				"Para atender tu solicitud "+a.Folio+" necesitamos lo siguiente:\n\n"+note)
		}
		return "", nil
	})
}

func (s *Server) arcoNote(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Note string `json:"note"`
	}
	if !decode(w, r, &req) {
		return
	}
	note, ok := arcoCleanText(req.Note, 2, 2000)
	if !ok {
		writeError(w, http.StatusBadRequest, "La nota debe tener entre 2 y 2000 caracteres.")
		return
	}
	s.arcoMutate(w, r, func(ctx context.Context, tx pgx.Tx, a *arcoRequest, p *Principal) (string, error) {
		return "", arcoEvent(ctx, tx, p.ClinicID, a.ID, "note", p.actorName(), note)
	})
}

func (s *Server) arcoRespond(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Outcome      string `json:"outcome"` // procedente | improcedente
		ResponseText string `json:"response_text"`
		DenialReason string `json:"denial_reason"`
		Executed     bool   `json:"executed"`   // the request was already carried out with this answer
		SendEmail    bool   `json:"send_email"` // e-mail the response to the requester
	}
	if !decode(w, r, &req) {
		return
	}
	text, okText := arcoCleanText(req.ResponseText, 10, 5000)
	reason, okReason := arcoCleanText(req.DenialReason, 10, 2000)
	switch {
	case req.Outcome != "procedente" && req.Outcome != "improcedente":
		writeError(w, http.StatusBadRequest, "Indica si la solicitud es procedente o no.")
		return
	case !okText:
		writeError(w, http.StatusBadRequest, "Escribe la respuesta para quien solicita (entre 10 y 5000 caracteres).")
		return
	case req.Outcome == "improcedente" && !okReason:
		writeError(w, http.StatusBadRequest, "Explica el motivo de la negativa (entre 10 y 2000 caracteres).")
		return
	}
	s.arcoMutate(w, r, func(ctx context.Context, tx pgx.Tx, a *arcoRequest, p *Principal) (string, error) {
		if !a.Open {
			return "La solicitud ya tiene una determinación.", nil
		}
		if !a.IdentityVerified {
			return "Verifica y registra la identidad de quien solicita antes de dar una respuesta.", nil
		}
		loc := clinicLocation(ctx, tx, p.ClinicID)
		now := time.Now()
		if req.Outcome == "procedente" {
			var exec any
			var executedAt any
			if req.Executed {
				executedAt = now
			} else {
				exec = arcoAddBusinessDays(arcoDay(now, loc), arcoExecuteDays)
			}
			if _, err := tx.Exec(ctx, `UPDATE arco_requests SET status = 'atendida', resolution = 'procedente', answered_at = $3, response_text = $4,
				due_execute_at = $5, executed_at = $6, denial_reason = '', handled_by_name = $7 WHERE clinic_id = $1 AND id = $2`,
				p.ClinicID, a.ID, now, text, exec, executedAt, p.actorName()); err != nil {
				return "", err
			}
		} else {
			if _, err := tx.Exec(ctx, `UPDATE arco_requests SET status = 'negada', resolution = 'improcedente', answered_at = $3, response_text = $4,
				denial_reason = $5, handled_by_name = $6 WHERE clinic_id = $1 AND id = $2`,
				p.ClinicID, a.ID, now, text, reason, p.actorName()); err != nil {
				return "", err
			}
		}
		msg := "Determinación comunicada: " + req.Outcome
		if req.Outcome == "procedente" && req.Executed {
			msg += " (ya ejecutada)"
		}
		if err := arcoEvent(ctx, tx, p.ClinicID, a.ID, "response", p.actorName(), msg); err != nil {
			return "", err
		}
		audit(ctx, tx, p.ClinicID, p, "arco_response", a.Folio+": "+msg, map[string]any{"arco_id": a.ID})
		if req.SendEmail {
			s.arcoMailRequester(a.RequesterEmail, s.arcoClinicName(ctx, p.ClinicID), "Respuesta a tu solicitud "+a.Folio, text)
		}
		return "", nil
	})
}

func (s *Server) arcoExecute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Note string `json:"note"`
	}
	if r.ContentLength != 0 && !decode(w, r, &req) {
		return
	}
	note, ok := arcoCleanText(req.Note, 0, 1000)
	if !ok {
		writeError(w, http.StatusBadRequest, "La nota es demasiado larga.")
		return
	}
	s.arcoMutate(w, r, func(ctx context.Context, tx pgx.Tx, a *arcoRequest, p *Principal) (string, error) {
		if !a.PendingExecute {
			return "Esta solicitud no tiene una ejecución pendiente.", nil
		}
		if _, err := tx.Exec(ctx, `UPDATE arco_requests SET executed_at = now() WHERE clinic_id = $1 AND id = $2`, p.ClinicID, a.ID); err != nil {
			return "", err
		}
		msg := "Se hizo efectiva la solicitud"
		if note != "" {
			msg += ". " + note
		}
		if err := arcoEvent(ctx, tx, p.ClinicID, a.ID, "executed", p.actorName(), msg); err != nil {
			return "", err
		}
		audit(ctx, tx, p.ClinicID, p, "arco_executed", a.Folio+": ejecutada", map[string]any{"arco_id": a.ID})
		return "", nil
	})
}

// arcoArchivePatient blocks the use of the linked record the way the rest of the system does: it archives it.
// Nothing is deleted: NOM-004-SSA3-2012 requires keeping the expediente for at least 5 years.
func (s *Server) arcoArchivePatient(w http.ResponseWriter, r *http.Request) {
	s.arcoMutate(w, r, func(ctx context.Context, tx pgx.Tx, a *arcoRequest, p *Principal) (string, error) {
		if a.Kind != "cancelacion" && a.Kind != "oposicion" && a.Kind != "revocacion" {
			return "Solo las solicitudes de cancelación, oposición o revocación bloquean el uso del expediente.", nil
		}
		if a.PatientID == nil {
			return "Vincula primero el expediente del paciente.", nil
		}
		if !a.IdentityVerified {
			return "Verifica y registra la identidad de quien solicita antes de bloquear el expediente.", nil
		}
		reason := "Solicitud ARCO " + a.Folio + " (" + arcoKindLabels[a.Kind] + "): uso bloqueado, expediente conservado por NOM-004"
		t, err := tx.Exec(ctx, `UPDATE patients SET archived_at = now(), archived_by = $3, archive_reason = $4, updated_at = now()
			WHERE clinic_id = $1 AND id = $2 AND archived_at IS NULL`, p.ClinicID, *a.PatientID, p.actorName(), truncate(reason, 200))
		if err != nil {
			return "", err
		}
		if t.RowsAffected() == 0 {
			return "El expediente ya está archivado.", nil
		}
		if err := arcoEvent(ctx, tx, p.ClinicID, a.ID, "archive", p.actorName(), "Expediente archivado: se bloqueó su uso y se conserva (NOM-004)"); err != nil {
			return "", err
		}
		audit(ctx, tx, p.ClinicID, p, "patient_archived", "Archivó un expediente: "+reason, map[string]any{"patient": *a.PatientID, "arco_id": a.ID})
		return "", nil
	})
}
