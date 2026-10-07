package api

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// Cobros (point of sale). Capabilities:
//
//	pos         sell, open/close the register, see the catalog and past sales
//	posReports  sales reports and invoice requests
//	posManage   edit the catalog and stock, void sales, change settings, connect payment providers
const (
	PermPOS        = "pos"
	PermPOSReports = "posReports"
	PermPOSManage  = "posManage"
)

// requireCobros blocks the POS for plans that do not include it (Básico).
func requireCobros(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := principalFrom(r.Context())
		if p == nil || p.Billing == nil {
			writeError(w, http.StatusForbidden, "Esta sección es solo para cuentas de un consultorio.")
			return
		}
		if plan, ok := planByID(p.Billing.Plan); !ok || !plan.Cobros {
			writeJSON(w, http.StatusForbidden, errorBody{
				Code:    "PLAN_REQUIRED",
				Message: "Cobros viene con los planes Crecimiento y Pro.",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------------------
// Settings
// ---------------------------------------------------------------------------

type PrinterPrefs struct {
	// Kind is how this business usually prints: browser (any printer), usb, serial or bluetooth (thermal, ESC/POS).
	Kind       string `json:"kind"`
	Width      int    `json:"width"` // paper width in mm: 58 or 80
	Copies     int    `json:"copies"`
	AutoPrint  bool   `json:"auto_print"`
	OpenDrawer bool   `json:"open_drawer"` // pulse the cash drawer on cash sales
	Cut        bool   `json:"cut"`
}

type PosSettings struct {
	// Fiscal and ticket data
	BusinessName string `json:"business_name"`
	LegalName    string `json:"legal_name"`
	RFC          string `json:"rfc"`
	TaxRegime    string `json:"tax_regime"`
	TaxAddress   string `json:"tax_address"`
	ZipCode      string `json:"zip_code"`
	Phone        string `json:"phone"`
	TicketHeader string `json:"ticket_header"`
	TicketFooter string `json:"ticket_footer"`
	ShowTaxLine  bool   `json:"show_tax_line"`

	// Rules
	Currency           string   `json:"currency"`
	DefaultTaxRate     float64  `json:"default_tax_rate"`
	AllowNegativeStock bool     `json:"allow_negative_stock"`
	RequireOpenCash    bool     `json:"require_open_cash"`
	AllowDiscounts     bool     `json:"allow_discounts"`
	MaxDiscountPct     int      `json:"max_discount_pct"`
	Methods            []string `json:"methods"` // enabled payment methods

	Printer PrinterPrefs `json:"printer"`
}

var allPayMethods = []string{"cash", "card", "transfer", "mp_point", "mp_link", "other"}

func defaultPosSettings() PosSettings {
	return PosSettings{
		Currency: "MXN", DefaultTaxRate: 0, RequireOpenCash: true, AllowDiscounts: true, MaxDiscountPct: 100,
		ShowTaxLine: true, TicketFooter: "¡Gracias por su preferencia!",
		Methods: []string{"cash", "card", "transfer"},
		Printer: PrinterPrefs{Kind: "browser", Width: 80, Copies: 1, Cut: true},
	}
}

func (s PosSettings) normalized() PosSettings {
	def := defaultPosSettings()
	if s.Currency == "" {
		s.Currency = def.Currency
	}
	if s.Methods == nil {
		s.Methods = def.Methods
	}
	if s.Printer.Kind == "" {
		s.Printer.Kind = def.Printer.Kind
	}
	if s.Printer.Width == 0 {
		s.Printer.Width = def.Printer.Width
	}
	if s.Printer.Copies == 0 {
		s.Printer.Copies = 1
	}
	return s
}

func (s PosSettings) validate() string {
	for _, f := range []struct {
		v    string
		max  int
		name string
	}{
		{s.BusinessName, 120, "El nombre comercial"}, {s.LegalName, 200, "La razón social"}, {s.TaxAddress, 300, "El domicilio fiscal"},
		{s.TicketHeader, 400, "El encabezado del ticket"}, {s.TicketFooter, 400, "El pie del ticket"}, {s.Phone, 40, "El teléfono"},
		{s.TaxRegime, 80, "El régimen fiscal"},
	} {
		if utf8.RuneCountInString(f.v) > f.max {
			return f.name + " es demasiado largo."
		}
	}
	if s.RFC != "" && !rfcRe.MatchString(strings.ToUpper(s.RFC)) {
		return "El RFC no es válido (12 o 13 caracteres)."
	}
	if s.ZipCode != "" && (len(s.ZipCode) != 5 || strings.Trim(s.ZipCode, "0123456789") != "") {
		return "El código postal debe tener 5 dígitos."
	}
	if s.Currency != "MXN" && s.Currency != "USD" {
		return "Moneda no admitida."
	}
	if s.DefaultTaxRate < 0 || s.DefaultTaxRate > 100 {
		return "El IVA predeterminado debe estar entre 0 y 100."
	}
	if s.MaxDiscountPct < 0 || s.MaxDiscountPct > 100 {
		return "El descuento máximo debe estar entre 0 y 100 %."
	}
	if len(s.Methods) == 0 {
		return "Activa al menos un método de pago."
	}
	for _, m := range s.Methods {
		if !hasPermission(allPayMethods, m) {
			return "Método de pago inválido."
		}
	}
	if s.Printer.Width != 58 && s.Printer.Width != 80 {
		return "El ancho del papel debe ser 58 u 80 mm."
	}
	if !hasPermission([]string{"browser", "usb", "serial", "bluetooth"}, s.Printer.Kind) {
		return "Tipo de impresora inválido."
	}
	if s.Printer.Copies < 1 || s.Printer.Copies > 5 {
		return "Las copias deben estar entre 1 y 5."
	}
	return ""
}

func loadPosSettings(ctx context.Context, q queryRower, clinicID string) (PosSettings, error) {
	var raw []byte
	if err := q.QueryRow(ctx, `SELECT pos_settings FROM clinics WHERE id = $1`, clinicID).Scan(&raw); err != nil {
		return PosSettings{}, err
	}
	s := defaultPosSettings()
	if len(raw) > 2 {
		_ = json.Unmarshal(raw, &s)
	}
	return s.normalized(), nil
}

func (s *Server) getPosSettings(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	cfg, err := loadPosSettings(r.Context(), s.db, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if cfg.BusinessName == "" { // fall back to the clinic's name and phone for tickets
		var name, phone string
		if err := s.db.QueryRow(r.Context(), `SELECT name, phone_number FROM clinics WHERE id = $1`, p.ClinicID).Scan(&name, &phone); err == nil {
			cfg.BusinessName = name
			if cfg.Phone == "" {
				cfg.Phone = phone
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": cfg, "providers": s.providerStatus(r.Context(), p.ClinicID)})
}

func (s *Server) updatePosSettings(w http.ResponseWriter, r *http.Request) {
	var cfg PosSettings
	if !decode(w, r, &cfg) {
		return
	}
	cfg.RFC = strings.ToUpper(strings.TrimSpace(cfg.RFC))
	cfg.BusinessName, cfg.LegalName = strings.TrimSpace(cfg.BusinessName), strings.TrimSpace(cfg.LegalName)
	cfg = cfg.normalized()
	if msg := cfg.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	p := principalFrom(r.Context())
	raw, _ := json.Marshal(cfg)
	if err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `UPDATE clinics SET pos_settings = $2, updated_at = now() WHERE id = $1`, p.ClinicID, raw); err != nil {
			return err
		}
		audit(r.Context(), tx, p.ClinicID, p, "pos_settings", "Actualizó los ajustes de cobros", nil)
		return nil
	}); err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": cfg})
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// money parses and bounds an amount in cents.
const maxCents = 100_000_000 // $1,000,000.00

func okCents(n int) bool { return n >= 0 && n <= maxCents }

func roundDiv(a, b int64) int64 { return int64(math.Round(float64(a) / float64(b))) }

// dayRange turns YYYY-MM-DD bounds (inclusive) into a half-open [from, to) timestamp range
// in the server's local time. Empty inputs default to the last 30 days.
func dayRange(from, to string) (time.Time, time.Time, string) {
	loc := time.Local
	now := time.Now().In(loc)
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1)
	start := end.AddDate(0, 0, -30)
	if from != "" {
		t, err := time.ParseInLocation("2006-01-02", from, loc)
		if err != nil {
			return start, end, "La fecha inicial no es válida."
		}
		start = t
	}
	if to != "" {
		t, err := time.ParseInLocation("2006-01-02", to, loc)
		if err != nil {
			return start, end, "La fecha final no es válida."
		}
		end = t.AddDate(0, 0, 1)
	}
	if !end.After(start) {
		return start, end, "El rango de fechas no es válido."
	}
	if end.Sub(start) > 400*24*time.Hour {
		return start, end, "El rango no puede ser mayor a 13 meses."
	}
	return start, end, ""
}
