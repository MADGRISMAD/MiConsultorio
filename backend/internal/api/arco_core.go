package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var arcoKindLabels = map[string]string{
	"acceso":        "Acceso",
	"rectificacion": "Rectificación",
	"cancelacion":   "Cancelación",
	"oposicion":     "Oposición",
	"revocacion":    "Revocación del consentimiento",
}

var arcoStatusLabels = map[string]string{
	"recibida":      "Recibida",
	"en_revision":   "En revisión",
	"requiere_info": "Requiere información",
	"atendida":      "Atendida",
	"negada":        "Negada",
	"vencida":       "Vencida",
}

func arcoValidKind(k string) bool { _, ok := arcoKindLabels[k]; return ok }

// arcoRequest is one ARCO request as staff see it. The fields after Open are derived from today's date.
type arcoRequest struct {
	ID                 string     `json:"id"`
	Folio              string     `json:"folio"`
	Kind               string     `json:"kind"`
	PatientID          *string    `json:"patient_id"`
	PatientName        string     `json:"patient_name"`
	RequesterName      string     `json:"requester_name"`
	RequesterEmail     string     `json:"requester_email"`
	RequesterPhone     string     `json:"requester_phone"`
	IdentityVerified   bool       `json:"identity_verified"`
	IdentityMethod     string     `json:"identity_method"`
	IdentityVerifiedAt *time.Time `json:"identity_verified_at"`
	Description        string     `json:"description"`
	Status             string     `json:"status"`
	Resolution         string     `json:"resolution"`
	ReceivedAt         time.Time  `json:"received_at"`
	DueAckAt           string     `json:"due_ack_at"`
	DueAnswerAt        string     `json:"due_answer_at"`
	DueExecuteAt       *string    `json:"due_execute_at"`
	AnsweredAt         *time.Time `json:"answered_at"`
	ExecutedAt         *time.Time `json:"executed_at"`
	ResponseText       string     `json:"response_text"`
	DenialReason       string     `json:"denial_reason"`
	HandledByName      string     `json:"handled_by_name"`
	CreatedVia         string     `json:"created_via"`

	Open           bool   `json:"open"`            // still waiting for the determination
	PendingExecute bool   `json:"pending_execute"` // procedente, not carried out yet
	Deadline       string `json:"deadline"`        // answer | execute | "" (nothing pending)
	DaysLeft       *int   `json:"days_left"`       // business days to the pending deadline (negative: overdue)
	DeadlineState  string `json:"deadline_state"`  // ok | soon | overdue | ""

	dueAnswer, dueExecute time.Time
}

const arcoCols = `r.id, r.folio, r.kind, r.patient_id, coalesce(btrim(p.names || ' ' || p.last_names), ''),
	r.requester_name, r.requester_email, r.requester_phone, r.identity_verified, r.identity_method, r.identity_verified_at,
	r.description, r.status, r.resolution, r.received_at, r.due_ack_at, r.due_answer_at, r.due_execute_at,
	r.answered_at, r.executed_at, r.response_text, r.denial_reason, r.handled_by_name, r.created_via`

const arcoFrom = ` FROM arco_requests r LEFT JOIN patients p ON p.id = r.patient_id AND p.clinic_id = r.clinic_id `

func arcoScan(row pgx.Row) (arcoRequest, error) {
	var a arcoRequest
	var ack, ans time.Time
	var exe *time.Time
	err := row.Scan(&a.ID, &a.Folio, &a.Kind, &a.PatientID, &a.PatientName,
		&a.RequesterName, &a.RequesterEmail, &a.RequesterPhone, &a.IdentityVerified, &a.IdentityMethod, &a.IdentityVerifiedAt,
		&a.Description, &a.Status, &a.Resolution, &a.ReceivedAt, &ack, &ans, &exe,
		&a.AnsweredAt, &a.ExecutedAt, &a.ResponseText, &a.DenialReason, &a.HandledByName, &a.CreatedVia)
	if err != nil {
		return a, err
	}
	a.DueAckAt, a.DueAnswerAt, a.dueAnswer = ack.Format("2006-01-02"), ans.Format("2006-01-02"), ans
	if exe != nil {
		s := exe.Format("2006-01-02")
		a.DueExecuteAt, a.dueExecute = &s, *exe
	}
	return a, nil
}

// derive fills the fields that depend on today (a date at midnight UTC, see arcoDay).
func (a *arcoRequest) derive(today time.Time) {
	a.Open = a.Status == "recibida" || a.Status == "en_revision" || a.Status == "requiere_info"
	a.PendingExecute = a.Status == "atendida" && a.Resolution == "procedente" && a.ExecutedAt == nil
	a.Deadline, a.DeadlineState, a.DaysLeft = "", "", nil
	var due time.Time
	switch {
	case a.Open:
		a.Deadline, due = "answer", a.dueAnswer
	case a.PendingExecute && a.DueExecuteAt != nil:
		a.Deadline, due = "execute", a.dueExecute
	default:
		return
	}
	left := arcoBusinessDaysLeft(today, due)
	a.DaysLeft, a.DeadlineState = &left, arcoState(left)
}

// arcoSummary counts what needs attention in a clinic.
type arcoSummary struct {
	Open           int `json:"open"`            // waiting for a determination or for its execution
	Overdue        int `json:"overdue"`         // past a deadline
	Soon           int `json:"soon"`            // deadline within a few business days
	PendingExecute int `json:"pending_execute"` // procedente, not yet carried out
}

func arcoSummarize(list []arcoRequest) arcoSummary {
	var s arcoSummary
	for _, a := range list {
		if !a.Open && !a.PendingExecute {
			continue
		}
		s.Open++
		if a.PendingExecute {
			s.PendingExecute++
		}
		switch a.DeadlineState {
		case "overdue":
			s.Overdue++
		case "soon":
			s.Soon++
		}
	}
	return s
}

// arcoLoadPending returns the requests of a clinic that still have a deadline running.
func arcoLoadPending(ctx context.Context, q rowsQuerier, clinicID string) ([]arcoRequest, error) {
	rows, err := q.Query(ctx, `SELECT `+arcoCols+arcoFrom+`
		WHERE r.clinic_id = $1 AND (r.status IN ('recibida', 'en_revision', 'requiere_info')
		   OR (r.status = 'atendida' AND r.resolution = 'procedente' AND r.executed_at IS NULL))`, clinicID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	today := arcoDay(time.Now(), clinicLocation(ctx, q, clinicID))
	var out []arcoRequest
	for rows.Next() {
		a, err := arcoScan(rows)
		if err != nil {
			return nil, err
		}
		a.derive(today)
		out = append(out, a)
	}
	return out, rows.Err()
}

// arcoNew is what a new request needs.
type arcoNew struct {
	Kind, Name, Email, Phone, Description string
	Via, HandledBy                        string
	PatientID                             string
	Received                              time.Time // zero: now
	Actor                                 string
}

// arcoCreate stores a request with its deadlines and its first history line. It returns the id, the folio
// and the follow-up token (only its hash is stored).
func arcoCreate(ctx context.Context, tx pgx.Tx, clinicID string, in arcoNew, loc *time.Location) (id, folio, token string, err error) {
	if token, err = newToken(); err != nil {
		return
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('arco:' || $1))`, clinicID); err != nil {
		return
	}
	var seq int
	if err = tx.QueryRow(ctx, `SELECT coalesce(max(seq), 0) + 1 FROM arco_requests WHERE clinic_id = $1`, clinicID).Scan(&seq); err != nil {
		return
	}
	received := in.Received
	if received.IsZero() {
		received = time.Now()
	}
	due := arcoDeadlines(arcoDay(received, loc))
	folio = fmt.Sprintf("ARCO-%d-%04d", received.In(loc).Year(), seq)
	var patient any
	if in.PatientID != "" {
		patient = in.PatientID
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO arco_requests (clinic_id, seq, folio, kind, patient_id, requester_name, requester_email, requester_phone, description,
		                           received_at, due_ack_at, due_answer_at, handled_by_name, created_via, token_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15) RETURNING id`,
		clinicID, seq, folio, in.Kind, patient, in.Name, in.Email, in.Phone, in.Description,
		received, due.Ack, due.Answer, in.HandledBy, in.Via, hashToken(token)).Scan(&id)
	if err != nil {
		return
	}
	msg := "Solicitud recibida (" + arcoKindLabels[in.Kind] + ")"
	if in.Via == "public" {
		msg += " desde el formulario público"
	}
	err = arcoEvent(ctx, tx, clinicID, id, "received", in.Actor, msg)
	return
}

func arcoEvent(ctx context.Context, q execer, clinicID, requestID, kind, actor, message string) error {
	_, err := q.Exec(ctx, `INSERT INTO arco_events (clinic_id, request_id, kind, actor_name, message) VALUES ($1, $2, $3, $4, $5)`,
		clinicID, requestID, kind, actor, message)
	return err
}

// arcoCleanText trims and bounds free text (in runes). It returns false when it is out of range.
func arcoCleanText(s string, min, max int) (string, bool) {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
	n := len([]rune(s))
	return s, n >= min && n <= max
}

// arcoOneLine strips line breaks for fields shown in a single line.
func arcoOneLine(s string) string {
	return strings.TrimSpace(strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(s))
}
