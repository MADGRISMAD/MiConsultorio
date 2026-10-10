package api

import (
	"context"
	"net/http"
	"strings"
)

// One open appointment per giro: a patient who already has an upcoming (scheduled or confirmed) appointment with someone
// of a giro cannot get another with the same giro, but can with a different one (a general physician today and the
// physiotherapist tomorrow). A professional's giro is their areas, or the clinic's main giro when they work in all.
// Appointments without a professional have no giro and are not counted.

const areaOfProfessional = `coalesce(nullif(u.areas, '{}'), ARRAY[c.kind])`

// openInSameArea returns an error when the patient (by id, or by e-mail or phone for someone not yet registered) already has an
// upcoming appointment in a giro of the professional. excludeID is the appointment being moved.
func (s *Server) openInSameArea(ctx context.Context, q queryRower, clinicID, patientID string, email, phone string, proID *string, excludeID string) error {
	if proID == nil || *proID == "" || (patientID == "" && email == "" && phone == "") {
		return nil
	}
	var date, start, who string
	var found bool
	loc := clinicLocation(ctx, q, clinicID)
	err := q.QueryRow(ctx, `
		SELECT true, to_char(ap.date, 'YYYY-MM-DD'), to_char(ap.start_hour, 'HH24:MI'), u.name
		FROM appointments ap
		JOIN users u ON u.id = ap.professional_id
		JOIN clinics c ON c.id = ap.clinic_id
		WHERE ap.clinic_id = $1 AND ap.status IN ('scheduled', 'confirmed')
		  AND (ap.date + ap.start_hour) > (now() AT TIME ZONE $2)
		  AND ($3 = '' OR ap.id::text <> $3)
		  AND (ap.patient_id = nullif($4, '')::uuid OR ($5 <> '' AND lower(ap.email) = $5) OR ($6 <> '' AND ap.phone = $6))
		  AND `+areaOfProfessional+` && (SELECT coalesce(nullif(u2.areas, '{}'), ARRAY[c2.kind]) FROM users u2 JOIN clinics c2 ON c2.id = u2.clinic_id WHERE u2.id = $7::uuid)
		ORDER BY ap.date, ap.start_hour LIMIT 1`,
		clinicID, loc.String(), excludeID, patientID, strings.ToLower(strings.TrimSpace(email)), strings.TrimSpace(phone), *proID).Scan(&found, &date, &start, &who)
	if err != nil || !found {
		return nil // no match (or no way to tell): never block on a failed lookup
	}
	e := fail(http.StatusConflict, "Ya hay una cita pendiente en este giro: "+date+" a las "+start+" con "+who+". Para agendar otra con la misma especialidad, primero reprograma o cancela esa. En otras especialidades sí se puede.")
	e.Code = "ALREADY_BOOKED_AREA"
	return e
}
