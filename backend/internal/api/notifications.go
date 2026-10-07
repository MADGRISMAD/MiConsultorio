package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/madgrismad/miconsultorio/backend/internal/config"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

const (
	// Notifications older than this are neither listed nor counted.
	ntfVisibleDays = 60
	// ... and are deleted by the daily job after this many days.
	ntfKeepDays = 180
	// Lots expiring within this many days are reported by the stock job.
	ntfExpiryDays = 30
)

// ntfDB is what a notification needs from the pool or a transaction.
type ntfDB interface {
	execer
	queryRower
}

// ntfNotice is one in-app notice. UserID aims it at one person; otherwise everyone whose role holds
// Perm sees it (an empty Perm means the whole team). Dedupe makes the insert idempotent per clinic.
type ntfNotice struct {
	UserID, Perm, Kind, Title, Body, Link, Dedupe string
}

// notify records an in-app notification. It never fails the caller: the insert cannot violate anything
// but the dedupe key, which it skips, so it is safe to call inside another transaction.
func (s *Server) notify(ctx context.Context, q execer, clinicID string, n ntfNotice) {
	var user, dedupe any
	if n.UserID != "" {
		user = n.UserID
	}
	if n.Dedupe != "" {
		dedupe = n.Dedupe
	}
	if _, err := q.Exec(ctx, `
		INSERT INTO notifications (clinic_id, user_id, perm, kind, title, body, link, dedupe_key)
		VALUES ($1, $2::uuid, $3, $4, $5, $6, $7, $8) ON CONFLICT DO NOTHING`,
		clinicID, user, n.Perm, n.Kind, truncate(n.Title, 160), truncate(n.Body, 500), n.Link, dedupe); err != nil {
		log.Printf("notify: %v", err)
	}
}

// ntfAppointment notifies the people who manage the agenda about something that happened to an
// appointment and, when its professional cannot manage the agenda (a doctor), the professional too.
func (s *Server) ntfAppointment(ctx context.Context, q ntfDB, clinicID, professionalID, kind, title, body, link string) {
	s.notify(ctx, q, clinicID, ntfNotice{Perm: PermAdminAppointments, Kind: kind, Title: title, Body: body, Link: link})
	if professionalID == "" {
		return
	}
	var role string
	if q.QueryRow(ctx, `SELECT role FROM users WHERE clinic_id = $1 AND id = $2 AND NOT disabled`, clinicID, professionalID).Scan(&role) != nil {
		return
	}
	if !hasPermission(permissionsFor(role), PermAdminAppointments) {
		s.notify(ctx, q, clinicID, ntfNotice{UserID: professionalID, Kind: kind, Title: title, Body: body, Link: link})
	}
}

// ntfAppointmentByID is ntfAppointment for an appointment already stored: it describes it from its row.
func (s *Server) ntfAppointmentByID(ctx context.Context, q ntfDB, clinicID, appointmentID, kind, title, link string) {
	var names, last, prof string
	var date, start string
	if q.QueryRow(ctx, `
		SELECT names, last_names, coalesce(professional_id::text, ''), to_char(date, 'YYYY-MM-DD'), to_char(start_hour, 'HH24:MI')
		FROM appointments WHERE clinic_id = $1 AND id = $2`, clinicID, appointmentID).Scan(&names, &last, &prof, &date, &start) != nil {
		return
	}
	s.ntfAppointment(ctx, q, clinicID, prof, kind, title, names+" "+last+" · "+date+" "+start, link)
}

// ---------------------------------------------------------------------------
// Routes
// ---------------------------------------------------------------------------

// mountNotificationsInbox: the bell of every signed-in team member.
func (s *Server) mountNotificationsInbox(r chi.Router) {
	r.Get("/notifications", s.listNotifications)
	r.Get("/notifications/count", s.countNotifications)
	r.Post("/notifications/read-all", s.readAllNotifications)
	r.Post("/notifications/{id}/read", s.readNotification)
}

type ntfOut struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Link      string    `json:"link"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

// ntfVisible is the WHERE clause of what person $2 (with the permissions $3) may see in clinic $1.
const ntfVisible = `n.clinic_id = $1 AND n.created_at > now() - interval '` + "60" + ` days'
	AND (n.user_id = $2::uuid OR (n.user_id IS NULL AND (n.perm = '' OR n.perm = ANY($3::text[]))))`

const ntfIsRead = `(CASE WHEN n.user_id IS NOT NULL THEN n.read_at IS NOT NULL
	ELSE EXISTS (SELECT 1 FROM notification_reads x WHERE x.notification_id = n.id AND x.user_id = $2::uuid) END)`

func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	limit := 30
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v >= 1 && v <= 200 {
		limit = v
	}
	onlyUnread := r.URL.Query().Get("unread") == "1"
	rows, err := s.db.Query(r.Context(), `
		SELECT n.id::text, n.kind, n.title, n.body, n.link, `+ntfIsRead+` AS is_read, n.created_at
		FROM notifications n WHERE `+ntfVisible+` AND (NOT $4 OR NOT `+ntfIsRead+`)
		ORDER BY n.created_at DESC, n.id LIMIT $5`, p.ClinicID, p.UserID, p.Permissions, onlyUnread, limit)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	out := []ntfOut{}
	for rows.Next() {
		var n ntfOut
		if err := rows.Scan(&n.ID, &n.Kind, &n.Title, &n.Body, &n.Link, &n.Read, &n.CreatedAt); err != nil {
			serverError(w, r, err)
			return
		}
		out = append(out, n)
	}
	if rows.Err() != nil {
		serverError(w, r, rows.Err())
		return
	}
	unread, err := s.ntfUnread(r.Context(), p)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": out, "unread": unread})
}

func (s *Server) ntfUnread(ctx context.Context, p *Principal) (int, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM notifications n WHERE `+ntfVisible+` AND NOT `+ntfIsRead, p.ClinicID, p.UserID, p.Permissions).Scan(&n)
	return n, err
}

// countNotifications is the light endpoint the bell polls.
func (s *Server) countNotifications(w http.ResponseWriter, r *http.Request) {
	n, err := s.ntfUnread(r.Context(), principalFrom(r.Context()))
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"unread": n})
}

// ntfMarkRead marks as read what the person can see and has not read (one notification or, with an
// empty id, all of them) and returns how many were visible.
func (s *Server) ntfMarkRead(ctx context.Context, p *Principal, id string) (int64, error) {
	var seen int64
	err := inTx(ctx, s.db, func(tx pgx.Tx) error {
		extra := ""
		args := []any{p.ClinicID, p.UserID, p.Permissions}
		if id != "" {
			extra = ` AND n.id = $4::uuid`
			args = append(args, id)
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM notifications n WHERE `+ntfVisible+extra, args...).Scan(&seen); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE notifications n SET read_at = now() WHERE `+ntfVisible+` AND n.user_id IS NOT NULL AND n.read_at IS NULL`+extra, args...); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO notification_reads (notification_id, user_id) SELECT n.id, $2::uuid FROM notifications n
			WHERE `+ntfVisible+` AND n.user_id IS NULL`+extra+` ON CONFLICT DO NOTHING`, args...)
		return err
	})
	return seen, err
}

func (s *Server) readNotification(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Aviso no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	seen, err := s.ntfMarkRead(r.Context(), p, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if seen == 0 {
		writeError(w, http.StatusNotFound, "Aviso no encontrado.")
		return
	}
	n, _ := s.ntfUnread(r.Context(), p)
	writeJSON(w, http.StatusOK, map[string]any{"unread": n})
}

func (s *Server) readAllNotifications(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	if _, err := s.ntfMarkRead(r.Context(), p, ""); err != nil {
		serverError(w, r, err)
		return
	}
	n, _ := s.ntfUnread(r.Context(), p)
	writeJSON(w, http.StatusOK, map[string]any{"unread": n})
}

// ---------------------------------------------------------------------------
// Daily job: stock and expiry alerts
// ---------------------------------------------------------------------------

// ntfStockPass writes, per clinic and per day, one notice summarizing lots about to expire (or expired)
// and products at or below their minimum. The dedupe key makes running it again the same day harmless.
func (s *Server) ntfStockPass(ctx context.Context) (int, error) {
	day := time.Now().Format("2006-01-02")
	rows, err := s.db.Query(ctx, `
		WITH lots AS (
			SELECT l.clinic_id,
			       count(DISTINCT l.item_id) FILTER (WHERE l.expires_on < $1::date) AS expired,
			       count(DISTINCT l.item_id) FILTER (WHERE l.expires_on >= $1::date) AS soon
			FROM stock_lots l JOIN catalog_items c ON c.id = l.item_id
			WHERE c.track_stock AND c.active AND l.qty > 0 AND l.expires_on IS NOT NULL AND l.expires_on <= $1::date + $2::int
			GROUP BY l.clinic_id),
		low AS (
			SELECT clinic_id, count(*) AS n FROM catalog_items WHERE track_stock AND active AND stock <= min_stock GROUP BY clinic_id)
		SELECT coalesce(lots.clinic_id, low.clinic_id), coalesce(lots.expired, 0), coalesce(lots.soon, 0), coalesce(low.n, 0)
		FROM lots FULL JOIN low ON low.clinic_id = lots.clinic_id`, day, ntfExpiryDays)
	if err != nil {
		return 0, err
	}
	type row struct {
		clinic                string
		expired, soon, lowQty int
	}
	var todo []row
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.clinic, &x.expired, &x.soon, &x.lowQty); err != nil {
			rows.Close()
			return 0, err
		}
		todo = append(todo, x)
	}
	rows.Close()
	if rows.Err() != nil {
		return 0, rows.Err()
	}
	n := 0
	for _, x := range todo {
		parts := []string{}
		if x.expired > 0 {
			parts = append(parts, fmt.Sprintf("%d con lotes caducados", x.expired))
		}
		if x.soon > 0 {
			parts = append(parts, fmt.Sprintf("%d con lotes por caducar en %d días", x.soon, ntfExpiryDays))
		}
		if x.lowQty > 0 {
			parts = append(parts, fmt.Sprintf("%d en existencia mínima", x.lowQty))
		}
		if len(parts) == 0 {
			continue
		}
		body := "Productos: "
		for i, p := range parts {
			if i > 0 {
				body += ", "
			}
			body += p
		}
		tag, err := s.db.Exec(ctx, `
			INSERT INTO notifications (clinic_id, perm, kind, title, body, link, dedupe_key)
			VALUES ($1, $2, 'stock_alert', 'Inventario por revisar', $3, '/pos/inventario', $4) ON CONFLICT DO NOTHING`,
			x.clinic, PermPOSManage, body, "stock:"+day)
		if err != nil {
			return n, err
		}
		n += int(tag.RowsAffected())
	}
	return n, nil
}

// ntfCleanup deletes notifications nobody will look at anymore.
func (s *Server) ntfCleanup(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `DELETE FROM notifications WHERE created_at < now() - $1::int * interval '1 day'`, ntfKeepDays)
	return err
}

// runNotificationJobs runs the stock job and the cleanup about once an hour until ctx ends (the dedupe key
// keeps it to one notice per clinic and day).
func (s *Server) runNotificationJobs(ctx context.Context) {
	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	for {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("notifications: panic: %v", r)
				}
			}()
			if _, err := s.ntfStockPass(ctx); err != nil && ctx.Err() == nil {
				log.Printf("notifications: stock: %v", err)
			}
			if err := s.ntfCleanup(ctx); err != nil && ctx.Err() == nil {
				log.Printf("notifications: cleanup: %v", err)
			}
		}()
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// RunNotificationJobsOnce runs the stock job now (used by tests and tooling) and returns how many
// notifications it wrote.
func RunNotificationJobsOnce(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, mailer mail.Sender) (int, error) {
	return newServer(pool, cfg, mailer).ntfStockPass(ctx)
}
