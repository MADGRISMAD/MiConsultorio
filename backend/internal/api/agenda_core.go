package api

import "context"

// Codes returned by slotConflict.
const (
	SlotFree    = ""
	SlotTaken   = "SLOT_TAKEN"   // the professional (or the room) already has an appointment then
	SlotBlocked = "SLOT_BLOCKED" // a time block (vacation, emergency, maintenance) covers it
)

// slotConflict says whether a professional can take an appointment on date between start and end
// ("HH:MM"). professionalID may be nil (no professional assigned): then only whole-clinic blocks
// apply. room, when not empty, must also be free. excludeID is the appointment being moved.
// Cancelled and no-show appointments never conflict. q is the pool or a transaction.
func (s *Server) slotConflict(ctx context.Context, q queryRower, clinicID string, professionalID *string, room, date, start, end, excludeID string) (string, error) {
	var blocked bool
	if err := q.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM time_blocks b
			WHERE b.clinic_id = $1 AND $2::date BETWEEN b.date_from AND b.date_to
			  AND (b.professional_id IS NULL OR b.professional_id = $3::uuid)
			  AND (b.start_hour IS NULL OR (b.start_hour < $5::time AND b.end_hour > $4::time)))`,
		clinicID, date, professionalID, start, end).Scan(&blocked); err != nil {
		return "", err
	}
	if blocked {
		return SlotBlocked, nil
	}
	var taken bool
	if err := q.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM appointments a
			WHERE a.clinic_id = $1 AND a.date = $2::date AND a.status NOT IN ('cancelled', 'no_show')
			  AND a.start_hour < $5::time AND a.end_hour > $4::time
			  AND ($6 = '' OR a.id <> NULLIF($6, '')::uuid)
			  AND ((a.professional_id IS NOT DISTINCT FROM $3::uuid AND $3::uuid IS NOT NULL) OR ($7 <> '' AND a.room = $7)))`,
		clinicID, date, professionalID, start, end, excludeID, room).Scan(&taken); err != nil {
		return "", err
	}
	if taken {
		return SlotTaken, nil
	}
	return SlotFree, nil
}
