package api

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// ---------------------------------------------------------------------------
// Token encryption (provider tokens at rest)
// ---------------------------------------------------------------------------

func (s *Server) seal(plain string) ([]byte, error) {
	block, err := aes.NewCipher(s.cfg.TokenEncKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, []byte(plain), nil), nil
}

func (s *Server) open(blob []byte) (string, error) {
	block, err := aes.NewCipher(s.cfg.TokenEncKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(blob) < gcm.NonceSize() {
		return "", errors.New("token ilegible")
	}
	plain, err := gcm.Open(nil, blob[:gcm.NonceSize()], blob[gcm.NonceSize():], nil)
	return string(plain), err
}

// ---------------------------------------------------------------------------
// HTTP client
// ---------------------------------------------------------------------------

var providerHTTP = &http.Client{Timeout: 20 * time.Second}

// mpCall talks to Mercado Pago with a bearer token. A 4xx/5xx becomes an error carrying MP's message.
func (s *Server) mpCall(ctx context.Context, token, method, path string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.cfg.MPAPIBase+path, rdr)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	if method == http.MethodPost {
		key := make([]byte, 16)
		_, _ = rand.Read(key)
		req.Header.Set("X-Idempotency-Key", hex.EncodeToString(key))
	}
	res, err := providerHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("mercado pago: %w", err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		var e struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		_ = json.Unmarshal(data, &e)
		msg := e.Message
		if msg == "" {
			msg = e.Error
		}
		return &mpError{Status: res.StatusCode, Msg: msg}
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

type mpError struct {
	Status int
	Msg    string
}

func (e *mpError) Error() string { return "mercado pago (" + strconv.Itoa(e.Status) + "): " + e.Msg }

// providerFailure turns a provider error into a response the user can act on.
func providerFailure(w http.ResponseWriter, r *http.Request, err error) {
	var me *mpError
	if errors.As(err, &me) && (me.Status == 401 || me.Status == 403) {
		writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER_AUTH", Message: "Mercado Pago rechazó las credenciales. Vuelve a conectar la cuenta."})
		return
	}
	logf(r, "provider error: %v", err)
	writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER", Message: "No pudimos hablar con Mercado Pago. Intenta de nuevo en un momento."})
}

// ---------------------------------------------------------------------------
// Status
// ---------------------------------------------------------------------------

type providerStatus struct {
	MPConfigured   bool   `json:"mp_configured"`   // platform can charge subscriptions
	PointAvailable bool   `json:"point_available"` // server has OAuth credentials to connect clinics
	PointConnected bool   `json:"point_connected"`
	PointAccount   string `json:"point_account,omitempty"`
	MagicAvailable bool   `json:"magic_available"`
	MPPublicKey    string `json:"mp_public_key,omitempty"`
	Sandbox        bool   `json:"sandbox"`
}

func (s *Server) providerStatus(ctx context.Context, clinicID string) providerStatus {
	st := providerStatus{
		MPConfigured:   s.cfg.MPAccessToken != "",
		PointAvailable: s.cfg.MPClientID != "" && s.cfg.MPClientSecret != "" && s.oauthRedirect() != "",
		MagicAvailable: s.cfg.GeminiAPIKey != "",
		MPPublicKey:    s.cfg.MPPublicKey, Sandbox: s.cfg.MPSandbox,
	}
	var user string
	if err := s.db.QueryRow(ctx, `SELECT mp_user_id FROM mp_accounts WHERE clinic_id = $1`, clinicID).Scan(&user); err == nil {
		st.PointConnected, st.PointAccount = true, user
	}
	return st
}

func (s *Server) oauthRedirect() string {
	if s.cfg.MPOAuthRedirect != "" {
		return s.cfg.MPOAuthRedirect
	}
	if s.cfg.APIPublicURL != "" {
		return s.cfg.APIPublicURL + "/api/point/oauth/callback"
	}
	return ""
}

// ---------------------------------------------------------------------------
// Subscription checkout (the platform charges the clinic)
// ---------------------------------------------------------------------------

type planOffer struct {
	Plan
	MonthCents int  `json:"month_cents"`
	YearCents  int  `json:"year_cents"`
	Online     bool `json:"online"` // can be paid online
}

func (s *Server) offers() []planOffer {
	out := make([]planOffer, 0, len(planCatalog))
	for _, p := range planCatalog {
		o := planOffer{Plan: p}
		month, year := p.PriceMonth, p.PriceMonth*10 // a year costs 10 months
		if v, ok := s.cfg.PlanPriceMonth[p.ID]; ok {
			month = v
			year = v * 10
		}
		if v, ok := s.cfg.PlanPriceYear[p.ID]; ok {
			year = v
		}
		o.MonthCents, o.YearCents = month*100, year*100
		o.Online = month > 0
		out = append(out, o)
	}
	return out
}

func (s *Server) offerFor(plan, period string) (planOffer, int, bool) {
	for _, o := range s.offers() {
		if o.ID == plan {
			amt := o.MonthCents
			if period == "year" {
				amt = o.YearCents
			}
			return o, amt, o.Online && amt > 0
		}
	}
	return planOffer{}, 0, false
}

type checkoutRow struct {
	ID          string     `json:"id"`
	Plan        string     `json:"plan"`
	Period      string     `json:"period"`
	AmountCents int        `json:"amount_cents"`
	Status      string     `json:"status"`
	InitPoint   string     `json:"init_point"`
	CreatedAt   time.Time  `json:"created_at"`
	PaidAt      *time.Time `json:"paid_at"`
}

func (s *Server) billingOverview(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(), `
		SELECT id, plan, period, amount_cents, status, init_point, created_at, paid_at
		FROM billing_checkouts WHERE clinic_id = $1 ORDER BY created_at DESC LIMIT 20`, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	list := []checkoutRow{}
	for rows.Next() {
		var c checkoutRow
		if err := rows.Scan(&c.ID, &c.Plan, &c.Period, &c.AmountCents, &c.Status, &c.InitPoint, &c.CreatedAt, &c.PaidAt); err != nil {
			serverError(w, r, err)
			return
		}
		list = append(list, c)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"offers": s.offers(), "billing": p.Billing.info(time.Now()), "checkouts": list,
		"online": s.cfg.MPAccessToken != "", "sandbox": s.cfg.MPSandbox, "currency": s.cfg.MPCurrency,
	})
}

func (s *Server) billingCheckout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Plan   string `json:"plan"`
		Period string `json:"period"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Period == "" {
		req.Period = "month"
	}
	if req.Period != "month" && req.Period != "year" {
		writeError(w, http.StatusBadRequest, "El periodo debe ser mensual o anual.")
		return
	}
	offer, amount, ok := s.offerFor(req.Plan, req.Period)
	if offer.ID == "" {
		writeError(w, http.StatusBadRequest, "Plan inválido.")
		return
	}
	if !ok {
		writeError(w, http.StatusConflict, "Este plan se contrata a medida. Escríbenos para cotizarlo.")
		return
	}
	if s.cfg.MPAccessToken == "" {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Code: "NOT_CONFIGURED", Message: "El pago en línea no está configurado todavía. Contacta a Caresia para pagar."})
		return
	}
	p := principalFrom(r.Context())

	// The new plan must fit the people already on the account.
	var users, doctors int
	if err := s.db.QueryRow(r.Context(), `
		SELECT count(*), count(*) FILTER (WHERE role = 'doctor') FROM users WHERE clinic_id = $1 AND NOT disabled`, p.ClinicID).Scan(&users, &doctors); err != nil {
		serverError(w, r, err)
		return
	}
	if offer.MaxUsers != nil && users > *offer.MaxUsers {
		writeError(w, http.StatusConflict, "Tienes "+itoa(users)+" cuentas activas y el plan "+offer.Name+" permite "+itoa(*offer.MaxUsers)+". Desactiva cuentas primero.")
		return
	}
	if offer.MaxDoctors != nil && doctors > *offer.MaxDoctors {
		writeError(w, http.StatusConflict, "Tienes "+itoa(doctors)+" médicos activos y el plan "+offer.Name+" permite "+itoa(*offer.MaxDoctors)+". Desactiva cuentas primero.")
		return
	}

	var id string
	if err := s.db.QueryRow(r.Context(), `
		INSERT INTO billing_checkouts (clinic_id, plan, period, amount_cents, created_by) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		p.ClinicID, req.Plan, req.Period, amount, p.UserID).Scan(&id); err != nil {
		serverError(w, r, err)
		return
	}
	periodLabel := map[string]string{"month": "1 mes", "year": "12 meses"}[req.Period]
	pref := map[string]any{
		"items": []map[string]any{{
			"id": id, "title": "Caresia · Plan " + offer.Name + " (" + periodLabel + ")", "quantity": 1,
			"currency_id": s.cfg.MPCurrency, "unit_price": float64(amount) / 100,
		}},
		"external_reference":   "caresia:" + id,
		"back_urls":            s.returnURLs("/suscripcion?pago="),
		"auto_return":          "approved",
		"statement_descriptor": "CARESIA",
	}
	if s.cfg.APIPublicURL != "" {
		pref["notification_url"] = s.cfg.APIPublicURL + "/api/webhooks/mercadopago"
	}
	var raw struct {
		ID               string `json:"id"`
		InitPoint        string `json:"init_point"`
		SandboxInitPoint string `json:"sandbox_init_point"`
	}
	if err := s.mpCall(r.Context(), s.cfg.MPAccessToken, http.MethodPost, "/checkout/preferences", pref, &raw); err != nil {
		_, _ = s.db.Exec(r.Context(), `UPDATE billing_checkouts SET status='failed' WHERE id=$1`, id)
		providerFailure(w, r, err)
		return
	}
	pay := raw.InitPoint
	if s.cfg.MPSandbox && raw.SandboxInitPoint != "" {
		pay = raw.SandboxInitPoint
	}
	if _, err := s.db.Exec(r.Context(), `UPDATE billing_checkouts SET preference_id=$2, init_point=$3 WHERE id=$1`, id, raw.ID, pay); err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "checkout_started", "Inició el pago del plan "+offer.Name+" ("+periodLabel+")", map[string]any{"checkout": id})
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "init_point": pay})
}

// returnURLs are the pages Mercado Pago sends the buyer back to.
func (s *Server) returnURLs(path string) map[string]string {
	base := s.cfg.AppURL
	if base == "" {
		return map[string]string{}
	}
	return map[string]string{"success": base + path + "ok", "pending": base + path + "pendiente", "failure": base + path + "error"}
}

// billingCheckoutStatus lets the return page ask "did my payment arrive?". It re-checks with
// Mercado Pago when the webhook has not landed yet.
func (s *Server) billingCheckoutStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Pago no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	read := func() (checkoutRow, error) {
		var c checkoutRow
		err := s.db.QueryRow(r.Context(), `
			SELECT id, plan, period, amount_cents, status, init_point, created_at, paid_at FROM billing_checkouts WHERE clinic_id=$1 AND id=$2`, p.ClinicID, id).
			Scan(&c.ID, &c.Plan, &c.Period, &c.AmountCents, &c.Status, &c.InitPoint, &c.CreatedAt, &c.PaidAt)
		return c, err
	}
	c, err := read()
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Pago no encontrado.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	if c.Status == "pending" && s.cfg.MPAccessToken != "" {
		var found struct {
			Results []struct {
				ID     int64  `json:"id"`
				Status string `json:"status"`
			} `json:"results"`
		}
		if err := s.mpCall(r.Context(), s.cfg.MPAccessToken, http.MethodGet, "/v1/payments/search?external_reference="+url.QueryEscape("caresia:"+id), nil, &found); err == nil {
			for _, pay := range found.Results {
				if pay.Status == "approved" {
					if err := s.settleCheckout(r.Context(), strconv.FormatInt(pay.ID, 10)); err != nil {
						logf(r, "settle checkout: %v", err)
					}
					break
				}
			}
			c, _ = read()
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"checkout": c})
}

// mpWebhook receives payment notifications. The body is never trusted: the payment is fetched
// from Mercado Pago and matched to a pending checkout by its external reference.
func (s *Server) mpWebhook(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	var n struct {
		Type   string `json:"type"`
		Action string `json:"action"`
		Data   struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(body, &n)
	id := n.Data.ID
	if id == "" {
		id = r.URL.Query().Get("data.id")
	}
	if s.cfg.MPWebhookSecret != "" && !s.validWebhookSignature(r, id) {
		writeError(w, http.StatusUnauthorized, "Firma inválida.")
		return
	}
	topic := n.Type
	if topic == "" {
		topic = r.URL.Query().Get("type")
	}
	if topic == "payment" && id != "" && s.cfg.MPAccessToken != "" {
		if err := s.settleCheckout(r.Context(), id); err != nil {
			logf(r, "webhook: %v", err)
			writeError(w, http.StatusInternalServerError, "No se pudo procesar.") // MP retries
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) validWebhookSignature(r *http.Request, dataID string) bool {
	var ts, v1 string
	for _, part := range strings.Split(r.Header.Get("x-signature"), ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			switch kv[0] {
			case "ts":
				ts = kv[1]
			case "v1":
				v1 = kv[1]
			}
		}
	}
	if ts == "" || v1 == "" {
		return false
	}
	if q := r.URL.Query().Get("data.id"); q != "" {
		dataID = q
	}
	manifest := "id:" + strings.ToLower(dataID) + ";request-id:" + r.Header.Get("x-request-id") + ";ts:" + ts + ";"
	mac := hmac.New(sha256.New, []byte(s.cfg.MPWebhookSecret))
	mac.Write([]byte(manifest))
	want := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(strings.ToLower(v1)))
}

// settleCheckout fetches a payment from Mercado Pago and, when it is approved and belongs to a
// pending checkout for the right amount, extends the clinic's subscription. Idempotent.
func (s *Server) settleCheckout(ctx context.Context, paymentID string) error {
	var pay struct {
		ID                int64   `json:"id"`
		Status            string  `json:"status"`
		ExternalReference string  `json:"external_reference"`
		Amount            float64 `json:"transaction_amount"`
	}
	if err := s.mpCall(ctx, s.cfg.MPAccessToken, http.MethodGet, "/v1/payments/"+url.PathEscape(paymentID), nil, &pay); err != nil {
		var me *mpError
		if errors.As(err, &me) && me.Status == 404 {
			return nil // not ours (e.g. another account's payment)
		}
		return err
	}
	ref, ok := strings.CutPrefix(pay.ExternalReference, "caresia:")
	if !ok || !validUUID(ref) || pay.Status != "approved" {
		return nil
	}
	return inTx(ctx, s.db, func(tx pgx.Tx) error {
		var clinicID, plan, period, status string
		var amount int
		err := tx.QueryRow(ctx, `SELECT clinic_id, plan, period, amount_cents, status FROM billing_checkouts WHERE id = $1 FOR UPDATE`, ref).
			Scan(&clinicID, &plan, &period, &amount, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if status == "paid" {
			return nil
		}
		if int(pay.Amount*100+0.5) < amount {
			return nil // underpaid: leave it pending for support to review
		}
		months := 1
		if period == "year" {
			months = 12
		}
		var name string
		var cur *time.Time
		if err := tx.QueryRow(ctx, `SELECT name, current_period_end FROM clinics WHERE id = $1 FOR UPDATE`, clinicID).Scan(&name, &cur); err != nil {
			return err
		}
		base := time.Now()
		if cur != nil && cur.After(base) {
			base = *cur
		}
		end := base.AddDate(0, months, 0)
		pid := strconv.FormatInt(pay.ID, 10)
		if _, err := tx.Exec(ctx, `
			INSERT INTO payments (clinic_id, amount_cents, plan, months, note, period_end, provider, provider_ref)
			VALUES ($1,$2,$3,$4,$5,$6,'mercadopago',$7) ON CONFLICT DO NOTHING`,
			clinicID, amount, plan, months, "Pago en línea (Mercado Pago)", end, pid); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE clinics SET plan = $2, billing_status = 'active', current_period_end = $3, suspended_at = NULL, suspended_reason = '', updated_at = now()
			WHERE id = $1`, clinicID, plan, end); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE billing_checkouts SET status='paid', paid_at=now(), mp_payment_id=$2 WHERE id=$1`, ref, pid); err != nil {
			return err
		}
		audit(ctx, tx, clinicID, nil, "payment_online", "Pago en línea recibido: plan "+plan+" por "+itoa(months)+" mes(es), $"+cents(amount), map[string]any{"mp_payment": pid})
		return nil
	})
}

// ---------------------------------------------------------------------------
// Clinic's own Mercado Pago account (OAuth)
// ---------------------------------------------------------------------------

func (s *Server) signState(clinicID string) string {
	exp := strconv.FormatInt(time.Now().Add(15*time.Minute).Unix(), 10)
	payload := clinicID + "." + exp
	mac := hmac.New(sha256.New, s.cfg.OAuthStateSecret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + hex.EncodeToString(mac.Sum(nil))
}

func (s *Server) verifyState(state string) (string, bool) {
	parts := strings.SplitN(state, ".", 2)
	if len(parts) != 2 {
		return "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	mac := hmac.New(sha256.New, s.cfg.OAuthStateSecret)
	mac.Write(raw)
	if !hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(parts[1])) {
		return "", false
	}
	f := strings.SplitN(string(raw), ".", 2)
	if len(f) != 2 || !validUUID(f[0]) {
		return "", false
	}
	if exp, err := strconv.ParseInt(f[1], 10, 64); err != nil || time.Now().Unix() > exp {
		return "", false
	}
	return f[0], true
}

func (s *Server) pointConnect(w http.ResponseWriter, r *http.Request) {
	st := s.providerStatus(r.Context(), principalFrom(r.Context()).ClinicID)
	if !st.PointAvailable {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Code: "NOT_CONFIGURED", Message: "La conexión con Mercado Pago no está configurada en el servidor (faltan MP_CLIENT_ID, MP_CLIENT_SECRET y API_PUBLIC_URL)."})
		return
	}
	q := url.Values{
		"client_id": {s.cfg.MPClientID}, "response_type": {"code"}, "platform_id": {"mp"},
		"state": {s.signState(principalFrom(r.Context()).ClinicID)}, "redirect_uri": {s.oauthRedirect()},
	}
	writeJSON(w, http.StatusOK, map[string]any{"url": "https://auth.mercadopago.com/authorization?" + q.Encode()})
}

// pointCallback finishes the OAuth dance; the browser lands here from Mercado Pago.
func (s *Server) pointCallback(w http.ResponseWriter, r *http.Request) {
	back := func(result string) {
		http.Redirect(w, r, s.cfg.AppURL+"/pos/ajustes?mp="+result, http.StatusFound)
	}
	clinicID, ok := s.verifyState(r.URL.Query().Get("state"))
	code := r.URL.Query().Get("code")
	if !ok || code == "" {
		back("error")
		return
	}
	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		UserID       int64  `json:"user_id"`
	}
	form := map[string]string{
		"client_id": s.cfg.MPClientID, "client_secret": s.cfg.MPClientSecret, "grant_type": "authorization_code",
		"code": code, "redirect_uri": s.oauthRedirect(),
	}
	if err := s.mpCall(r.Context(), "", http.MethodPost, "/oauth/token", form, &tok); err != nil || tok.AccessToken == "" {
		logf(r, "oauth exchange: %v", err)
		back("error")
		return
	}
	at, err1 := s.seal(tok.AccessToken)
	rt, err2 := s.seal(tok.RefreshToken)
	if err1 != nil || err2 != nil {
		back("error")
		return
	}
	expires := time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	if _, err := s.db.Exec(r.Context(), `
		INSERT INTO mp_accounts (clinic_id, mp_user_id, access_token, refresh_token, expires_at) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (clinic_id) DO UPDATE SET mp_user_id=$2, access_token=$3, refresh_token=$4, expires_at=$5, connected_at=now()`,
		clinicID, strconv.FormatInt(tok.UserID, 10), at, rt, expires); err != nil {
		logf(r, "save mp account: %v", err)
		back("error")
		return
	}
	audit(r.Context(), s.db, clinicID, nil, "mp_connected", "Conectó su cuenta de Mercado Pago", nil)
	back("ok")
}

func (s *Server) pointDisconnect(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	if _, err := s.db.Exec(r.Context(), `DELETE FROM mp_accounts WHERE clinic_id = $1`, p.ClinicID); err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "mp_disconnected", "Desconectó su cuenta de Mercado Pago", nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

var errNotConnected = &httpError{Status: http.StatusConflict, Code: "MP_NOT_CONNECTED", Msg: "Conecta tu cuenta de Mercado Pago en Ajustes de cobros."}

// clinicToken returns a valid access token for the clinic's Mercado Pago account, refreshing it if needed.
func (s *Server) clinicToken(ctx context.Context, clinicID string) (string, error) {
	var at, rt []byte
	var exp *time.Time
	err := s.db.QueryRow(ctx, `SELECT access_token, refresh_token, expires_at FROM mp_accounts WHERE clinic_id = $1`, clinicID).Scan(&at, &rt, &exp)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errNotConnected
	}
	if err != nil {
		return "", err
	}
	token, err := s.open(at)
	if err != nil {
		return "", err
	}
	if exp != nil && time.Until(*exp) < 24*time.Hour {
		refresh, err := s.open(rt)
		if err != nil {
			return token, nil
		}
		var tok struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int    `json:"expires_in"`
		}
		form := map[string]string{"client_id": s.cfg.MPClientID, "client_secret": s.cfg.MPClientSecret, "grant_type": "refresh_token", "refresh_token": refresh}
		if err := s.mpCall(ctx, "", http.MethodPost, "/oauth/token", form, &tok); err == nil && tok.AccessToken != "" {
			nat, e1 := s.seal(tok.AccessToken)
			nrt, e2 := s.seal(tok.RefreshToken)
			if e1 == nil && e2 == nil {
				_, _ = s.db.Exec(ctx, `UPDATE mp_accounts SET access_token=$2, refresh_token=$3, expires_at=$4 WHERE clinic_id=$1`,
					clinicID, nat, nrt, time.Now().Add(time.Duration(tok.ExpiresIn)*time.Second))
				return tok.AccessToken, nil
			}
		}
	}
	return token, nil
}

// ---------------------------------------------------------------------------
// Point terminals and payment links
// ---------------------------------------------------------------------------

type pointDevice struct {
	ID            string `json:"id"`
	OperatingMode string `json:"operating_mode"`
	PosID         int64  `json:"pos_id"`
	StoreID       string `json:"store_id"`
}

func (s *Server) pointDevices(w http.ResponseWriter, r *http.Request) {
	token, err := s.clinicToken(r.Context(), principalFrom(r.Context()).ClinicID)
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	var out struct {
		Devices []pointDevice `json:"devices"`
	}
	if err := s.mpCall(r.Context(), token, http.MethodGet, "/point/integration-api/devices", nil, &out); err != nil {
		providerFailure(w, r, err)
		return
	}
	if out.Devices == nil {
		out.Devices = []pointDevice{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": out.Devices})
}

// pointMode switches a terminal to PDV (it only charges what the app sends it).
func (s *Server) pointMode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Mode string `json:"mode"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Mode != "PDV" && req.Mode != "STANDALONE" {
		writeError(w, http.StatusBadRequest, "Modo inválido.")
		return
	}
	token, err := s.clinicToken(r.Context(), principalFrom(r.Context()).ClinicID)
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	if err := s.mpCall(r.Context(), token, http.MethodPatch, "/point/integration-api/devices/"+url.PathEscape(id), map[string]string{"operating_mode": req.Mode}, nil); err != nil {
		providerFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) pointCreateIntent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID    string `json:"device_id"`
		AmountCents int    `json:"amount_cents"`
		Reference   string `json:"reference"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.DeviceID == "" || len(req.DeviceID) > 80 || req.AmountCents < 100 || !okCents(req.AmountCents) {
		writeError(w, http.StatusBadRequest, "Elige la terminal y un monto válido (mínimo $1.00).")
		return
	}
	p := principalFrom(r.Context())
	token, err := s.clinicToken(r.Context(), p.ClinicID)
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	ref := "caresia:" + p.ClinicID[:8] + ":" + strconv.FormatInt(time.Now().Unix(), 10)
	var out struct {
		ID string `json:"id"`
	}
	body := map[string]any{"amount": req.AmountCents, "additional_info": map[string]any{"external_reference": ref, "print_on_terminal": true}}
	if err := s.mpCall(r.Context(), token, http.MethodPost, "/point/integration-api/devices/"+url.PathEscape(req.DeviceID)+"/payment-intents", body, &out); err != nil {
		var me *mpError
		if errors.As(err, &me) && me.Status == 409 {
			writeError(w, http.StatusConflict, "La terminal tiene un cobro pendiente. Cancélalo en la terminal o espera a que termine.")
			return
		}
		providerFailure(w, r, err)
		return
	}
	if _, err := s.db.Exec(r.Context(), `INSERT INTO mp_charges (id, clinic_id, kind, device_id, amount_cents, external_ref) VALUES ($1,$2,'point',$3,$4,$5)`,
		out.ID, p.ClinicID, req.DeviceID, req.AmountCents, ref); err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": out.ID, "status": "open"})
}

type chargeRow struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Status      string `json:"status"` // open | approved | canceled | error
	AmountCents int    `json:"amount_cents"`
	PayURL      string `json:"pay_url,omitempty"`
	Used        bool   `json:"used"`
}

func (s *Server) loadCharge(ctx context.Context, clinicID, id string) (chargeRow, string, string, error) {
	var c chargeRow
	var device, ref string
	var sale *string
	err := s.db.QueryRow(ctx, `SELECT id, kind, status, amount_cents, pay_url, device_id, external_ref, sale_id::text FROM mp_charges WHERE clinic_id=$1 AND id=$2`, clinicID, id).
		Scan(&c.ID, &c.Kind, &c.Status, &c.AmountCents, &c.PayURL, &device, &ref, &sale)
	c.Used = sale != nil
	return c, device, ref, err
}

// pointIntentStatus polls the terminal's intent and, once finished, confirms the payment with Mercado Pago.
func (s *Server) chargeStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p := principalFrom(r.Context())
	c, device, ref, err := s.loadCharge(r.Context(), p.ClinicID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Cobro no encontrado.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	if c.Status == "open" {
		token, err := s.clinicToken(r.Context(), p.ClinicID)
		if err != nil {
			writeFailure(w, r, err)
			return
		}
		switch c.Kind {
		case "point":
			var in struct {
				State   string `json:"state"`
				Payment struct {
					ID int64 `json:"id"`
				} `json:"payment"`
			}
			if err := s.mpCall(r.Context(), token, http.MethodGet, "/point/integration-api/payment-intents/"+url.PathEscape(id), nil, &in); err != nil {
				var me *mpError
				if errors.As(err, &me) && me.Status == 404 {
					in.State = "CANCELED"
				} else {
					providerFailure(w, r, err)
					return
				}
			}
			switch strings.ToUpper(in.State) {
			case "FINISHED", "PROCESSED":
				if in.Payment.ID != 0 {
					c.Status = s.confirmPayment(r.Context(), token, p.ClinicID, id, strconv.FormatInt(in.Payment.ID, 10), c.AmountCents)
				}
			case "CANCELED", "ABANDONED", "EXPIRED":
				c.Status = s.setCharge(r.Context(), id, "canceled")
			case "ERROR":
				c.Status = s.setCharge(r.Context(), id, "error")
			}
		case "link":
			var found struct {
				Results []struct {
					ID     int64  `json:"id"`
					Status string `json:"status"`
				} `json:"results"`
			}
			if err := s.mpCall(r.Context(), token, http.MethodGet, "/v1/payments/search?external_reference="+url.QueryEscape(ref), nil, &found); err != nil {
				providerFailure(w, r, err)
				return
			}
			for _, pay := range found.Results {
				if pay.Status == "approved" {
					c.Status = s.confirmPayment(r.Context(), token, p.ClinicID, id, strconv.FormatInt(pay.ID, 10), c.AmountCents)
					break
				}
			}
		}
		_ = device
	}
	writeJSON(w, http.StatusOK, map[string]any{"charge": c})
}

func (s *Server) setCharge(ctx context.Context, id, status string) string {
	_, _ = s.db.Exec(ctx, `UPDATE mp_charges SET status = $2 WHERE id = $1 AND status = 'open'`, id, status)
	return status
}

// confirmPayment re-reads the payment from Mercado Pago before trusting it.
func (s *Server) confirmPayment(ctx context.Context, token, clinicID, chargeID, paymentID string, wantCents int) string {
	var pay struct {
		Status string  `json:"status"`
		Amount float64 `json:"transaction_amount"`
	}
	if err := s.mpCall(ctx, token, http.MethodGet, "/v1/payments/"+url.PathEscape(paymentID), nil, &pay); err != nil {
		return "open"
	}
	switch pay.Status {
	case "approved":
		if int(pay.Amount*100+0.5) != wantCents {
			return s.setCharge(ctx, chargeID, "error")
		}
		_, _ = s.db.Exec(ctx, `UPDATE mp_charges SET status='approved', mp_payment_id=$3 WHERE clinic_id=$1 AND id=$2 AND status='open'`, clinicID, chargeID, paymentID)
		return "approved"
	case "rejected", "cancelled":
		return s.setCharge(ctx, chargeID, "error")
	}
	return "open"
}

func (s *Server) chargeCancel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p := principalFrom(r.Context())
	c, device, _, err := s.loadCharge(r.Context(), p.ClinicID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Cobro no encontrado.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	if c.Status != "open" {
		writeError(w, http.StatusConflict, "Ese cobro ya no está pendiente.")
		return
	}
	if c.Kind == "point" {
		token, err := s.clinicToken(r.Context(), p.ClinicID)
		if err != nil {
			writeFailure(w, r, err)
			return
		}
		if err := s.mpCall(r.Context(), token, http.MethodDelete, "/point/integration-api/devices/"+url.PathEscape(device)+"/payment-intents/"+url.PathEscape(id), nil, nil); err != nil {
			var me *mpError
			if !errors.As(err, &me) || me.Status != 404 {
				providerFailure(w, r, err)
				return
			}
		}
	}
	s.setCharge(r.Context(), id, "canceled")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// linkCreate makes a Checkout Pro link the patient can pay from their phone (QR on the screen).
func (s *Server) linkCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AmountCents int    `json:"amount_cents"`
		Title       string `json:"title"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		req.Title = "Cobro"
	}
	if len([]rune(req.Title)) > 80 || req.AmountCents < 100 || !okCents(req.AmountCents) {
		writeError(w, http.StatusBadRequest, "El monto o el concepto no son válidos.")
		return
	}
	p := principalFrom(r.Context())
	token, err := s.clinicToken(r.Context(), p.ClinicID)
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	ref := "caresia-link:" + p.ClinicID[:8] + ":" + strconv.FormatInt(time.Now().UnixNano(), 10)
	pref := map[string]any{
		"items":              []map[string]any{{"title": req.Title, "quantity": 1, "currency_id": s.cfg.MPCurrency, "unit_price": float64(req.AmountCents) / 100}},
		"external_reference": ref,
	}
	var out struct {
		ID               string `json:"id"`
		InitPoint        string `json:"init_point"`
		SandboxInitPoint string `json:"sandbox_init_point"`
	}
	if err := s.mpCall(r.Context(), token, http.MethodPost, "/checkout/preferences", pref, &out); err != nil {
		providerFailure(w, r, err)
		return
	}
	pay := out.InitPoint
	if s.cfg.MPSandbox && out.SandboxInitPoint != "" {
		pay = out.SandboxInitPoint
	}
	if _, err := s.db.Exec(r.Context(), `INSERT INTO mp_charges (id, clinic_id, kind, amount_cents, external_ref, pay_url) VALUES ($1,$2,'link',$3,$4,$5)`,
		out.ID, p.ClinicID, req.AmountCents, ref, pay); err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": out.ID, "url": pay})
}

// claimCharge reserves an approved terminal payment or payment link for one sale.
func claimCharge(ctx context.Context, tx pgx.Tx, clinicID, kind, id string, amountCents int) (string, error) {
	if id == "" {
		return "", fail(http.StatusBadRequest, "Falta el cobro de Mercado Pago.")
	}
	var status string
	var amount int
	var sale *string
	var payID *string
	err := tx.QueryRow(ctx, `SELECT status, amount_cents, sale_id::text, mp_payment_id FROM mp_charges WHERE clinic_id=$1 AND id=$2 AND kind=$3 FOR UPDATE`, clinicID, id, kind).
		Scan(&status, &amount, &sale, &payID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fail(http.StatusBadRequest, "No encontramos ese cobro de Mercado Pago.")
	}
	if err != nil {
		return "", err
	}
	if status != "approved" {
		return "", fail(http.StatusConflict, "El pago con Mercado Pago aún no está aprobado.")
	}
	if sale != nil {
		return "", fail(http.StatusConflict, "Ese pago ya se usó en otra venta.")
	}
	if amount != amountCents {
		return "", fail(http.StatusBadRequest, "El monto no coincide con el cobro aprobado ($"+cents(amount)+").")
	}
	if payID != nil {
		return *payID, nil
	}
	return id, nil
}
