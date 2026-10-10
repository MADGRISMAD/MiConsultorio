package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Suscripción recurrente con Mercado Pago (preapproval), igual que en MiTiendita: Mercado Pago cobra solo cada mes o
// cada año a la tarjeta del consultorio. Cada cobro queda en payments y el plan vale hasta el siguiente cobro
// (next_payment_date). Los avisos de Mercado Pago aceleran la actualización, pero no hacen falta: la página de
// regreso, la de Suscripción y subscriptionPass le vuelven a preguntar a Mercado Pago.

const subRefPrefix = "caresia-sub:"

type mpPreapproval struct {
	ID                string     `json:"id"`
	Status            string     `json:"status"` // pending | authorized | paused | cancelled
	ExternalReference string     `json:"external_reference"`
	NextPaymentDate   *time.Time `json:"next_payment_date"`
	InitPoint         string     `json:"init_point"`
	SandboxInitPoint  string     `json:"sandbox_init_point"`
}

// startSubscription da de alta la suscripción y devuelve a dónde mandar al administrador para autorizarla.
func (s *Server) startSubscription(w http.ResponseWriter, r *http.Request, p *Principal, offer planOffer, period string, amount int) {
	if s.cfg.AppURL == "" {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Code: "NOT_CONFIGURED", Message: "Falta la dirección pública de Caresia (APP_URL) para volver después del pago."})
		return
	}
	if p.Email == "" {
		writeError(w, http.StatusConflict, "Tu usuario necesita un correo para suscribirse: agrégalo en Mi cuenta.")
		return
	}
	var id string
	if err := s.db.QueryRow(r.Context(), `
		INSERT INTO billing_checkouts (clinic_id, plan, period, amount_cents, created_by, kind) VALUES ($1,$2,$3,$4,$5,'subscription') RETURNING id`,
		p.ClinicID, offer.ID, period, amount, p.UserID).Scan(&id); err != nil {
		serverError(w, r, err)
		return
	}
	months, label := 1, "mensual"
	if period == "year" {
		months, label = 12, "anual"
	}
	recurring := map[string]any{"frequency": months, "frequency_type": "months", "transaction_amount": float64(amount) / 100, "currency_id": s.cfg.MPCurrency}
	// Si aún le quedan días de prueba, el primer cobro espera a que termine.
	if d := p.Billing.TrialDaysLeft(time.Now()); d != nil && *d > 0 {
		recurring["free_trial"] = map[string]any{"frequency": *d, "frequency_type": "days"}
	}
	body := map[string]any{
		"reason":             "Caresia · Plan " + offer.Name + " (" + label + ")",
		"external_reference": subRefPrefix + id,
		"payer_email":        p.Email,
		"auto_recurring":     recurring,
		"back_url":           s.cfg.AppURL + "/suscripcion?suscripcion=" + id,
		"status":             "pending",
	}
	if s.cfg.APIPublicURL != "" {
		body["notification_url"] = s.cfg.APIPublicURL + "/api/webhooks/mercadopago"
	}
	var pre mpPreapproval
	if err := s.mpCall(r.Context(), s.cfg.MPAccessToken, http.MethodPost, "/preapproval", body, &pre); err != nil {
		_, _ = s.db.Exec(r.Context(), `UPDATE billing_checkouts SET status='failed' WHERE id=$1`, id)
		providerFailure(w, r, err)
		return
	}
	pay := pre.InitPoint
	if s.cfg.MPSandbox && pre.SandboxInitPoint != "" {
		pay = pre.SandboxInitPoint
	}
	if _, err := s.db.Exec(r.Context(), `UPDATE billing_checkouts SET preapproval_id=$2, preapproval_status=$3, init_point=$4 WHERE id=$1`, id, pre.ID, pre.Status, pay); err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "subscription_started", "Inició la suscripción al plan "+offer.Name+" ("+label+")", map[string]any{"checkout": id})
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "init_point": pay, "subscription": true})
}

// syncSubscription lee la suscripción en Mercado Pago y deja la clínica igual: plan, estado y hasta cuándo vale.
// También anota los cobros que Mercado Pago ya hizo. Idempotente; ignora suscripciones que no son de Caresia.
func (s *Server) syncSubscription(ctx context.Context, preapprovalID string) error {
	if preapprovalID == "" || s.cfg.MPAccessToken == "" {
		return nil
	}
	var pre mpPreapproval
	if err := s.mpCall(ctx, s.cfg.MPAccessToken, http.MethodGet, "/preapproval/"+url.PathEscape(preapprovalID), nil, &pre); err != nil {
		var me *mpError
		if errors.As(err, &me) && (me.Status == 404 || me.Status == 401 || me.Status == 403) {
			return nil
		}
		return err
	}
	ref, ok := strings.CutPrefix(pre.ExternalReference, subRefPrefix)
	if !ok || !validUUID(ref) {
		return nil // de otro producto con la misma cuenta (por ejemplo MiTiendita)
	}
	var clinicID, plan, period, oldSub string
	var months int
	var notify func()
	err := inTx(ctx, s.db, func(tx pgx.Tx) error {
		var amount int
		var status string
		err := tx.QueryRow(ctx, `SELECT clinic_id, plan, period, amount_cents, status FROM billing_checkouts WHERE id=$1 AND kind='subscription' FOR UPDATE`, ref).
			Scan(&clinicID, &plan, &period, &amount, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		months = 1
		if period == "year" {
			months = 12
		}
		if _, err := tx.Exec(ctx, `UPDATE billing_checkouts SET preapproval_id=$2, preapproval_status=$3 WHERE id=$1`, ref, pre.ID, pre.Status); err != nil {
			return err
		}
		var current string
		var cur *time.Time
		if err := tx.QueryRow(ctx, `SELECT mp_preapproval_id, current_period_end FROM clinics WHERE id=$1 FOR UPDATE`, clinicID).Scan(&current, &cur); err != nil {
			return err
		}
		switch pre.Status {
		case "authorized":
			end := time.Now().AddDate(0, months, 0)
			if pre.NextPaymentDate != nil {
				end = *pre.NextPaymentDate // vale hasta el siguiente cobro (con prueba: hasta que termina la prueba)
			}
			if cur != nil && cur.After(end) {
				end = *cur // lo ya pagado (por ejemplo un pago único anterior) no se pierde
			}
			if _, err := tx.Exec(ctx, `
				UPDATE clinics SET plan=$2, billing_status='active', current_period_end=$3, suspended_at=NULL, suspended_reason='',
				       mp_preapproval_id=$4, cancel_at_period_end=false, updated_at=now()
				WHERE id=$1`, clinicID, plan, end, pre.ID); err != nil {
				return err
			}
			if status != "paid" {
				if _, err := tx.Exec(ctx, `UPDATE billing_checkouts SET status='paid', paid_at=now() WHERE id=$1`, ref); err != nil {
					return err
				}
				audit(ctx, tx, clinicID, nil, "subscription_active", "Suscripción activa: plan "+plan+", se cobra sola cada "+itoa(months)+" mes(es)", map[string]any{"preapproval": pre.ID})
				if current != "" && current != pre.ID {
					oldSub = current // cambio de plan: la anterior se cancela al confirmar la nueva
				}
			}
		case "paused":
			if current == pre.ID {
				if _, err := tx.Exec(ctx, `UPDATE clinics SET billing_status='past_due', updated_at=now() WHERE id=$1`, clinicID); err != nil {
					return err
				}
			}
		case "cancelled":
			if current == pre.ID {
				// Conserva el acceso hasta el fin del periodo pagado; después el estado pasa solo a vencido.
				if _, err := tx.Exec(ctx, `UPDATE clinics SET cancel_at_period_end=true, updated_at=now() WHERE id=$1`, clinicID); err != nil {
					return err
				}
			}
			if status == "pending" {
				if _, err := tx.Exec(ctx, `UPDATE billing_checkouts SET status='failed' WHERE id=$1`, ref); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if oldSub != "" {
		if err := s.mpCall(ctx, s.cfg.MPAccessToken, http.MethodPut, "/preapproval/"+url.PathEscape(oldSub), map[string]any{"status": "cancelled"}, nil); err != nil {
			log.Printf("subscription: no se pudo cancelar la anterior %s: %v", oldSub, err)
		}
	}
	if clinicID != "" {
		notify = s.recordSubscriptionCharges(ctx, clinicID, plan, months, pre.ID)
	}
	if notify != nil {
		notify()
	}
	return nil
}

// recordSubscriptionCharges anota en payments cada cobro ya hecho de la suscripción (una vez cada uno).
// Devuelve el envío del recibo del cobro más reciente que se anotó por primera vez.
func (s *Server) recordSubscriptionCharges(ctx context.Context, clinicID, plan string, months int, preapprovalID string) func() {
	var found struct {
		Results []struct {
			ID                int64   `json:"id"`
			Status            string  `json:"status"`
			TransactionAmount float64 `json:"transaction_amount"`
			Payment           struct {
				Status string `json:"status"`
			} `json:"payment"`
		} `json:"results"`
	}
	if err := s.mpCall(ctx, s.cfg.MPAccessToken, http.MethodGet, "/authorized_payments/search?preapproval_id="+url.QueryEscape(preapprovalID), nil, &found); err != nil {
		log.Printf("subscription: no se pudieron leer los cobros de %s: %v", preapprovalID, err)
		return nil
	}
	var end time.Time
	if err := s.db.QueryRow(ctx, `SELECT coalesce(current_period_end, now()) FROM clinics WHERE id=$1`, clinicID).Scan(&end); err != nil {
		return nil
	}
	var notify func()
	for _, c := range found.Results {
		if c.Status != "processed" || (c.Payment.Status != "" && c.Payment.Status != "approved") {
			continue
		}
		amount := int(c.TransactionAmount*100 + 0.5)
		tag, err := s.db.Exec(ctx, `
			INSERT INTO payments (clinic_id, amount_cents, plan, months, note, period_end, provider, provider_ref)
			VALUES ($1,$2,$3,$4,'Cobro de la suscripción (Mercado Pago)',$5,'mercadopago',$6) ON CONFLICT DO NOTHING`,
			clinicID, amount, plan, months, end, "ap:"+strconv.FormatInt(c.ID, 10))
		if err != nil || tag.RowsAffected() == 0 {
			continue
		}
		audit(ctx, s.db, clinicID, nil, "payment_online", "Cobro de la suscripción: plan "+plan+", $"+cents(amount), map[string]any{"authorized_payment": c.ID})
		planName := plan
		if pl, ok := planByID(plan); ok {
			planName = pl.Name
		}
		a := amount
		notify = func() { s.paymentReceivedEmail(ctx, clinicID, planName, months, a, end) }
	}
	return notify
}

// syncFromAuthorizedPayment atiende el aviso de un cobro: busca su suscripción y la sincroniza.
func (s *Server) syncFromAuthorizedPayment(ctx context.Context, id string) error {
	var ap struct {
		PreapprovalID string `json:"preapproval_id"`
	}
	if err := s.mpCall(ctx, s.cfg.MPAccessToken, http.MethodGet, "/authorized_payments/"+url.PathEscape(id), nil, &ap); err != nil {
		var me *mpError
		if errors.As(err, &me) && (me.Status == 404 || me.Status == 401 || me.Status == 403) {
			return nil
		}
		return err
	}
	return s.syncSubscription(ctx, ap.PreapprovalID)
}

// syncCheckoutSubscription busca en Mercado Pago la suscripción de un checkout (la página de regreso la usa
// cuando el aviso aún no llega).
func (s *Server) syncCheckoutSubscription(ctx context.Context, checkoutID string) {
	var found struct {
		Results []struct {
			ID string `json:"id"`
		} `json:"results"`
	}
	if err := s.mpCall(ctx, s.cfg.MPAccessToken, http.MethodGet, "/preapproval/search?external_reference="+url.QueryEscape(subRefPrefix+checkoutID), nil, &found); err != nil {
		log.Printf("subscription: búsqueda de %s: %v", checkoutID, err)
		return
	}
	for _, p := range found.Results {
		if err := s.syncSubscription(ctx, p.ID); err != nil {
			log.Printf("subscription: sync %s: %v", p.ID, err)
		}
	}
}

// cancelSubscription cancela la suscripción vigente; el plan sigue hasta el fin del periodo ya pagado.
func (s *Server) cancelSubscription(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	var id string
	if err := s.db.QueryRow(r.Context(), `SELECT mp_preapproval_id FROM clinics WHERE id=$1`, p.ClinicID).Scan(&id); err != nil {
		serverError(w, r, err)
		return
	}
	if id == "" {
		writeError(w, http.StatusNotFound, "No tienes una suscripción activa.")
		return
	}
	if err := s.mpCall(r.Context(), s.cfg.MPAccessToken, http.MethodPut, "/preapproval/"+url.PathEscape(id), map[string]any{"status": "cancelled"}, nil); err != nil {
		providerFailure(w, r, err)
		return
	}
	if err := s.syncSubscription(r.Context(), id); err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "subscription_cancelled", "Canceló la suscripción; el plan sigue hasta el fin del periodo pagado", map[string]any{"preapproval": id})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// subscriptionPass vuelve a preguntar por todas las suscripciones vigentes (por si se perdió algún aviso).
func (s *Server) subscriptionPass(ctx context.Context) {
	if s.cfg.MPAccessToken == "" {
		return
	}
	rows, err := s.db.Query(ctx, `SELECT DISTINCT mp_preapproval_id FROM clinics WHERE mp_preapproval_id <> ''`)
	if err != nil {
		log.Printf("subscriptions: %v", err)
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		if err := s.syncSubscription(ctx, id); err != nil {
			log.Printf("subscriptions: %s: %v", id, err)
		}
	}
}

// subscriptionInfo es lo que ve el administrador de su suscripción.
type subscriptionInfo struct {
	Active            bool   `json:"active"`
	Status            string `json:"status"`
	Period            string `json:"period"`
	AmountCents       int    `json:"amount_cents"`
	CancelAtPeriodEnd bool   `json:"cancel_at_period_end"`
}

func (s *Server) subscriptionFor(ctx context.Context, clinicID string) subscriptionInfo {
	var si subscriptionInfo
	_ = s.db.QueryRow(ctx, `
		SELECT c.mp_preapproval_id <> '', c.cancel_at_period_end, coalesce(b.preapproval_status, ''), coalesce(b.period, ''), coalesce(b.amount_cents, 0)
		FROM clinics c LEFT JOIN billing_checkouts b ON b.preapproval_id = c.mp_preapproval_id AND c.mp_preapproval_id <> ''
		WHERE c.id = $1`, clinicID).Scan(&si.Active, &si.CancelAtPeriodEnd, &si.Status, &si.Period, &si.AmountCents)
	return si
}
