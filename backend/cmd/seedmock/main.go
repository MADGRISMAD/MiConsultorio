// Command seedmock fills a clinic with example data of the last weeks (patients, appointments, notes, prescriptions,
// register sales) so the panel, reports and the home can be seen working.
//
//	seedmock -email you@clinic.com            # shows the plan, writes nothing
//	seedmock -email you@clinic.com -yes       # creates the data
//	seedmock -email you@clinic.com -undo -yes # removes what a previous run created
//
// Everything is created through the real API in-process (same validation, encryption and rules as the app), as the
// account with that e-mail, and then moved back in time so it spreads over the last weeks. Example patients have no
// e-mail, so no reminder, survey or greeting is ever sent for them; they are recognised by a phone starting with
// 5500000 and the address "Dato de ejemplo", which is also how -undo finds them.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/madgrismad/miconsultorio/backend/internal/api"
	"github.com/madgrismad/miconsultorio/backend/internal/config"
	"github.com/madgrismad/miconsultorio/backend/internal/db"
)

const (
	phonePrefix = "5500000"
	markAddress = "Dato de ejemplo"
	skuPrefix   = "MOCK-"
	cashNote    = "Dato de ejemplo"
)

type seeder struct {
	c      *api.ToolClient
	pool   *pgxpool.Pool
	rnd    *rand.Rand
	loc    *time.Location
	now    time.Time
	days   int
	pro    string // professional that sees the patients (the signed-in user when they consult)
	failed map[string]int
}

func main() {
	email := flag.String("email", "", "e-mail of the account whose clinic is filled (required)")
	days := flag.Int("days", 35, "how many days back the history reaches")
	seed := flag.Int64("seed", 7, "random seed (same seed, same data)")
	undo := flag.Bool("undo", false, "remove the example data a previous run created")
	yes := flag.Bool("yes", false, "really write (without it only the plan is shown)")
	flag.Parse()
	config.LoadDotEnv()
	if *email == "" {
		flag.Usage()
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()
	c, err := api.NewToolClient(ctx, pool, cfg, *email)
	if err != nil {
		log.Fatal(err)
	}
	var clinic string
	var tz string
	_ = pool.QueryRow(ctx, `SELECT name, coalesce(settings->>'timezone', '') FROM clinics WHERE id = $1`, c.ClinicID).Scan(&clinic, &tz)
	loc, err := time.LoadLocation(map[bool]string{true: tz, false: "America/Mexico_City"}[tz != ""])
	if err != nil {
		loc = time.FixedZone("CST", -6*3600)
	}
	s := &seeder{c: c, pool: pool, rnd: rand.New(rand.NewSource(*seed)), loc: loc, now: time.Now().In(loc), days: *days, failed: map[string]int{}}

	var existing int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM patients WHERE clinic_id = $1 AND phone LIKE $2`, c.ClinicID, phonePrefix+"%").Scan(&existing)
	fmt.Printf("Consultorio: %s (%s)\n", clinic, *email)
	switch {
	case *undo && !*yes:
		fmt.Printf("Se borrarían %d pacientes de ejemplo con sus citas, notas, recetas y ventas. Agrega -yes para hacerlo.\n", existing)
		return
	case *undo:
		s.undo(ctx)
		return
	case !*yes:
		fmt.Printf("Plan: ~40 pacientes, ~%d citas (completadas, no asistió, canceladas y próximas), notas de consulta, recetas, medicación crónica, estudios de laboratorio, lista de espera, bloqueos de agenda y cortes de caja de los últimos %d días.\n", *days*4, *days)
		fmt.Printf("Ya hay %d pacientes de ejemplo. Agrega -yes para crear los datos (y -undo -yes para quitarlos).\n", existing)
		return
	case existing > 0:
		log.Fatalf("este consultorio ya tiene %d pacientes de ejemplo; quítalos primero con -undo -yes", existing)
	}
	s.run(ctx)
}

// ---------------------------------------------------------------------------------------------------------------

func (s *seeder) must(method, path string, body, out any) bool {
	_, err := s.c.Do(method, path, body, out)
	if err != nil {
		key := method + " " + strings.SplitN(path, "?", 2)[0]
		if s.failed[key]++; s.failed[key] <= 2 {
			fmt.Printf("  aviso: %s %s: %v\n", method, path, err)
		}
		return false
	}
	return true
}

func (s *seeder) chance(p float64) bool { return s.rnd.Float64() < p }
func pick[T any](s *seeder, l []T) T    { return l[s.rnd.Intn(len(l))] }

// at is the clinic-local instant of `minutes` after midnight of that day.
func (s *seeder) at(day time.Time, minutes int) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), 0, minutes, 0, 0, s.loc)
}
func hhmm(m int) string { return fmt.Sprintf("%02d:%02d", m/60, m%60) }

type patient struct {
	ID, Names, Last string
	firstSeen       time.Time
}

type visit struct {
	ID        string
	P         *patient
	Day       time.Time
	Start     int
	End       int
	Status    string // final: completed | no_show | cancelled | scheduled | confirmed | arrived | in_progress
	Encounter string
	Rx        string
}

func (s *seeder) run(ctx context.Context) {
	fmt.Println("Creando datos de ejemplo…")
	// who attends: the signed-in user when they consult, otherwise the first professional that does
	var pros struct {
		Professionals []struct {
			ID       string `json:"id"`
			Consults bool   `json:"consults"`
		} `json:"professionals"`
	}
	if !s.must("GET", "/agenda/professionals", nil, &pros) || len(pros.Professionals) == 0 {
		log.Fatal("el consultorio no tiene profesionales que atiendan")
	}
	for _, p := range pros.Professionals {
		if p.Consults && (s.pro == "" || p.ID == s.c.UserID) {
			s.pro = p.ID
		}
	}
	if s.pro == "" {
		s.pro = pros.Professionals[0].ID
	}

	services := s.catalog()
	patients := s.patients(40)
	visits := s.appointments(patients)
	s.notes(visits)
	s.register(visits, services)
	s.extras(patients)
	s.tidy(ctx, patients, visits)

	fmt.Println("Listo. Resumen:")
	for _, t := range []string{"patients", "appointments", "encounters", "prescriptions", "sales", "cash_sessions", "chronic_medications", "lab_orders", "waitlist_entries", "time_blocks"} {
		var n int
		_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM `+t+` WHERE clinic_id = $1`, s.c.ClinicID).Scan(&n)
		fmt.Printf("  %-14s %d\n", t, n)
	}
	for k, n := range s.failed {
		fmt.Printf("  (%d fallos en %s)\n", n, k)
	}
}

// ---------------------------------------------------------------------------------------------------------------
// catalog

type item struct {
	ID    string `json:"id"`
	Name  string
	Price int
}

func (s *seeder) catalog() []item {
	defs := []struct {
		name  string
		price int
		kind  string
	}{
		{"Consulta general", 80000, "service"}, {"Consulta de seguimiento", 60000, "service"}, {"Limpieza y revisión", 90000, "service"},
		{"Aplicación de inyección", 25000, "service"}, {"Curación", 35000, "service"}, {"Certificado médico", 30000, "service"},
		{"Vitaminas (frasco)", 28000, "product"}, {"Kit de curación", 18000, "product"},
	}
	var out []item
	for i, d := range defs {
		var r struct {
			Item struct {
				ID string `json:"id"`
			} `json:"item"`
		}
		if s.must("POST", "/pos/items", map[string]any{
			"kind": d.kind, "name": d.name, "sku": fmt.Sprintf("%s%02d", skuPrefix, i+1), "barcode": "", "category": map[string]string{"service": "Consultas", "product": "Productos"}[d.kind],
			"price_cents": d.price, "cost_cents": 0, "tax_rate": 0, "track_stock": false, "min_stock": 0, "unit": map[string]string{"service": "servicio", "product": "pieza"}[d.kind],
		}, &r) {
			out = append(out, item{ID: r.Item.ID, Name: d.name, Price: d.price})
		}
	}
	fmt.Printf("  catálogo: %d conceptos\n", len(out))
	return out
}

// ---------------------------------------------------------------------------------------------------------------
// patients

var firstNames = []string{"María", "José", "Guadalupe", "Juan", "Carmen", "Luis", "Ana", "Miguel", "Rosa", "Carlos", "Laura", "Jorge", "Patricia", "Fernando", "Sofía", "Ricardo", "Daniela", "Alejandro", "Verónica", "Héctor", "Lucía", "Andrés", "Mariana", "Raúl"}
var lastNames = []string{"Hernández", "García", "Martínez", "López", "González", "Rodríguez", "Pérez", "Sánchez", "Ramírez", "Flores", "Gómez", "Torres", "Díaz", "Vázquez", "Morales", "Reyes", "Cruz", "Ortiz", "Gutiérrez", "Chávez", "Ramos", "Mendoza", "Aguilar", "Castillo"}
var allergies = []string{"Ninguna conocida", "Ninguna conocida", "Ninguna conocida", "Polen y ácaros", "Intolerancia a la lactosa", "Alergia al látex"}

func (s *seeder) patients(n int) []*patient {
	var out []*patient
	for i := 0; i < n; i++ {
		first, l1, l2 := pick(s, firstNames), pick(s, lastNames), pick(s, lastNames)
		sex := "Mujer"
		if strings.HasSuffix(first, "o") || first == "José" || first == "Luis" || first == "Juan" || first == "Jorge" || first == "Andrés" || first == "Raúl" || first == "Héctor" {
			sex = "Hombre"
		}
		born := time.Date(1950+s.rnd.Intn(55), time.Month(1+s.rnd.Intn(12)), 1+s.rnd.Intn(28), 0, 0, 0, 0, time.UTC)
		var r struct {
			Patient struct {
				ID string `json:"id"`
			} `json:"patient"`
		}
		if !s.must("POST", "/patients/", map[string]any{
			"subject": "person", "names": first, "last_names": l1 + " " + l2, "sex": sex, "birth_date": born.Format("2006-01-02"), "curp": "",
			"phone": fmt.Sprintf("%s%03d", phonePrefix, i+1), "email": "", "address": markAddress,
			"guardian_name": "", "guardian_relation": "", "guardian_phone": "", "guardian_email": "",
			"profile": map[string]any{"allergies_text": pick(s, allergies)}, "privacy_ack": true, "reminders_ok": false,
		}, &r) {
			continue
		}
		out = append(out, &patient{ID: r.Patient.ID, Names: first, Last: l1 + " " + l2})
	}
	fmt.Printf("  pacientes: %d\n", len(out))
	if len(out) == 0 {
		log.Fatal("no se pudo crear ningún paciente (¿el consultorio atiende solo animales?)")
	}
	return out
}

// ---------------------------------------------------------------------------------------------------------------
// appointments

var reasons = []string{"Revisión general", "Dolor de garganta", "Control de presión", "Seguimiento", "Dolor de espalda", "Resultados de estudios", "Gripe y tos", "Dolor de cabeza", "Revisión de rutina", "Molestias estomacales"}

func (s *seeder) appointments(patients []*patient) []*visit {
	var visits []*visit
	today := time.Date(s.now.Year(), s.now.Month(), s.now.Day(), 0, 0, 0, 0, s.loc)
	nowMin := s.now.Hour()*60 + s.now.Minute()
	hasOpen := map[string]bool{} // a patient may only have one open appointment at a time
	for d := -s.days; d <= 10; d++ {
		day := today.AddDate(0, 0, d)
		if day.Weekday() == time.Sunday {
			continue
		}
		last := 17*60 + 20
		if day.Weekday() == time.Saturday {
			last = 13*60 + 20
		}
		for start := 9 * 60; start <= last; start += 40 {
			fill := 0.42
			if d > 0 {
				fill = 0.4 - float64(d)*0.025
			}
			if !s.chance(fill) {
				continue
			}
			p := patients[s.rnd.Intn(len(patients))]
			past := d < 0 || (d == 0 && start+40 <= nowMin)
			if !past && hasOpen[p.ID] {
				continue
			}
			v := &visit{P: p, Day: day, Start: start, End: start + 30}
			var r struct {
				Appointment struct {
					ID string `json:"id"`
				} `json:"appointment"`
			}
			if !s.must("POST", "/appointments/", map[string]any{
				"patient_id": p.ID, "names": p.Names, "last_names": p.Last, "CURP": "", "date": day.Format("2006-01-02"), "startHour": hhmm(start), "endHour": hhmm(v.End),
				"details": pick(s, reasons), "professional_id": s.pro, "service_id": nil, "room": "", "phone": "", "email": "", "reminders_consent": false, "overbook": false,
			}, &r) {
				continue
			}
			v.ID = r.Appointment.ID
			visits = append(visits, v)
			if p.firstSeen.IsZero() || day.Before(p.firstSeen) {
				p.firstSeen = day
			}
			switch {
			case past:
				switch x := s.rnd.Float64(); {
				case x < 0.78:
					v.Status = "completed"
				case x < 0.86:
					v.Status = "no_show"
				default:
					v.Status = "cancelled"
				}
			case d == 0 && start <= nowMin+30 && start+30 > nowMin:
				v.Status = "in_progress"
			case d == 0 && start <= nowMin+90:
				v.Status = "arrived"
			default:
				v.Status = map[bool]string{true: "confirmed", false: "scheduled"}[s.chance(0.5)]
			}
			if v.Status == "scheduled" || v.Status == "confirmed" || v.Status == "arrived" || v.Status == "in_progress" {
				hasOpen[p.ID] = true
			}
		}
	}
	fmt.Printf("  citas: %d\n", len(visits))
	return visits
}

// ---------------------------------------------------------------------------------------------------------------
// notes (bitácora), prescriptions and status of each appointment

type condition struct {
	reason, subj, exam, assess, plan, dx string
	codes                                []string
	rx                                   [][5]string // medicine, presentation, dose, frequency, duration
}

var conditions = []condition{
	{"Gripe y tos", "Refiere congestión nasal, tos seca y malestar general desde hace tres días.", "Faringe hiperémica, sin exudado. Campos pulmonares limpios.", "Infección de vías respiratorias altas", "Reposo, líquidos abundantes y vigilancia de fiebre.", "Infección respiratoria aguda", []string{"J06.9"},
		[][5]string{{"Paracetamol", "Tabletas 500 mg", "1 tableta", "Cada 8 horas", "3 días"}}},
	{"Dolor de garganta", "Odinofagia y fiebre de 38 °C desde ayer.", "Amígdalas hipertróficas con placas, adenopatía cervical sensible.", "Faringoamigdalitis aguda", "Antibiótico por siete días y analgésico si hay dolor.", "Faringoamigdalitis", []string{"J02.9"},
		[][5]string{{"Amoxicilina", "Cápsulas 500 mg", "1 cápsula", "Cada 8 horas", "7 días"}, {"Ibuprofeno", "Tabletas 400 mg", "1 tableta", "Cada 8 horas con alimento", "3 días"}}},
	{"Control de presión", "Acude a control, sin mareos ni dolor de cabeza. Toma su medicamento a diario.", "Tensión arterial en rango, sin edema.", "Hipertensión arterial controlada", "Continuar tratamiento, dieta baja en sal y caminata diaria.", "Hipertensión controlada", []string{"I10"},
		[][5]string{{"Losartán", "Tabletas 50 mg", "1 tableta", "Cada 24 horas", "30 días"}}},
	{"Dolor de espalda", "Dolor lumbar al cargar peso, sin irradiación.", "Contractura paravertebral lumbar, movilidad conservada.", "Lumbalgia mecánica", "Calor local, estiramientos y analgésico por tres días.", "Lumbalgia", []string{"M54.5"},
		[][5]string{{"Naproxeno", "Tabletas 550 mg", "1 tableta", "Cada 12 horas con alimento", "5 días"}}},
	{"Dolor de cabeza", "Cefalea frontal de predominio vespertino, asociada a estrés.", "Sin datos de alarma neurológica.", "Cefalea tensional", "Higiene del sueño, hidratación y pausas activas.", "Cefalea tensional", []string{"G44.2"},
		[][5]string{{"Paracetamol", "Tabletas 500 mg", "1 tableta", "Cada 8 horas si hay dolor", "3 días"}}},
	{"Molestias estomacales", "Ardor y acidez después de comer, sin vómito.", "Abdomen blando, dolor leve a la palpación en epigastrio.", "Gastritis", "Dieta blanda, evitar irritantes y protector gástrico.", "Gastritis", []string{"K29.7"},
		[][5]string{{"Omeprazol", "Cápsulas 20 mg", "1 cápsula", "En ayunas", "14 días"}}},
	{"Revisión general", "Acude a chequeo anual, asintomático.", "Exploración sin hallazgos relevantes.", "Paciente sano", "Mantener hábitos saludables y repetir chequeo en un año.", "Chequeo general", []string{"Z00.0"}, nil},
	{"Seguimiento", "Evolución favorable desde la última consulta.", "Sin datos de complicación.", "Evolución satisfactoria", "Continuar manejo indicado y regresar si hay cambios.", "Seguimiento", []string{"Z09"}, nil},
}

func (s *seeder) notes(visits []*visit) {
	cédula := true
	notes, rxs := 0, 0
	for _, v := range visits {
		switch v.Status {
		case "completed":
			// the usual path: the patient arrives and is seen; the note both documents and closes the appointment
			s.status(v, "arrived")
			s.status(v, "in_progress")
			if !s.chance(0.82) {
				s.status(v, "completed")
				continue
			}
			cond := pick(s, conditions)
			in := map[string]any{
				"kind": "consulta", "appointment_id": v.ID, "reason": cond.reason, "subjective": cond.subj, "exam": cond.exam, "assessment": cond.assess,
				"diagnosis_codes": cond.codes, "plan": cond.plan, "occurred_at": nil,
			}
			if s.chance(0.35) {
				in["next_visit"] = v.Day.AddDate(0, 0, 14+s.rnd.Intn(20)).Format("2006-01-02")
			}
			var r struct {
				Encounter struct {
					ID string `json:"id"`
				} `json:"encounter"`
			}
			if !s.must("POST", "/patients/"+v.P.ID+"/encounters", in, &r) {
				s.status(v, "completed")
				continue
			}
			v.Encounter = r.Encounter.ID
			notes++
			if cédula && len(cond.rx) > 0 && s.chance(0.7) {
				items := make([]map[string]string, 0, len(cond.rx))
				for _, m := range cond.rx {
					items = append(items, map[string]string{"medicine": m[0], "presentation": m[1], "dose": m[2], "route": "Oral", "frequency": m[3], "duration": m[4], "quantity": "1 caja"})
				}
				var rr struct {
					Prescription struct {
						ID string `json:"id"`
					} `json:"prescription"`
				}
				_, err := s.c.Do("POST", "/patients/"+v.P.ID+"/prescriptions", map[string]any{
					"encounter_id": v.Encounter, "diagnosis": cond.dx, "items": items, "instructions": "Tomar con abundante agua. Acudir si hay fiebre o empeora.", "valid_days": 30,
				}, &rr)
				var te *api.ToolError
				switch {
				case err == nil:
					v.Rx = rr.Prescription.ID
					rxs++
				case errors.As(err, &te) && te.Code == "CEDULA_REQUIRED":
					cédula = false
					fmt.Println("  recetas omitidas: la cuenta aún no tiene cédula profesional registrada")
				}
			}
		case "no_show", "cancelled":
			s.status(v, v.Status)
		case "confirmed":
			s.status(v, "confirmed")
		case "arrived":
			s.status(v, "arrived")
		case "in_progress":
			s.status(v, "arrived")
			s.status(v, "in_progress")
		}
	}
	fmt.Printf("  notas de consulta: %d, recetas: %d\n", notes, rxs)
}

func (s *seeder) status(v *visit, to string) {
	reason := ""
	if to == "cancelled" {
		reason = "El paciente avisó que no puede asistir."
	}
	s.must("POST", "/appointments/"+v.ID+"/status", map[string]string{"status": to, "reason": reason}, nil)
}

// ---------------------------------------------------------------------------------------------------------------
// register: one cash session per day with the charges of the completed visits

func (s *seeder) register(visits []*visit, services []item) {
	if len(services) == 0 {
		return
	}
	var cur struct {
		Session *struct {
			ID string `json:"id"`
		} `json:"session"`
	}
	if !s.must("GET", "/pos/cash/current", nil, &cur) {
		return
	}
	if cur.Session != nil {
		fmt.Println("  caja omitida: hay una caja abierta ahora; ciérrala y vuelve a correr -undo/-yes si quieres ventas de ejemplo")
		return
	}
	byDay := map[string][]*visit{}
	var days []time.Time
	for _, v := range visits {
		if v.Status != "completed" {
			continue
		}
		k := v.Day.Format("2006-01-02")
		if len(byDay[k]) == 0 {
			days = append(days, v.Day)
		}
		byDay[k] = append(byDay[k], v)
	}
	sales, sessions := 0, 0
	for _, day := range days {
		list := byDay[day.Format("2006-01-02")]
		var open struct {
			Session struct {
				ID string `json:"id"`
			} `json:"session"`
		}
		if !s.must("POST", "/pos/cash/open", map[string]int{"opening_cents": 100000}, &open) {
			return
		}
		sessionID := open.Session.ID
		opened := s.at(day, 8*60+30)
		closed := s.at(day, 19*60)
		saleIDs := []string{}
		for _, v := range list {
			main := services[s.rnd.Intn(3)]
			lines := []map[string]any{{"item_id": main.ID, "qty": 1}}
			total := main.Price
			if s.chance(0.25) {
				extra := services[3+s.rnd.Intn(len(services)-3)]
				lines = append(lines, map[string]any{"item_id": extra.ID, "qty": 1})
				total += extra.Price
			}
			pay := map[string]any{"method": "cash", "amount_cents": total, "received_cents": (total/10000 + 1) * 10000}
			switch x := s.rnd.Float64(); {
			case x >= 0.7 && x < 0.9:
				pay = map[string]any{"method": "card", "amount_cents": total, "reference": fmt.Sprintf("%04d", s.rnd.Intn(10000))}
			case x >= 0.9:
				pay = map[string]any{"method": "transfer", "amount_cents": total, "reference": fmt.Sprintf("SPEI%06d", s.rnd.Intn(1000000))}
			}
			var r struct {
				Sale struct {
					ID string `json:"id"`
				} `json:"sale"`
			}
			if s.must("POST", "/pos/sales", map[string]any{
				"lines": lines, "patient_id": v.P.ID, "professional_id": s.pro, "appointment_id": v.ID,
				"customer_name": v.P.Names + " " + v.P.Last, "payments": []any{pay},
			}, &r) {
				saleIDs = append(saleIDs, r.Sale.ID)
				sales++
				// each sale lands during the visit
				t := s.at(day, v.End+s.rnd.Intn(8))
				s.exec(`UPDATE sales SET created_at = $2 WHERE id = $1`, r.Sale.ID, t)
				s.exec(`UPDATE sale_payments SET created_at = $2 WHERE sale_id = $1`, r.Sale.ID, t)
			}
		}
		var exp struct {
			Session struct {
				Expected int `json:"expected_cents"`
			} `json:"session"`
		}
		s.must("GET", "/pos/cash/current", nil, &exp)
		s.must("POST", "/pos/cash/close", map[string]any{"counted_cents": exp.Session.Expected, "note": cashNote}, nil)
		s.exec(`UPDATE cash_sessions SET opened_at = $2, closed_at = $3, note = $4 WHERE id = $1`, sessionID, opened, closed, cashNote)
		_ = saleIDs
		sessions++
	}
	fmt.Printf("  ventas: %d en %d cortes de caja\n", sales, sessions)
}

func (s *seeder) exec(sql string, args ...any) {
	if _, err := s.pool.Exec(context.Background(), sql, args...); err != nil {
		key := "SQL " + strings.Fields(sql)[1]
		if s.failed[key]++; s.failed[key] <= 2 {
			fmt.Printf("  aviso: %s: %v\n", key, err)
		}
	}
}

// ---------------------------------------------------------------------------------------------------------------
// extras: chronic medication, lab orders, waiting list and agenda blocks

func (s *seeder) extras(patients []*patient) {
	chronic := [][4]string{{"Metformina 850 mg", "1 tableta con la comida", "Cada 12 horas", "Diabetes tipo 2"}, {"Losartán 50 mg", "1 tableta", "Cada 24 horas", "Hipertensión arterial"},
		{"Levotiroxina 100 mcg", "1 tableta en ayunas", "Cada 24 horas", "Hipotiroidismo"}, {"Atorvastatina 20 mg", "1 tableta", "Cada noche", "Colesterol alto"}}
	labs := [][]string{{"Biometría hemática", "Química sanguínea de 6 elementos"}, {"Perfil de lípidos"}, {"Hemoglobina glicosilada", "Glucosa en ayunas"}, {"Examen general de orina"}, {"Perfil tiroideo"}}
	labTitles := []string{"Estudios de control", "Chequeo anual", "Seguimiento de diabetes", "Estudios por infección", "Control de tiroides"}
	meds, orders := 0, 0
	for i, p := range patients {
		if i%6 == 0 {
			c := chronic[(i/6)%len(chronic)]
			start := s.now.AddDate(0, -(2 + s.rnd.Intn(18)), 0)
			if s.must("POST", "/patients/"+p.ID+"/chronic-meds", map[string]any{
				"name": c[0], "dose": c[1], "frequency": c[2], "indication": c[3],
				"started_on": start.Format("2006-01-02"), "next_renewal": s.now.AddDate(0, 0, 5+s.rnd.Intn(40)).Format("2006-01-02"),
			}, nil) {
				meds++
			}
		}
		if i%4 == 1 {
			k := s.rnd.Intn(len(labs))
			when := s.now.AddDate(0, 0, -s.rnd.Intn(s.days))
			in := map[string]any{"title": labTitles[k], "lab_name": "Laboratorio Clínico del Centro", "notes": "", "requested": labs[k], "ordered_at": when.Format(time.RFC3339)}
			if s.must("POST", "/patients/"+p.ID+"/lab/orders", in, nil) {
				orders++
			}
		}
	}
	entries := 0
	for i, n := range []string{"Rebeca Salinas", "Tomás Villarreal", "Irene Domínguez"} {
		if s.must("POST", "/waitlist", map[string]any{
			"name": n, "phone": fmt.Sprintf("%s9%02d", phonePrefix, i+1), "email": "", "professional_id": s.pro, "service_id": nil, "days": []int{1, 2, 3, 4, 5},
			"from_time": "09:00", "to_time": "13:00", "notes": "Prefiere por la mañana.", "consent": true,
		}, nil) {
			entries++
		}
	}
	blocks := 0
	for _, b := range []struct {
		off, span int
		from, to  string
		reason    string
	}{{4, 0, "14:00", "15:30", "Dato de ejemplo: junta de equipo"}, {16, 1, "09:00", "18:00", "Dato de ejemplo: capacitación"}} {
		from := s.now.AddDate(0, 0, b.off)
		for from.Weekday() == time.Saturday || from.Weekday() == time.Sunday {
			from = from.AddDate(0, 0, 1)
		}
		if s.must("POST", "/agenda/blocks", map[string]any{
			"professional_id": s.pro, "date_from": from.Format("2006-01-02"), "date_to": from.AddDate(0, 0, b.span).Format("2006-01-02"), "startHour": b.from, "endHour": b.to, "reason": b.reason,
		}, nil) {
			blocks++
		}
	}
	fmt.Printf("  medicación crónica: %d, estudios de laboratorio: %d, lista de espera: %d, bloqueos de agenda: %d\n", meds, orders, entries, blocks)
}

// ---------------------------------------------------------------------------------------------------------------
// tidy: move everything back in time

func (s *seeder) tidy(ctx context.Context, patients []*patient, visits []*visit) {
	for _, p := range patients {
		created := p.firstSeen
		if created.IsZero() {
			created = s.now.AddDate(0, 0, -s.rnd.Intn(s.days))
		}
		s.exec(`UPDATE patients SET created_at = $2, updated_at = $2, privacy_notice_at = $2 WHERE id = $1`, p.ID, s.at(created.AddDate(0, 0, -s.rnd.Intn(3)), 9*60+s.rnd.Intn(480)))
	}
	for _, v := range visits {
		start := s.at(v.Day, v.Start)
		booked := start.AddDate(0, 0, -(1 + s.rnd.Intn(9)))
		if booked.After(s.now) {
			booked = s.now.Add(-time.Hour)
		}
		s.exec(`UPDATE appointments SET created_at = $2 WHERE id = $1`, v.ID, booked)
		switch v.Status {
		case "completed":
			arrived := start.Add(-time.Duration(s.rnd.Intn(10)) * time.Minute)
			begun := start.Add(time.Duration(s.rnd.Intn(8)) * time.Minute)
			done := s.at(v.Day, v.End)
			s.exec(`UPDATE appointments SET arrived_at = $2, started_at = $3, finished_at = $4, updated_at = $4 WHERE id = $1`, v.ID, arrived, begun, done)
			if v.Encounter != "" {
				s.exec(`UPDATE encounters SET occurred_at = $2, created_at = $2 WHERE id = $1`, v.Encounter, done)
				s.exec(`UPDATE patients SET last_encounter_at = greatest(coalesce(last_encounter_at, $2), $2) WHERE id = $1`, v.P.ID, done)
			}
			if v.Rx != "" {
				s.exec(`UPDATE prescriptions SET issued_at = $2, created_at = $2, valid_until = ($2::timestamptz + interval '30 days')::date WHERE id = $1`, v.Rx, done)
			}
			s.exec(`UPDATE encounter_charges SET created_at = $2, updated_at = $2, sent_at = $2 WHERE appointment_id = $1`, v.ID, done)
		case "no_show", "cancelled":
			s.exec(`UPDATE appointments SET updated_at = $2 WHERE id = $1`, v.ID, start.Add(-2*time.Hour))
		}
	}
	s.exec(`UPDATE chronic_medications SET created_at = started_on::timestamptz, updated_at = started_on::timestamptz WHERE clinic_id = $1 AND patient_id IN (SELECT id FROM patients WHERE clinic_id = $1 AND phone LIKE '`+phonePrefix+`%') AND started_on IS NOT NULL`, s.c.ClinicID)
	s.exec(`UPDATE waitlist_entries SET created_at = $2, updated_at = $2 WHERE clinic_id = $1 AND phone LIKE '%`+phonePrefix+`%'`, s.c.ClinicID, s.now.AddDate(0, 0, -9))
	// the register already knows these visits were paid for
	s.exec(`UPDATE encounter_charges ec SET status = 'charged', charged_at = s.created_at, sale_id = s.id
	        FROM sales s WHERE s.appointment_id = ec.appointment_id AND ec.status IN ('draft', 'sent') AND ec.clinic_id = $1`, s.c.ClinicID)
	_ = ctx
}

// ---------------------------------------------------------------------------------------------------------------

func (s *seeder) undo(ctx context.Context) {
	clinic := s.c.ClinicID
	del := func(label, sql string, args ...any) {
		tag, err := s.pool.Exec(ctx, sql, args...)
		if err != nil {
			log.Fatalf("%s: %v", label, err)
		}
		fmt.Printf("  %-22s %d\n", label, tag.RowsAffected())
	}
	mock := `(SELECT id FROM patients WHERE clinic_id = $1 AND phone LIKE '` + phonePrefix + `%')`
	del("pre-cuentas", `DELETE FROM encounter_charges WHERE clinic_id = $1 AND patient_id IN `+mock, clinic)
	del("pagos de ventas", `DELETE FROM sale_payments WHERE sale_id IN (SELECT id FROM sales WHERE clinic_id = $1 AND patient_id IN `+mock+`)`, clinic)
	del("ventas", `DELETE FROM sales WHERE clinic_id = $1 AND patient_id IN `+mock, clinic)
	del("cortes de caja", `DELETE FROM cash_sessions WHERE clinic_id = $1 AND note = $2`, clinic, cashNote)
	del("lista de espera", `DELETE FROM waitlist_entries WHERE clinic_id = $1 AND phone LIKE '%`+phonePrefix+`%'`, clinic)
	del("bloqueos de agenda", `DELETE FROM time_blocks WHERE clinic_id = $1 AND reason LIKE 'Dato de ejemplo%'`, clinic)
	del("citas", `DELETE FROM appointments WHERE clinic_id = $1 AND (patient_id IN `+mock+`)`, clinic)
	del("pacientes", `DELETE FROM patients WHERE clinic_id = $1 AND phone LIKE '`+phonePrefix+`%'`, clinic)
	del("conceptos del catálogo", `DELETE FROM catalog_items WHERE clinic_id = $1 AND sku LIKE '`+skuPrefix+`%'`, clinic)
	fmt.Println("Listo: los datos de ejemplo se quitaron.")
}
