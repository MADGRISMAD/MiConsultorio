package api

import (
	"context"
	"encoding/xml"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Directorio público, como el de Doctoralia: el paciente busca por especialidad, estado, ciudad o nombre; cada
// resultado trae su calificación (de encuestas ligadas a citas reales, o sea verificadas), sus áreas y el próximo
// horario libre; de ahí entra al perfil o agenda. Aparecen todos los consultorios con la suscripción vigente (salvo
// los que se ocultan); los que completan su perfil suben y los que tienen datos faltantes bajan (ver profileScoreSQL).

// mxStates son los 32 estados; el perfil solo acepta uno de estos.
var mxStates = []string{
	"Aguascalientes", "Baja California", "Baja California Sur", "Campeche", "Chiapas", "Chihuahua", "Ciudad de México",
	"Coahuila", "Colima", "Durango", "Estado de México", "Guanajuato", "Guerrero", "Hidalgo", "Jalisco", "Michoacán",
	"Morelos", "Nayarit", "Nuevo León", "Oaxaca", "Puebla", "Querétaro", "Quintana Roo", "San Luis Potosí", "Sinaloa",
	"Sonora", "Tabasco", "Tamaulipas", "Tlaxcala", "Veracruz", "Yucatán", "Zacatecas",
}

// areaSlugs da direcciones legibles para el directorio y el mapa del sitio (/directorio/dentistas/tijuana).
var areaSlugs = map[string]string{
	"GENERAL_MEDICAL": "medicos-generales", "DENTAL": "dentistas", "PEDIATRICS": "pediatras", "INTERNAL_MEDICINE": "internistas",
	"PHYSIOTHERAPY": "fisioterapeutas", "NUTRITION": "nutriologos", "PSYCHOLOGY": "psicologos", "DERMATOLOGY": "dermatologos",
	"GYNECOLOGY": "ginecologos", "ORTHOPEDICS": "ortopedistas", "VETERINARY": "veterinarios", "CHIROPRACTIC": "quiropracticos",
}

// paymentMethods son las formas de pago que un perfil puede declarar.
var paymentMethods = []string{"Efectivo", "Tarjeta de débito", "Tarjeta de crédito", "Transferencia", "Mercado Pago", "Vales"}

const directoryPageSize = 12

// placeSlug: "San Luis Potosí" → "san-luis-potosi".
func placeSlug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range norm.NFD.String(strings.ToLower(strings.TrimSpace(s))) {
		switch {
		case unicode.Is(unicode.Mn, r):
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			dash = false
		default:
			if b.Len() > 0 && !dash {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// nextSlotCache guarda el próximo horario libre de cada consultorio unos minutos: el directorio lo pide seguido.
// ponytail: en memoria y por proceso; si un día hay varias instancias, cada una calcula el suyo.
var nextSlotCache = struct {
	sync.Mutex
	m map[string]nextSlotEntry
}{m: map[string]nextSlotEntry{}}

type nextSlotEntry struct {
	val *nextSlot
	exp time.Time
}

type nextSlot struct {
	Date         string `json:"date"`
	Start        string `json:"start"`
	Professional string `json:"professional"`
}

// nextFreeSlot busca el primer horario libre (cualquier especialista) en los próximos 14 días. La caché va por id
// del consultorio (el enlace puede cambiar o repetirse entre bases distintas).
func (s *Server) nextFreeSlot(ctx context.Context, clinicID, slug string) *nextSlot {
	nextSlotCache.Lock()
	if e, ok := nextSlotCache.m[clinicID]; ok && time.Now().Before(e.exp) {
		nextSlotCache.Unlock()
		return e.val
	}
	nextSlotCache.Unlock()
	var found *nextSlot
	c, err := s.loadBookingClinic(ctx, slug)
	if err == nil {
		pros, err := s.listBookable(ctx, c, "")
		now := time.Now()
		days := min(c.HorizonDays, 14)
		for d := 0; err == nil && found == nil && d <= days; d++ {
			date := now.In(c.Loc).AddDate(0, 0, d).Format("2006-01-02")
			for _, p := range pros {
				pid := p.ID
				held, herr := s.slotHeldSpans(ctx, s.db, c.ID, pid, date, "")
				if herr != nil {
					break
				}
				for _, st := range c.candidateSlots(p, date, now, 0) {
					end := p.end(c, st, 0)
					code, cerr := s.slotConflict(ctx, s.db, c.ID, &pid, "", date, st, end, "")
					if cerr == nil && code == SlotFree && !held.overlaps(st, end) {
						found = &nextSlot{Date: date, Start: st, Professional: p.Name}
						break
					}
				}
				if found != nil {
					break
				}
			}
		}
	}
	nextSlotCache.Lock()
	nextSlotCache.m[clinicID] = nextSlotEntry{val: found, exp: time.Now().Add(5 * time.Minute)}
	nextSlotCache.Unlock()
	return found
}

type directoryHit struct {
	Slug       string      `json:"slug"`
	Name       string      `json:"name"`
	Tagline    string      `json:"tagline"`
	City       string      `json:"city"`
	State      string      `json:"state"`
	Areas      []string    `json:"areas"`
	PhotoURL   string      `json:"photo_url"`
	CoverURL   string      `json:"cover_url"`
	Rating     surveyStats `json:"rating"`
	Booking    bool        `json:"booking"`
	NextSlot   *nextSlot   `json:"next_slot"`
	PriceFrom  int         `json:"price_from_cents"` // el servicio público más barato (0 = no publica precios)
	Insurances []string    `json:"insurances"`
	// HasPage: tiene página pública a la que entrar; sin ella la tarjeta solo informa.
	HasPage bool `json:"has_page"`
	// Completeness (0–100): qué tanto llenó su perfil; ordena el directorio.
	Completeness int `json:"completeness"`
}

// Todos los consultorios con suscripción vigente están en el directorio, tengan o no perfil (por eso los LEFT JOIN):
// directoryBase es ese conjunto; locatedBase los que indicaron ciudad y estado (los filtros por lugar); pageBase
// los que además tienen página pública con enlace (el mapa del sitio solo lista esos).
const (
	directoryFrom = `
	FROM clinics c LEFT JOIN clinic_profile cp ON cp.clinic_id = c.id LEFT JOIN agenda_settings a ON a.clinic_id = c.id`
	directoryBase = directoryFrom + `
	WHERE c.billing_status IN ('active', 'trialing', 'past_due') AND c.branch_suspended_at IS NULL AND cp.directory_hidden IS NOT TRUE`
	locatedBase = directoryBase + ` AND cp.city <> '' AND cp.state <> ''`
	pageBase    = locatedBase + ` AND cp.enabled AND a.booking_slug <> ''`
)

// profileCriteria es lo que puntúa el perfil (suman 100). Pesa más lo que le sirve al paciente para decidir y agendar
// (página, reservas, ubicación, servicios con precio) que los detalles. Es la única fuente: de aquí salen el puntaje
// con el que se ordena el directorio (profileScoreSQL) y la lista de pendientes que ve el consultorio.
// ponytail: pesos fijos en código; si se quieren ajustar sin desplegar, pasarlos a una tabla de configuración.
var profileCriteria = []struct {
	Label  string
	Points int
	Cond   string // SQL sobre c (clinics), cp (clinic_profile) y a (agenda_settings)
}{
	{"Publica tu página del consultorio", 15, `cp.enabled`},
	{"Activa las reservas en línea", 15, `a.booking_enabled AND a.booking_slug <> ''`},
	{"Indica tu estado y ciudad", 15, `cp.city <> '' AND cp.state <> ''`},
	{"Muestra al menos un servicio con su precio", 10, `EXISTS (SELECT 1 FROM catalog_items i WHERE i.clinic_id = c.id AND i.kind = 'service' AND i.public AND i.active AND i.price_cents > 0)`},
	{"Cuenta sobre tu consultorio (40 letras o más)", 8, `length(cp.about) >= 40`},
	{"Sube la foto de perfil", 8, `EXISTS (SELECT 1 FROM clinic_media m WHERE m.clinic_id = c.id AND m.slot = 'profile')`},
	{"Agrega WhatsApp, correo o sitio web", 5, `cp.whatsapp <> '' OR cp.contact_email <> '' OR cp.website <> ''`},
	{"Escribe una frase corta", 4, `cp.tagline <> ''`},
	{"Escribe tu horario", 4, `cp.hours_text <> ''`},
	{"Sube una imagen de portada", 4, `EXISTS (SELECT 1 FROM clinic_media m WHERE m.clinic_id = c.id AND m.slot = 'cover')`},
	{"Indica tus formas de pago", 3, `cardinality(cp.payment_methods) > 0`},
	{"Captura la cédula de un especialista", 3, `EXISTS (SELECT 1 FROM users u WHERE u.clinic_id = c.id AND u.role IN ('admin', 'doctor') AND NOT u.disabled AND NOT u.public_hidden AND u.cedula <> '')`},
	{"Agrega fotos a la galería", 2, `EXISTS (SELECT 1 FROM clinic_media m WHERE m.clinic_id = c.id AND m.slot = 'gallery')`},
	{"Indica las aseguradoras que aceptas", 2, `cardinality(cp.insurances) > 0`},
	{"Escribe la semblanza de un especialista", 2, `EXISTS (SELECT 1 FROM users u WHERE u.clinic_id = c.id AND u.role IN ('admin', 'doctor') AND NOT u.disabled AND NOT u.public_hidden AND u.public_bio <> '')`},
}

// profileScoreSQL suma los puntos de los criterios cumplidos: entre más datos faltan, menos puntos. El directorio
// ordena por tramos de 10 puntos y, dentro del tramo, por calificación.
var profileScoreSQL = func() string {
	parts := make([]string, len(profileCriteria))
	for i, c := range profileCriteria {
		parts[i] = "(CASE WHEN " + c.Cond + " THEN " + strconv.Itoa(c.Points) + " ELSE 0 END)"
	}
	return "(" + strings.Join(parts, " + ") + ")"
}()

type profileCheck struct {
	Label  string `json:"label"`
	Points int    `json:"points"`
	Done   bool   `json:"done"`
}

// profileChecklist dice a un consultorio qué ha completado y qué le falta para subir en el directorio.
func (s *Server) profileChecklist(ctx context.Context, clinicID string) (score int, items []profileCheck, err error) {
	conds := make([]string, len(profileCriteria))
	for i, c := range profileCriteria {
		conds[i] = "coalesce((" + c.Cond + "), false)"
	}
	done := make([]bool, len(profileCriteria))
	dest := make([]any, len(done))
	for i := range done {
		dest[i] = &done[i]
	}
	if err = s.db.QueryRow(ctx, `SELECT `+strings.Join(conds, ", ")+directoryFrom+` WHERE c.id = $1`, clinicID).Scan(dest...); err != nil {
		return 0, nil, err
	}
	for i, c := range profileCriteria {
		items = append(items, profileCheck{c.Label, c.Points, done[i]})
		if done[i] {
			score += c.Points
		}
	}
	return score, items, nil
}

func (s *surveyPublic) directory(w http.ResponseWriter, r *http.Request) {
	if !limit(w, s.reads, "directory|"+clientIP(r)) {
		return
	}
	ctx := r.Context()
	q := r.URL.Query()
	text := strings.TrimSpace(q.Get("q"))
	if len([]rune(text)) > 60 {
		text = string([]rune(text)[:60])
	}
	area := q.Get("area")
	if area != "" && areaLabels[area] == "" {
		area = ""
	}
	state := q.Get("state")
	if state != "" && !slices.Contains(mxStates, state) {
		state = ""
	}
	// la ciudad llega como texto o como su forma de URL ("tijuana"); se compara por la forma de URL
	city := placeSlug(q.Get("city"))
	page, _ := strconv.Atoi(q.Get("page"))
	page = max(1, min(page, 50))

	where := directoryBase + `
	  AND ($1 = '' OR cp.state = $1)
	  AND ($2 = '' OR c.kind = $2 OR $2 = ANY(c.specialties))
	  AND ($3 = '' OR c.name ILIKE '%' || $3 || '%' OR cp.tagline ILIKE '%' || $3 || '%' OR EXISTS (
	        SELECT 1 FROM users u WHERE u.clinic_id = c.id AND u.role IN ('admin', 'doctor') AND NOT u.disabled AND NOT u.public_hidden
	          AND (u.name ILIKE '%' || $3 || '%' OR u.specialty_title ILIKE '%' || $3 || '%')))`
	args := []any{state, area, likeEscape(text)}

	// La ciudad se compara ya normalizada (sin acentos ni mayúsculas): se resuelve aquí a los nombres guardados.
	if city != "" {
		names, err := s.citiesForSlug(ctx, city)
		if err != nil {
			serverError(w, r, err)
			return
		}
		if len(names) == 0 {
			writeJSON(w, http.StatusOK, map[string]any{"results": []directoryHit{}, "total": 0, "page": page, "page_size": directoryPageSize})
			return
		}
		where += ` AND cp.city = ANY($4)`
		args = append(args, names)
	}

	var total int
	if err := s.db.QueryRow(ctx, `SELECT count(*) `+where, args...).Scan(&total); err != nil {
		serverError(w, r, err)
		return
	}
	// el promedio de calificaciones y el puntaje del perfil van en LATERAL para poder ordenar por ellos
	ranked := directoryFrom + `
	LEFT JOIN LATERAL (
			SELECT count(*) AS n, avg(x.rating) AS avg FROM satisfaction_surveys x
			WHERE x.clinic_id = c.id AND x.answered_at IS NOT NULL AND x.rating IS NOT NULL) st ON true
	CROSS JOIN LATERAL (SELECT ` + profileScoreSQL + ` AS score) sc` + strings.TrimPrefix(where, directoryFrom)
	rows, err := s.db.Query(ctx, `
		SELECT c.id::text, lower(coalesce(a.booking_slug, '')), c.name, coalesce(cp.tagline, ''), coalesce(cp.city, ''), coalesce(cp.state, ''),
		       coalesce(a.booking_enabled, false), coalesce(cp.insurances, '{}'), coalesce(cp.enabled, false), sc.score,
		       coalesce((SELECT min(i.price_cents) FROM catalog_items i WHERE i.clinic_id = c.id AND i.kind = 'service' AND i.public AND i.active AND i.price_cents > 0), 0)
		`+ranked+`
		ORDER BY sc.score / 10 DESC, (st.n > 0) DESC, st.avg DESC NULLS LAST, st.n DESC, sc.score DESC, c.name
		LIMIT `+strconv.Itoa(directoryPageSize)+` OFFSET $`+strconv.Itoa(len(args)+1), append(args, (page-1)*directoryPageSize)...)
	if err != nil {
		serverError(w, r, err)
		return
	}
	type row struct {
		id string
		h  directoryHit
	}
	var list []row
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.id, &x.h.Slug, &x.h.Name, &x.h.Tagline, &x.h.City, &x.h.State, &x.h.Booking, &x.h.Insurances, &x.h.HasPage, &x.h.Completeness, &x.h.PriceFrom); err != nil {
			rows.Close()
			serverError(w, r, err)
			return
		}
		list = append(list, x)
	}
	rows.Close()
	out := make([]directoryHit, 0, len(list))
	for _, x := range list {
		h := x.h
		h.HasPage = h.HasPage && h.Slug != "" // sin enlace no hay a dónde entrar
		if kinds, err := s.clinicKindsFor(ctx, x.id); err == nil {
			for _, k := range kinds {
				if l := areaLabels[k]; l != "" {
					h.Areas = append(h.Areas, l)
				}
			}
		}
		if !h.HasPage {
			h.Slug = "" // sin página publicada no se expone nada de ella
		}
		if prof, cover, _, err := s.publicMedia(ctx, x.id); err == nil && h.HasPage {
			if prof != nil {
				h.PhotoURL = mediaURL(h.Slug, *prof)
			}
			if cover != nil {
				h.CoverURL = mediaURL(h.Slug, *cover)
			}
		}
		if st, err := s.surveyStatsFor(ctx, x.id, nil, nil); err == nil {
			h.Rating = st
		}
		if h.Booking && h.HasPage {
			h.NextSlot = s.nextFreeSlot(ctx, x.id, h.Slug)
		}
		if h.Insurances == nil {
			h.Insurances = []string{}
		}
		out = append(out, h)
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": out, "total": total, "page": page, "page_size": directoryPageSize})
}

// citiesForSlug devuelve los nombres de ciudad guardados que corresponden a una forma de URL.
func (s *Server) citiesForSlug(ctx context.Context, slug string) ([]string, error) {
	rows, err := s.db.Query(ctx, `SELECT DISTINCT cp.city `+locatedBase)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		if placeSlug(c) == slug {
			out = append(out, c)
		}
	}
	return out, rows.Err()
}

// directoryOptions da los filtros: especialidades (con su forma de URL), estados y las ciudades que sí tienen consultorios.
func (s *surveyPublic) directoryOptions(w http.ResponseWriter, r *http.Request) {
	if !limit(w, s.reads, "directory|"+clientIP(r)) {
		return
	}
	areas := make([]map[string]string, 0, len(areaLabels))
	for code, label := range areaLabels {
		areas = append(areas, map[string]string{"code": code, "label": label, "slug": areaSlugs[code]})
	}
	sort.Slice(areas, func(i, j int) bool { return areas[i]["label"] < areas[j]["label"] })
	rows, err := s.db.Query(r.Context(), `SELECT cp.state, cp.city, count(*) `+locatedBase+` GROUP BY 1, 2 ORDER BY 3 DESC, 2`)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	cities := []map[string]any{}
	for rows.Next() {
		var st, city string
		var n int
		if err := rows.Scan(&st, &city, &n); err != nil {
			serverError(w, r, err)
			return
		}
		cities = append(cities, map[string]any{"state": st, "city": city, "slug": placeSlug(city), "count": n})
	}
	writeJSON(w, http.StatusOK, map[string]any{"areas": areas, "states": mxStates, "cities": cities, "payment_methods": paymentMethods})
}

// sitemap lista para Google el directorio, cada combinación especialidad + ciudad con resultados y cada perfil.
func (s *surveyPublic) sitemap(w http.ResponseWriter, r *http.Request) {
	if !limit(w, s.reads, "sitemap|"+clientIP(r)) {
		return
	}
	base := strings.TrimRight(s.cfg.AppURL, "/")
	type u struct {
		Loc string `xml:"loc"`
	}
	urls := []u{{base + "/directorio"}}
	rows, err := s.db.Query(r.Context(), `SELECT lower(a.booking_slug), cp.city, c.kind, c.specialties `+pageBase+` ORDER BY 1`)
	if err != nil {
		serverError(w, r, err)
		return
	}
	seen := map[string]bool{}
	var profiles []string
	for rows.Next() {
		var slug, city, kind string
		var specs []string
		if err := rows.Scan(&slug, &city, &kind, &specs); err != nil {
			rows.Close()
			serverError(w, r, err)
			return
		}
		profiles = append(profiles, base+"/"+slug)
		for _, k := range clinicKindsOf(kind, specs) {
			if a := areaSlugs[k]; a != "" {
				for _, p := range []string{"/directorio/" + a, "/directorio/" + a + "/" + placeSlug(city)} {
					if !seen[p] {
						seen[p] = true
						urls = append(urls, u{base + p})
					}
				}
			}
		}
	}
	rows.Close()
	for _, p := range profiles {
		urls = append(urls, u{p})
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(struct {
		XMLName xml.Name `xml:"urlset"`
		NS      string   `xml:"xmlns,attr"`
		URLs    []u      `xml:"url"`
	}{NS: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: urls})
}
