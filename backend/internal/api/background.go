package api

import "context"

// StartBackground launches the workers that run for the life of the process.
func (s *Server) StartBackground(ctx context.Context) {
	go s.runReminders(ctx)
	go s.runWaitlist(ctx)
	go s.runBirthdays(ctx)
	go s.runRecalls(ctx)
	go s.runNotificationJobs(ctx)
}
