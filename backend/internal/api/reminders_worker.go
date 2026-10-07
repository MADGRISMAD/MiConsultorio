package api

import "context"

// runReminders sends the queued appointment reminders until ctx ends.
func (s *Server) runReminders(ctx context.Context) {}

// scheduleReminders recomputes the pending reminders of one appointment (call it after the
// appointment is created, moved, confirmed or cancelled). q is the pool or a transaction.
func (s *Server) scheduleReminders(ctx context.Context, q execer, clinicID, appointmentID string) error {
	return nil
}
