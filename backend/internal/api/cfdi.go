package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// CFDI 4.0 stamping through a PAC. The handlers only know cfdiProvider; facturama is the production
// implementation (Multiemisor API: every clinic issues under its own RFC with its CSD uploaded to the
// account). Credentials come from the server configuration and are never logged or returned.

type cfdiParty struct {
	RFC, Name, Regime, Zip string
}

type cfdiItem struct {
	ProductCode, UnitCode, Unit, Description string
	Qty                                      float64
	UnitPrice                                float64 // before tax and discount
	Subtotal, Discount, Base, Tax, Total     float64
	TaxRate                                  float64 // percent; 0 means the line is not subject to tax
}

type cfdiDoc struct {
	Issuer, Receiver cfdiParty
	CfdiUse          string
	PaymentForm      string // c_FormaPago
	PaymentMethod    string // PUE or PPD
	ExpeditionZip    string
	Currency         string
	Folio            string
	Items            []cfdiItem
}

type cfdiStamped struct {
	ProviderID string
	UUID       string
}

type cfdiProvider interface {
	Stamp(ctx context.Context, doc cfdiDoc) (cfdiStamped, error)
	// StampPayment stamps the payment complement (Pago 2.0) of one abono of a PPD invoice.
	StampPayment(ctx context.Context, doc cfdiPaymentDoc) (cfdiStamped, error)
	// Fetch downloads the stamped document; format is "xml" or "pdf".
	Fetch(ctx context.Context, providerID, format string) ([]byte, error)
	Cancel(ctx context.Context, providerID, motive, replacementUUID string) error
}

// cfdiError is a refusal from the PAC that is safe to show to the user (validation messages).
type cfdiError struct {
	Status int
	Msg    string
}

func (e *cfdiError) Error() string { return fmt.Sprintf("pac (%d): %s", e.Status, e.Msg) }

// cfdiProvider returns the configured PAC, or nil when stamping is not set up.
func (s *Server) cfdiProvider() cfdiProvider {
	if s.cfg.FacturamaUser == "" || s.cfg.FacturamaPass == "" {
		return nil
	}
	base := strings.TrimRight(s.cfg.FacturamaBase, "/")
	if base == "" {
		base = "https://api.facturama.mx"
	}
	return &facturama{user: s.cfg.FacturamaUser, pass: s.cfg.FacturamaPass, base: base, http: &http.Client{Timeout: 45 * time.Second}}
}

// ---------------------------------------------------------------------------
// Facturama
// ---------------------------------------------------------------------------

type facturama struct {
	user, pass, base string
	http             *http.Client
}

type fmTax struct {
	Total       float64
	Name        string
	Base        float64
	Rate        float64
	IsRetention bool
}

type fmItem struct {
	ProductCode string
	Description string
	Unit        string
	UnitCode    string
	UnitPrice   float64
	Quantity    float64
	Subtotal    float64
	Discount    float64
	TaxObject   string
	Taxes       []fmTax `json:",omitempty"`
	Total       float64
}

type fmCfdi struct {
	Issuer          map[string]string
	Receiver        map[string]string
	CfdiType        string
	NameId          string
	ExpeditionPlace string
	PaymentForm     string
	PaymentMethod   string
	Currency        string
	Folio           string
	Items           []fmItem
}

func r2(v float64) float64 { return math.Round(v*100) / 100 }
func r6(v float64) float64 { return math.Round(v*1e6) / 1e6 }

func (f *facturama) body(doc cfdiDoc) fmCfdi {
	items := make([]fmItem, 0, len(doc.Items))
	for _, it := range doc.Items {
		x := fmItem{ProductCode: it.ProductCode, Description: it.Description, Unit: it.Unit, UnitCode: it.UnitCode,
			UnitPrice: r6(it.UnitPrice), Quantity: it.Qty, Subtotal: r2(it.Subtotal), Discount: r2(it.Discount), Total: r2(it.Total), TaxObject: "01"}
		if it.TaxRate > 0 {
			x.TaxObject = "02"
			x.Taxes = []fmTax{{Total: r2(it.Tax), Name: "IVA", Base: r2(it.Base), Rate: r6(it.TaxRate / 100)}}
		}
		items = append(items, x)
	}
	return fmCfdi{
		Issuer:          map[string]string{"Rfc": doc.Issuer.RFC, "Name": doc.Issuer.Name, "FiscalRegime": doc.Issuer.Regime},
		Receiver:        map[string]string{"Rfc": doc.Receiver.RFC, "Name": doc.Receiver.Name, "CfdiUse": doc.CfdiUse, "FiscalRegime": doc.Receiver.Regime, "TaxZipCode": doc.Receiver.Zip},
		CfdiType:        "I",
		NameId:          "1",
		ExpeditionPlace: doc.ExpeditionZip,
		PaymentForm:     doc.PaymentForm,
		PaymentMethod:   doc.PaymentMethod,
		Currency:        doc.Currency,
		Folio:           doc.Folio,
		Items:           items,
	}
}

// do sends an authenticated request and returns the body of a 2xx answer.
func (f *facturama) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, f.base+path, rd)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(f.user, f.pass)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := f.http.Do(req)
	if err != nil {
		// the URL (never the credentials) is enough to diagnose a network failure
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, fmt.Errorf("no se pudo conectar con el timbrado: %w", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, &cfdiError{Status: res.StatusCode, Msg: facturamaMessage(res.StatusCode, raw)}
	}
	return raw, nil
}

// facturamaMessage turns an error answer ({"Message": "...", "ModelState": {"field": ["problem"]}}) into text.
func facturamaMessage(status int, raw []byte) string {
	var e struct {
		Message    string
		ModelState map[string][]string
	}
	_ = json.Unmarshal(raw, &e)
	var parts []string
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	keys := make([]string, 0, len(e.ModelState))
	for k := range e.ModelState {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts = append(parts, e.ModelState[k]...)
	}
	msg := strings.Join(parts, " ")
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return "El servicio de timbrado rechazó las credenciales del servidor."
	case msg == "":
		return "El servicio de timbrado respondió con un error (" + fmt.Sprint(status) + ")."
	case len(msg) > 400:
		msg = msg[:400]
	}
	return msg
}

func (f *facturama) Stamp(ctx context.Context, doc cfdiDoc) (cfdiStamped, error) {
	return f.stampRaw(ctx, f.body(doc))
}

// stampRaw posts a CFDI request body and reads the stamped identifiers.
func (f *facturama) stampRaw(ctx context.Context, body any) (cfdiStamped, error) {
	raw, err := f.do(ctx, http.MethodPost, "/api-lite/3/cfdis", body)
	if err != nil {
		return cfdiStamped{}, err
	}
	var out struct {
		Id         string
		Complement struct{ TaxStamp struct{ Uuid string } }
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.Id == "" || out.Complement.TaxStamp.Uuid == "" {
		return cfdiStamped{}, errors.New("el servicio de timbrado devolvió una respuesta inesperada")
	}
	return cfdiStamped{ProviderID: out.Id, UUID: strings.ToUpper(out.Complement.TaxStamp.Uuid)}, nil
}

func (f *facturama) Fetch(ctx context.Context, providerID, format string) ([]byte, error) {
	if format != "xml" && format != "pdf" {
		return nil, errors.New("formato inválido")
	}
	raw, err := f.do(ctx, http.MethodGet, "/cfdi/"+format+"/issuedLite/"+url.PathEscape(providerID), nil)
	if err != nil {
		return nil, err
	}
	var out struct{ Content string }
	if err := json.Unmarshal(raw, &out); err != nil || out.Content == "" {
		return nil, errors.New("el servicio de timbrado devolvió un archivo vacío")
	}
	return base64.StdEncoding.DecodeString(out.Content)
}

func (f *facturama) Cancel(ctx context.Context, providerID, motive, replacementUUID string) error {
	q := url.Values{"type": {"issuedLite"}, "motive": {motive}}
	if replacementUUID != "" {
		q.Set("uuidReplacement", replacementUUID)
	}
	_, err := f.do(ctx, http.MethodDelete, "/cfdi/"+url.PathEscape(providerID)+"?"+q.Encode(), nil)
	return err
}
