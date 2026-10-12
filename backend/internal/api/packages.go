package api

import (
	"context"
	"errors"
	"math"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Paquetes de sesiones. A package is a catalog service with package_sessions set: it is sold at the POS like any
// other service and, sold to a patient, leaves a balance of sessions (patient_packages) that appointments draw down.

// packageAreas are the giros where prepaying a series of sessions is how people are treated: therapy, diet follow-up,
// rehabilitation, chiropractic care and skin treatments. The rest charge per visit or by treatment plan.
var packageAreas = []string{"PSYCHOLOGY", "NUTRITION", "PHYSIOTHERAPY", "CHIROPRACTIC", "DERMATOLOGY"}

func anyPackageArea(kinds []string) bool {
	for _, k := range kinds {
		if slices.Contains(packageAreas, k) {
			return true
		}
	}
	return false
}

type packageOut struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	PriceCents  int     `json:"price_cents"`
	Sessions    int     `json:"sessions"`
	ServiceID   *string `json:"service_id"`
	ServiceName string  `json:"service_name"`
	ValidDays   *int    `json:"valid_days"`
	Public      bool    `json:"public"`
	Active      bool    `json:"active"`
}

type packageIn struct {
	Name       string  `json:"name"`
	PriceCents int     `json:"price_cents"`
	Sessions   int     `json:"sessions"`
	ServiceID  *string `json:"service_id"`
	ValidDays  *int    `json:"valid_days"`
	Public     bool    `json:"public"`
	Active     *bool   `json:"active"`
}

func (in *packageIn) validate() string {
	in.Name = strings.TrimSpace(in.Name)
	switch {
	case in.Name == "" || utf8.RuneCountInString(in.Name) > 120:
		return "Escribe el nombre del paquete (máximo 120 caracteres)."
	case in.Sessions < 2 || in.Sessions > 100:
		return "Un paquete tiene entre 2 y 100 sesiones."
	case in.PriceCents <= 0 || !okCents(in.PriceCents):
		return "El precio del paquete no es válido."
	case in.ValidDays != nil && (*in.ValidDays < 1 || *in.ValidDays > 1825):
		return "La vigencia debe estar entre 1 y 1825 días."
	case in.ServiceID != nil && !validUUID(*in.ServiceID):
		return "El servicio no es válido."
	}
	return ""
}

// packageGate lets a clinic with a giro that sells packages use them, and nobody else.
func (s *Server) packageGate(ctx context.Context, clinicID string) error {
	kinds, err := s.clinicKindsFor(ctx, clinicID)
	if err != nil {
		return err
	}
	if !anyPackageArea(kinds) {
		return fail(http.StatusConflict, "Los paquetes de sesiones son para psicología, nutrición, fisioterapia, quiropráctica y dermatología.")
	}
	return nil
}

const packageSelect = `
	SELECT c.id::text, c.name, c.price_cents, c.package_sessions, c.package_service_id::text, coalesce(sv.name, ''), c.package_valid_days, c.public, c.active
	FROM catalog_items c LEFT JOIN catalog_items sv ON sv.id = c.package_service_id
	WHERE c.clinic_id = $1 AND c.package_sessions IS NOT NULL`

func scanPackage(row interface{ Scan(...any) error }) (packageOut, error) {
	var x packageOut
	err := row.Scan(&x.ID, &x.Name, &x.PriceCents, &x.Sessions, &x.ServiceID, &x.ServiceName, &x.ValidDays, &x.Public, &x.Active)
	return x, err
}

// listPackages is GET /packages.
func (s *Server) listPackages(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(), packageSelect+` ORDER BY c.active DESC, c.name`, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	out := []packageOut{}
	for rows.Next() {
		x, err := scanPackage(rows)
		if err != nil {
			serverError(w, r, err)
			return
		}
		out = append(out, x)
	}
	kinds, _ := s.clinicKindsFor(r.Context(), p.ClinicID)
	writeJSON(w, http.StatusOK, map[string]any{"packages": out, "enabled": anyPackageArea(kinds)})
}

// checkPackageService makes sure the service a package is for is a plain service of the clinic.
func checkPackageService(ctx context.Context, q queryRower, clinicID string, id *string) error {
	if id == nil || *id == "" {
		return nil
	}
	var ok bool
	if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM catalog_items WHERE id = $1 AND clinic_id = $2 AND kind = 'service' AND package_sessions IS NULL AND active)`, *id, clinicID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return fail(http.StatusBadRequest, "El servicio del paquete no existe en tu catálogo.")
	}
	return nil
}

// savePackage creates (id == "") or updates a package. Administrators and the clinic's specialists manage them.
func (s *Server) savePackage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id != "" && !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paquete no encontrado.")
		return
	}
	var in packageIn
	if !decode(w, r, &in) {
		return
	}
	if msg := in.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	p := principalFrom(r.Context())
	if err := s.packageGate(r.Context(), p.ClinicID); err != nil {
		writeFailure(w, r, err)
		return
	}
	if in.ServiceID != nil && *in.ServiceID == "" {
		in.ServiceID = nil
	}
	if err := checkPackageService(r.Context(), s.db, p.ClinicID, in.ServiceID); err != nil {
		writeFailure(w, r, err)
		return
	}
	active := in.Active == nil || *in.Active
	var out packageOut
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		if id == "" {
			if err := tx.QueryRow(r.Context(), `
				INSERT INTO catalog_items (clinic_id, kind, name, category, price_cents, tax_rate, active, public, package_sessions, package_service_id, package_valid_days)
				VALUES ($1, 'service', $2, 'Paquetes', $3, 0, $4, $5, $6, $7::uuid, $8) RETURNING id::text`,
				p.ClinicID, in.Name, in.PriceCents, active, in.Public, in.Sessions, in.ServiceID, in.ValidDays).Scan(&id); err != nil {
				return err
			}
		} else {
			tag, err := tx.Exec(r.Context(), `
				UPDATE catalog_items SET name = $3, price_cents = $4, active = $5, public = $6, package_sessions = $7, package_service_id = $8::uuid,
				       package_valid_days = $9, updated_at = now()
				WHERE clinic_id = $1 AND id = $2 AND package_sessions IS NOT NULL`,
				p.ClinicID, id, in.Name, in.PriceCents, active, in.Public, in.Sessions, in.ServiceID, in.ValidDays)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return pgx.ErrNoRows
			}
		}
		audit(r.Context(), tx, p.ClinicID, p, "package_saved", "Guardó el paquete «"+in.Name+"»", map[string]any{"package": id, "sessions": in.Sessions})
		var err error
		out, err = scanPackage(tx.QueryRow(r.Context(), packageSelect+` AND c.id = $2`, p.ClinicID, id))
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Paquete no encontrado.")
		return
	}
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "Ya hay un artículo con ese nombre.")
			return
		}
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"package": out})
}

// sellPackage leaves the balance of a package sold in a sale. A package needs the patient it is for.
func sellPackage(ctx context.Context, tx pgx.Tx, clinicID, saleID, patientID, itemID string, qty float64) error {
	var name string
	var sessions int
	var service *string
	var days *int
	err := tx.QueryRow(ctx, `SELECT name, package_sessions, package_service_id::text, package_valid_days FROM catalog_items WHERE clinic_id = $1 AND id = $2 AND package_sessions IS NOT NULL`,
		clinicID, itemID).Scan(&name, &sessions, &service, &days)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // not a package
	}
	if err != nil {
		return err
	}
	if patientID == "" {
		return fail(http.StatusBadRequest, "Elige al paciente para venderle el paquete «"+name+"».")
	}
	n := int(math.Round(qty))
	if n < 1 || n > 20 {
		return fail(http.StatusBadRequest, "Vende de 1 a 20 paquetes por venta.")
	}
	for i := 0; i < n; i++ {
		if _, err := tx.Exec(ctx, `
			INSERT INTO patient_packages (clinic_id, patient_id, catalog_item_id, name, service_id, sessions_total, expires_on, sale_id)
			VALUES ($1, $2, $3, $4, $5::uuid, $6, CASE WHEN $7::int IS NULL THEN NULL ELSE current_date + $7::int END, $8)`,
			clinicID, patientID, itemID, name, service, sessions, days, saleID); err != nil {
			return err
		}
	}
	return nil
}

type patientPackage struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	ServiceID   *string `json:"service_id"`
	ServiceName string  `json:"service_name"`
	Total       int     `json:"sessions_total"`
	Used        int     `json:"sessions_used"`
	ExpiresOn   *string `json:"expires_on"`
	Expired     bool    `json:"expired"`
	Status      string  `json:"status"`
	SoldAt      string  `json:"sold_at"`
}

// listPatientPackages is GET /patients/{id}/packages.
func (s *Server) listPatientPackages(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(), `
		SELECT pk.id::text, pk.name, pk.service_id::text, coalesce(sv.name, ''), pk.sessions_total, pk.sessions_used,
		       to_char(pk.expires_on, 'YYYY-MM-DD'), coalesce(pk.expires_on < current_date, false), pk.status, to_char(pk.created_at, 'YYYY-MM-DD')
		FROM patient_packages pk LEFT JOIN catalog_items sv ON sv.id = pk.service_id
		WHERE pk.clinic_id = $1 AND pk.patient_id = $2 ORDER BY pk.created_at DESC LIMIT 50`, p.ClinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	out := []patientPackage{}
	for rows.Next() {
		var x patientPackage
		if err := rows.Scan(&x.ID, &x.Name, &x.ServiceID, &x.ServiceName, &x.Total, &x.Used, &x.ExpiresOn, &x.Expired, &x.Status, &x.SoldAt); err != nil {
			serverError(w, r, err)
			return
		}
		out = append(out, x)
	}
	writeJSON(w, http.StatusOK, map[string]any{"packages": out})
}

// redeemPackage is POST /appointments/{id}/package: the visit is paid with a session of the patient's package.
func (s *Server) redeemPackage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var in struct {
		PackageID string `json:"package_id"`
	}
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Cita no encontrada.")
		return
	}
	if !decode(w, r, &in) {
		return
	}
	if !validUUID(in.PackageID) {
		writeError(w, http.StatusBadRequest, "El paquete no es válido.")
		return
	}
	p := principalFrom(r.Context())
	var left int
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		ap, err := lockAppointmentForPackage(r.Context(), tx, p, id)
		if err != nil {
			return err
		}
		if ap.packageID != "" {
			return fail(http.StatusConflict, "Esta cita ya se paga con un paquete.")
		}
		if ap.patientID == "" {
			return fail(http.StatusConflict, "Registra al paciente de la cita para usar su paquete.")
		}
		if ap.saleID != "" {
			return fail(http.StatusConflict, "Esta cita ya tiene una venta.")
		}
		var total, used int
		var service *string
		var expires *string
		var pstatus, pname string
		err = tx.QueryRow(r.Context(), `
			SELECT sessions_total, sessions_used, service_id::text, to_char(expires_on, 'YYYY-MM-DD'), status, name
			FROM patient_packages WHERE clinic_id = $1 AND id = $2 AND patient_id = $3 FOR UPDATE`, p.ClinicID, in.PackageID, ap.patientID).Scan(&total, &used, &service, &expires, &pstatus, &pname)
		if errors.Is(err, pgx.ErrNoRows) {
			return fail(http.StatusNotFound, "Ese paquete no es de este paciente.")
		}
		if err != nil {
			return err
		}
		switch {
		case pstatus != "active":
			return fail(http.StatusConflict, "Ese paquete está cancelado.")
		case used >= total:
			return fail(http.StatusConflict, "Ese paquete ya no tiene sesiones.")
		case expires != nil && *expires < ap.date:
			return fail(http.StatusConflict, "Ese paquete vence el "+*expires+", antes de la fecha de la cita.")
		case service != nil && (ap.serviceID == "" || ap.serviceID != *service):
			return fail(http.StatusConflict, "Ese paquete es para otro servicio.")
		}
		if _, err := tx.Exec(r.Context(), `UPDATE patient_packages SET sessions_used = sessions_used + 1 WHERE id = $1`, in.PackageID); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `UPDATE appointments SET package_id = $3, updated_at = now() WHERE clinic_id = $1 AND id = $2`, p.ClinicID, id, in.PackageID); err != nil {
			return err
		}
		left = total - used - 1
		audit(r.Context(), tx, p.ClinicID, p, "package_redeemed", "Usó una sesión del paquete «"+pname+"» en una cita", map[string]any{"appointment": id, "package": in.PackageID, "left": left})
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Cita no encontrada.")
		return
	}
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions_left": left})
}

// unredeemPackage is DELETE /appointments/{id}/package: gives the session back.
func (s *Server) unredeemPackage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Cita no encontrada.")
		return
	}
	p := principalFrom(r.Context())
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		ap, err := lockAppointmentForPackage(r.Context(), tx, p, id)
		if err != nil {
			return err
		}
		if ap.packageID == "" {
			return nil
		}
		return releasePackage(r.Context(), tx, p.ClinicID, id)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Cita no encontrada.")
		return
	}
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type pkgAppt struct{ patientID, serviceID, saleID, packageID, date string }

// lockAppointmentForPackage locks the appointment and checks the caller may handle it (the agenda's staff, or the
// assigned specialist) and that it is still open.
func lockAppointmentForPackage(ctx context.Context, tx pgx.Tx, p *Principal, id string) (pkgAppt, error) {
	var a pkgAppt
	var prof, status string
	err := tx.QueryRow(ctx, `
		SELECT coalesce(patient_id::text, ''), coalesce(service_id::text, ''), coalesce(sale_id::text, ''), coalesce(package_id::text, ''),
		       to_char(date, 'YYYY-MM-DD'), coalesce(professional_id::text, ''), status
		FROM appointments WHERE clinic_id = $1 AND id = $2 FOR UPDATE`, p.ClinicID, id).Scan(&a.patientID, &a.serviceID, &a.saleID, &a.packageID, &a.date, &prof, &status)
	if err != nil {
		return a, err
	}
	if !hasPermission(p.Permissions, PermAdminAppointments) && !(p.Role == RoleDoctor && prof == p.UserID) {
		return a, fail(http.StatusForbidden, "No tienes permiso para cambiar esta cita.")
	}
	if status == "cancelled" || status == "no_show" || status == "pending_payment" {
		return a, fail(http.StatusConflict, "Esta cita ya no está activa.")
	}
	return a, nil
}

// releasePackage gives back the session an appointment had taken from a package (a cancellation, or the staff changing their mind).
func releasePackage(ctx context.Context, q execer, clinicID, apptID string) error {
	if _, err := q.Exec(ctx, `
		UPDATE patient_packages SET sessions_used = greatest(sessions_used - 1, 0)
		WHERE clinic_id = $1 AND id = (SELECT package_id FROM appointments WHERE clinic_id = $1 AND id = $2)`, clinicID, apptID); err != nil {
		return err
	}
	_, err := q.Exec(ctx, `UPDATE appointments SET package_id = NULL WHERE clinic_id = $1 AND id = $2`, clinicID, apptID)
	return err
}
