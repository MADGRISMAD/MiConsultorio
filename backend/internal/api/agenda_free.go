package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

// freeSlots is GET /agenda/free-slots?professional=&date=: the free start times of a professional on a day, for the
// team to pick an exact time (a follow-up chosen from the calendar). It uses the same opening hours, breaks, blocks and
// bookings as the public page, but without the lead time or the "bookable online" filter.
func (s *Server) freeSlots(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	ctx := r.Context()
	proID, date := r.URL.Query().Get("professional"), r.URL.Query().Get("date")
	if proID == "" {
		proID = p.UserID
	}
	if !validUUID(proID) {
		writeError(w, http.StatusBadRequest, "Elige un profesional.")
		return
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		writeError(w, http.StatusBadRequest, "La fecha no es válida.")
		return
	}
	cl, err := loadClinic(ctx, s.db, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	var slotMin int
	_ = s.db.QueryRow(ctx, `SELECT slot_minutes FROM agenda_settings WHERE clinic_id = $1`, p.ClinicID).Scan(&slotMin)
	if slotMin <= 0 {
		slotMin = cl.Settings.AppointmentMinutes
	}
	c := &bookingClinic{ID: p.ClinicID, Name: cl.Name, SlotMinutes: slotMin, LeadHours: 0, HorizonDays: 730, Loc: clinicLocation(ctx, s.db, p.ClinicID), Hours: cl.Settings}
	pro := bookable{ID: proID}
	var raw []byte
	var own int
	err = s.db.QueryRow(ctx, `
		SELECT coalesce(ps.slot_minutes, 0), coalesce(ps.hours, '{}'::jsonb) FROM users u LEFT JOIN professional_settings ps ON ps.user_id = u.id
		WHERE u.id = $1 AND u.clinic_id = $2 AND NOT u.disabled AND u.role IN ('admin', 'doctor')`, proID, p.ClinicID).Scan(&own, &raw)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "Profesional no encontrado.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	pro.Slot = own
	_ = json.Unmarshal(raw, &pro.Hours)
	slots := []map[string]string{}
	for _, st := range c.candidateSlots(pro, date, time.Now(), 0) {
		end := pro.end(c, st, 0)
		code, err := s.slotConflict(ctx, s.db, p.ClinicID, &proID, "", date, st, end, "")
		if err != nil {
			serverError(w, r, err)
			return
		}
		if code == SlotFree {
			slots = append(slots, map[string]string{"start": st, "end": end})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"date": date, "slots": slots})
}

// pendingCheck is GET /agenda/pending-check?patient=&professional=&exclude=&email=&phone=: says before saving whether the
// patient already has an open appointment in the giro of that professional (the same rule the save enforces).
func (s *Server) pendingCheck(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	q := r.URL.Query()
	pro := q.Get("professional")
	patient, exclude := q.Get("patient"), q.Get("exclude")
	if (pro != "" && !validUUID(pro)) || (patient != "" && !validUUID(patient)) || (exclude != "" && !validUUID(exclude)) {
		writeError(w, http.StatusBadRequest, "Los datos no son válidos.")
		return
	}
	var proPtr *string
	if pro != "" {
		proPtr = &pro
	}
	writeJSON(w, http.StatusOK, map[string]any{"pending": s.findOpenInSameArea(r.Context(), s.db, p.ClinicID, patient, q.Get("email"), q.Get("phone"), proPtr, exclude)})
}
