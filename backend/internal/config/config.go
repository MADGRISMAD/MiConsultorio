// Package config loads runtime configuration from environment variables.
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr           string
	DatabaseURL    string
	JWTSecret      []byte
	CookieSecure   bool
	SessionTTL     time.Duration
	AllowedOrigins []string // extra origins allowed to make state-changing requests
	StaticDir      string   // optional: serve the built frontend from this directory

	// Optional first-run setup. The platform administrator is created when none exists;
	// the sample clinic when the database has no clinics.
	PlatformName, PlatformEmail, PlatformUsername, PlatformPassword string
	ClinicName, ClinicKind                                          string
	ClinicAdminName, ClinicAdminEmail, ClinicAdminUsername          string
	ClinicAdminPassword                                             string

	// Public addresses: the web app (return pages) and this API (webhooks, OAuth callbacks).
	AppURL, APIPublicURL string

	// Mercado Pago. Subscriptions are charged to the platform's own account (MPAccessToken);
	// each clinic connects its own account with OAuth to charge its patients.
	MPAccessToken, MPPublicKey, MPWebhookSecret string
	MPClientID, MPClientSecret, MPOAuthRedirect string
	MPAPIBase                                   string // override in tests
	MPAuthBase                                  string // where the owner authorizes the connection (country site)
	MPCurrency                                  string
	MPSandbox                                   bool
	PlanPriceMonth, PlanPriceYear               map[string]int // MXN overrides per plan id
	OAuthStateSecret                            []byte
	TokenEncKey                                 []byte // 32 bytes, encrypts stored provider tokens

	// Outgoing e-mail (password recovery, tickets, notices). Empty SMTPUser/SMTPHost disables it.
	SMTPHost, SMTPUser, SMTPPass, MailFrom string
	SMTPPort                               int
	SMTPSecure                             bool

	// Magic inventory / pricing (Gemini).
	GeminiAPIKey, GeminiModel, GeminiAPIBase string

	// Attachments (radiografías, laboratorio, consentimientos): files live on disk under UploadsDir.
	UploadsDir     string
	MaxUploadBytes int64

	// WhatsApp Business (Meta Cloud API) for reminders. Empty token or phone id disables it.
	WhatsAppToken, WhatsAppPhoneID, WhatsAppAPIBase, WhatsAppTemplate, WhatsAppLang string

	// CFDI stamping through Facturama. Empty credentials leave invoices as requests to issue by hand.
	FacturamaUser, FacturamaPass, FacturamaBase string

	// Operation: build identifier shown by /api/health and the file where scripts/backup.sh records its last run.
	BuildCommit, BackupStatusFile string
}

func Load() (*Config, error) {
	c := &Config{
		Addr:         env("ADDR", ":8080"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		CookieSecure: env("COOKIE_SECURE", "true") != "false",
		SessionTTL:   12 * time.Hour,
		StaticDir:    os.Getenv("STATIC_DIR"),

		PlatformName:     env("PLATFORM_ADMIN_NAME", "Administrador de Caresia"),
		PlatformEmail:    os.Getenv("PLATFORM_ADMIN_EMAIL"),
		PlatformUsername: env("PLATFORM_ADMIN_USERNAME", "plataforma"),
		PlatformPassword: os.Getenv("PLATFORM_ADMIN_PASSWORD"),

		ClinicName:          env("CLINIC_NAME", "Mi Clínica"),
		ClinicKind:          env("CLINIC_KIND", "GENERAL_MEDICAL"),
		ClinicAdminName:     os.Getenv("CLINIC_ADMIN_NAME"),
		ClinicAdminEmail:    first("CLINIC_ADMIN_EMAIL", "ADMIN_EMAIL"), // ADMIN_* are the old names
		ClinicAdminUsername: firstOr("admin", "CLINIC_ADMIN_USERNAME", "ADMIN_USERNAME"),
		ClinicAdminPassword: first("CLINIC_ADMIN_PASSWORD", "ADMIN_PASSWORD"),
	}
	c.AppURL = strings.TrimRight(os.Getenv("APP_URL"), "/")
	// API_PUBLIC_URL is the server's address WITHOUT "/api" (routes add it); accept it with the suffix too.
	c.APIPublicURL = strings.TrimSuffix(strings.TrimRight(os.Getenv("API_PUBLIC_URL"), "/"), "/api")
	c.MPAccessToken, c.MPPublicKey, c.MPWebhookSecret = os.Getenv("MP_ACCESS_TOKEN"), os.Getenv("MP_PUBLIC_KEY"), os.Getenv("MP_WEBHOOK_SECRET")
	c.MPClientID, c.MPClientSecret, c.MPOAuthRedirect = os.Getenv("MP_CLIENT_ID"), os.Getenv("MP_CLIENT_SECRET"), os.Getenv("MP_OAUTH_REDIRECT")
	c.MPAPIBase = strings.TrimRight(env("MP_API_BASE", "https://api.mercadopago.com"), "/")
	c.MPAuthBase = strings.TrimRight(env("MP_AUTH_BASE", "https://auth.mercadopago.com.mx"), "/")
	c.MPCurrency = env("MP_CURRENCY", "MXN")
	c.MPSandbox = env("MP_SANDBOX", "false") == "true"
	c.PlanPriceMonth, c.PlanPriceYear = map[string]int{}, map[string]int{}
	for id, name := range map[string]string{"basico": "BASICO", "crecimiento": "CRECIMIENTO", "pro": "PRO"} {
		if n, ok := envInt("MP_PLAN_" + name + "_PRICE"); ok {
			c.PlanPriceMonth[id] = n
		}
		if n, ok := envInt("MP_PLAN_" + name + "_YEAR_PRICE"); ok {
			c.PlanPriceYear[id] = n
		}
	}
	c.SMTPHost = os.Getenv("SMTP_HOST")
	if c.SMTPHost == "" && strings.EqualFold(os.Getenv("SMTP_SERVICE"), "gmail") {
		c.SMTPHost = "smtp.gmail.com"
	}
	c.SMTPUser, c.SMTPPass = os.Getenv("SMTP_USER"), os.Getenv("SMTP_PASS")
	c.SMTPSecure = env("SMTP_SECURE", "false") == "true"
	c.SMTPPort = 587
	if c.SMTPSecure {
		c.SMTPPort = 465
	}
	if v := os.Getenv("SMTP_PORT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > 65535 {
			return nil, errors.New("SMTP_PORT must be a valid port number")
		}
		c.SMTPPort = n
	}
	c.MailFrom = os.Getenv("MAIL_FROM")
	if c.MailFrom == "" && c.SMTPUser != "" {
		c.MailFrom = "Caresia <" + c.SMTPUser + ">"
	}
	c.GeminiAPIKey, c.GeminiModel = os.Getenv("GEMINI_API_KEY"), env("GEMINI_MODEL", "gemini-2.5-flash-lite")
	c.GeminiAPIBase = strings.TrimRight(env("GEMINI_API_BASE", "https://generativelanguage.googleapis.com"), "/")
	c.UploadsDir = env("UPLOADS_DIR", "data/uploads")
	c.MaxUploadBytes = 15 << 20
	if n, ok := envInt("MAX_UPLOAD_MB"); ok && n > 0 && n <= 200 {
		c.MaxUploadBytes = int64(n) << 20
	}
	c.WhatsAppToken, c.WhatsAppPhoneID = os.Getenv("WHATSAPP_TOKEN"), os.Getenv("WHATSAPP_PHONE_ID")
	c.WhatsAppAPIBase = strings.TrimRight(env("WHATSAPP_API_BASE", "https://graph.facebook.com/v20.0"), "/")
	c.WhatsAppTemplate, c.WhatsAppLang = env("WHATSAPP_TEMPLATE", "recordatorio_cita"), env("WHATSAPP_LANG", "es_MX")
	c.FacturamaUser, c.FacturamaPass = os.Getenv("FACTURAMA_USER"), os.Getenv("FACTURAMA_PASS")
	c.FacturamaBase = strings.TrimRight(env("FACTURAMA_BASE", "https://api.facturama.mx"), "/")
	c.BuildCommit, c.BackupStatusFile = os.Getenv("BUILD_COMMIT"), os.Getenv("BACKUP_STATUS_FILE")
	if c.StaticDir == "" {
		for _, d := range []string{"../frontend/build", "frontend/build"} {
			if _, err := os.Stat(d + "/index.html"); err == nil {
				c.StaticDir = d
				break
			}
		}
	}
	if c.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		return nil, errors.New("JWT_SECRET is required and must be at least 32 characters")
	}
	c.JWTSecret = []byte(secret)
	// Provider secrets: use the dedicated variables when given, otherwise derive them from JWT_SECRET.
	c.OAuthStateSecret = deriveKey("OAUTH_STATE_SECRET", secret, "oauth-state")
	c.TokenEncKey = deriveKey("TOKEN_ENC_KEY", secret, "token-enc")
	for _, o := range strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.AllowedOrigins = append(c.AllowedOrigins, strings.TrimRight(o, "/"))
		}
	}
	if v := os.Getenv("SESSION_TTL_HOURS"); v != "" {
		var h int
		if _, err := fmt.Sscanf(v, "%d", &h); err != nil || h <= 0 {
			return nil, errors.New("SESSION_TTL_HOURS must be a positive integer")
		}
		c.SessionTTL = time.Duration(h) * time.Hour
	}
	return c, nil
}

// first returns the first non-empty variable among keys.
func first(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func firstOr(def string, keys ...string) string {
	if v := first(keys...); v != "" {
		return v
	}
	return def
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// envInt reads a whole-number variable (pesos). Missing or invalid means "not set".
func envInt(key string) (int, bool) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// deriveKey returns 32 bytes: the hex value of the variable when it is a 64-char hex string,
// otherwise a SHA-256 derivation from the JWT secret and a purpose label.
func deriveKey(name, jwtSecret, purpose string) []byte {
	if raw, err := hex.DecodeString(strings.TrimSpace(os.Getenv(name))); err == nil && len(raw) == 32 {
		return raw
	}
	sum := sha256.Sum256([]byte(purpose + ":" + jwtSecret))
	return sum[:]
}
