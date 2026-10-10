package api

import (
	"context"
	"log"
	"time"
)

// StartBackground launches the workers that run for the life of the process.
func (s *Server) StartBackground(ctx context.Context) {
	go every(ctx, "reminders", time.Minute, s.reminderPass, nil)
	go every(ctx, "waitlist", time.Minute, s.waitlistPass, waitlistKick)
	go every(ctx, "birthdays", 30*time.Minute, s.birthdayPass, nil)
	go every(ctx, "recalls", 30*time.Minute, s.recallPass, nil)
	go every(ctx, "surveys", 15*time.Minute, s.surveyPass, nil)
	go every(ctx, "patient merge", 6*time.Hour, s.mergePass, nil)
	go every(ctx, "notifications", time.Hour, s.notificationPass, nil)
}

// every runs pass now and then each interval until ctx ends. A panic in one pass is logged and never ends the loop.
// A signal on wake (optional) runs it earlier, after a short pause so a burst of changes settles into one pass.
func every(ctx context.Context, name string, interval time.Duration, pass func(context.Context), wake <-chan struct{}) {
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("%s: panic: %v", name, r)
				}
			}()
			pass(ctx)
		}()
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-wake:
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}
}
