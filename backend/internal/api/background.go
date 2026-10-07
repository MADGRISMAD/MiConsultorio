package api

import "context"

// StartBackground launches the workers that run for the life of the process.
func (s *Server) StartBackground(ctx context.Context) {
	go s.runReminders(ctx)
}
