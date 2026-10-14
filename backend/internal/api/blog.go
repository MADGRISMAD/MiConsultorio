package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Blog de Caresia: la IA escribe un artículo cada lunes y jueves (9:00, hora del centro de México) siguiendo el
// banco de temas de blog_topics.go y las instrucciones de blog_prompt.go. Los artículos se guardan en la base
// y se sirven como páginas propias en /blog/{slug}. El equipo de plataforma los revisa, oculta o borra desde
// su panel, y puede pedir uno al momento.

const blogHour = 9 // hora local a partir de la cual se escribe el artículo del día

var blogLoc = func() *time.Location {
	loc, err := time.LoadLocation(defaultTimeZone)
	if err != nil {
		return time.UTC
	}
	return loc
}()

func blogDay(t time.Time) bool { return t.Weekday() == time.Monday || t.Weekday() == time.Thursday }

type blogBlock struct {
	Type  string   `json:"type"`
	Text  string   `json:"text,omitempty"`
	Items []string `json:"items,omitempty"`
}

type blogFAQ struct {
	Q string `json:"q"`
	A string `json:"a"`
}

type blogPost struct {
	ID             string      `json:"id" db:"id"`
	Slug           string      `json:"slug" db:"slug"`
	Title          string      `json:"title" db:"title"`
	Description    string      `json:"description" db:"description"`
	Excerpt        string      `json:"excerpt" db:"excerpt"`
	Keyword        string      `json:"keyword" db:"keyword"`
	Category       string      `json:"category" db:"category"`
	Topic          string      `json:"topic" db:"topic"`
	Body           []blogBlock `json:"body" db:"body"`
	FAQs           []blogFAQ   `json:"faqs" db:"faqs"`
	ReadingMinutes int         `json:"reading_minutes" db:"reading_minutes"`
	Status         string      `json:"status" db:"status"`
	PublishedAt    *time.Time  `json:"published_at" db:"published_at"`
	CreatedAt      time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at" db:"updated_at"`
}

// blogCard is a post without its body, for lists.
type blogCard struct {
	ID             string     `json:"id" db:"id"`
	Slug           string     `json:"slug" db:"slug"`
	Title          string     `json:"title" db:"title"`
	Excerpt        string     `json:"excerpt" db:"excerpt"`
	Category       string     `json:"category" db:"category"`
	ReadingMinutes int        `json:"reading_minutes" db:"reading_minutes"`
	Status         string     `json:"status" db:"status"`
	PublishedAt    *time.Time `json:"published_at" db:"published_at"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
}

const (
	blogCols     = `id, slug, title, description, excerpt, keyword, category, topic, body, faqs, reading_minutes, status, published_at, created_at, updated_at`
	blogCardCols = `id, slug, title, excerpt, category, reading_minutes, status, published_at, created_at`
)

// blogState is what the platform panel shows about the writer: whether it is busy and how the last try ended.
type blogState struct {
	mu      sync.Mutex
	busy    bool
	lastErr string
}

func (b *blogState) start() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.busy {
		return false
	}
	b.busy, b.lastErr = true, ""
	return true
}

func (b *blogState) finish(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.busy = false
	if err != nil {
		b.lastErr = err.Error()
	}
}

func (b *blogState) snapshot() (bool, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.busy, b.lastErr
}

// ---------------------------------------------------------------------------
// Writing a post
// ---------------------------------------------------------------------------

type blogDraft struct {
	Title       string      `json:"title"`
	Slug        string      `json:"slug"`
	Description string      `json:"description"`
	Excerpt     string      `json:"excerpt"`
	Keyword     string      `json:"keyword"`
	Category    string      `json:"category"`
	Body        []blogBlock `json:"body"`
	FAQs        []blogFAQ   `json:"faqs"`
}

// nextBlogTopic is the first topic of the bank that has no post yet; empty when the bank is used up.
func nextBlogTopic(used map[string]bool) blogTopic {
	for _, t := range blogTopics {
		if !used[t.Key] {
			return t
		}
	}
	return blogTopic{}
}

// writeBlogPost asks the AI for a new article and saves it. custom is an optional topic typed by the team.
func (s *Server) writeBlogPost(ctx context.Context, custom string, publish bool) (*blogPost, error) {
	var recent, usedList []string
	rows, err := s.db.Query(ctx, `SELECT title, topic FROM blog_posts ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var title, topic string
		if err := rows.Scan(&title, &topic); err != nil {
			rows.Close()
			return nil, err
		}
		if len(recent) < 80 {
			recent = append(recent, title)
		}
		usedList = append(usedList, topic)
	}
	rows.Close()
	used := map[string]bool{}
	for _, t := range usedList {
		used[t] = true
	}
	topic := blogTopic{}
	if custom == "" {
		topic = nextBlogTopic(used)
	}

	raw, err := s.geminiCall(ctx, geminiOpts{Model: s.cfg.BlogModel, Temperature: 0.7, Timeout: 4 * time.Minute}, blogPrompt(topic, custom, recent), nil, blogSchema)
	if err != nil {
		return nil, err
	}
	var d blogDraft
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("la IA devolvió un artículo ilegible: %w", err)
	}
	words, err := cleanBlogDraft(&d)
	if err != nil {
		return nil, err
	}

	slug, err := s.uniqueBlogSlug(ctx, d.Slug, d.Title)
	if err != nil {
		return nil, err
	}
	status := "draft"
	if publish {
		status = "published"
	}
	body, _ := json.Marshal(d.Body)
	faqs, _ := json.Marshal(d.FAQs)
	minutes := int(math.Ceil(float64(words) / 200))
	rows, err = s.db.Query(ctx, `INSERT INTO blog_posts (slug, title, description, excerpt, keyword, category, topic, body, faqs, reading_minutes, status, published_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, CASE WHEN $11 = 'published' THEN now() END)
		RETURNING `+blogCols,
		slug, d.Title, d.Description, d.Excerpt, d.Keyword, d.Category, topic.Key, body, faqs, max(minutes, 1), status)
	if err != nil {
		return nil, err
	}
	post, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[blogPost])
	if err != nil {
		return nil, err
	}
	return &post, nil
}

// cleanBlogDraft trims and validates what the AI wrote, drops malformed blocks and returns the word count.
func cleanBlogDraft(d *blogDraft) (int, error) {
	d.Title = clip(strings.TrimSpace(d.Title), 110)
	d.Description = clip(strings.TrimSpace(d.Description), 200)
	d.Excerpt = clip(strings.TrimSpace(d.Excerpt), 260)
	d.Keyword = clip(strings.ToLower(strings.TrimSpace(d.Keyword)), 120)
	d.Category = strings.TrimSpace(d.Category)
	if !hasPermission(blogCategories, d.Category) {
		d.Category = "Gestión"
	}
	if d.Excerpt == "" {
		d.Excerpt = d.Description
	}

	words, h2 := 0, 0
	count := func(t string) { words += len(strings.Fields(t)) }
	var body []blogBlock
	for _, b := range d.Body {
		b.Text = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(b.Text), "#"))
		var items []string
		for _, it := range b.Items {
			if it = strings.TrimSpace(it); it != "" {
				items = append(items, it)
				count(it)
			}
		}
		b.Items = items
		switch b.Type {
		case "h2", "h3", "p", "tip":
			if b.Text == "" {
				continue
			}
			b.Items = nil
			count(b.Text)
			if b.Type == "h2" {
				h2++
			}
		case "ul", "ol":
			if len(b.Items) == 0 {
				continue
			}
			b.Text = ""
		default:
			continue
		}
		body = append(body, b)
	}
	d.Body = body

	var faqs []blogFAQ
	for _, f := range d.FAQs {
		f.Q, f.A = strings.TrimSpace(f.Q), strings.TrimSpace(f.A)
		if f.Q != "" && f.A != "" {
			faqs = append(faqs, f)
			count(f.A)
		}
	}
	d.FAQs = faqs
	if faqs == nil {
		d.FAQs = []blogFAQ{}
	}

	if d.Title == "" || h2 < 3 || words < 500 {
		return 0, fmt.Errorf("la IA devolvió un artículo incompleto (%d palabras, %d secciones)", words, h2)
	}
	return words, nil
}

func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:n]))
}

var slugAccents = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n", "à", "a", "è", "e", "ì", "i", "ò", "o", "ù", "u")

// blogSlug turns text into a URL slug: lowercase ASCII words joined by hyphens, at most 80 characters.
func blogSlug(text string) string {
	text = slugAccents.Replace(strings.ToLower(text))
	var b strings.Builder
	dash := false
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 80 {
		slug = slug[:80]
		if i := strings.LastIndexByte(slug, '-'); i > 20 {
			slug = slug[:i]
		}
	}
	return strings.Trim(slug, "-")
}

func (s *Server) uniqueBlogSlug(ctx context.Context, want, title string) (string, error) {
	base := blogSlug(want)
	if base == "" {
		base = blogSlug(title)
	}
	if base == "" {
		base = "articulo"
	}
	for i := 1; i < 100; i++ {
		slug := base
		if i > 1 {
			slug = fmt.Sprintf("%s-%d", base, i)
		}
		var taken bool
		if err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM blog_posts WHERE slug = $1)`, slug).Scan(&taken); err != nil {
			return "", err
		}
		if !taken {
			return slug, nil
		}
	}
	return "", errors.New("no se encontró un slug libre")
}

// ---------------------------------------------------------------------------
// Autopilot: Mondays and Thursdays
// ---------------------------------------------------------------------------

// blogPass writes the article of the day once, on blog days after blogHour. Two servers never both write it, and
// a failed try is retried on the next pass (at most 3 tries a day).
func (s *Server) blogPass(ctx context.Context) {
	if !s.cfg.BlogAutopilot || s.cfg.GeminiAPIKey == "" {
		return
	}
	now := time.Now().In(blogLoc)
	if !blogDay(now) || now.Hour() < blogHour {
		return
	}
	day := now.Format("2006-01-02")
	var attempt int
	err := s.db.QueryRow(ctx, `INSERT INTO blog_runs (day) VALUES ($1)
		ON CONFLICT (day) DO UPDATE SET attempts = blog_runs.attempts + 1, started_at = now()
		WHERE NOT blog_runs.done AND blog_runs.attempts < 3 AND blog_runs.started_at < now() - interval '50 minutes'
		RETURNING attempts`, day).Scan(&attempt)
	if errors.Is(err, pgx.ErrNoRows) {
		return
	}
	if err != nil {
		log.Printf("blog: %v", err)
		return
	}
	if !s.blog.start() {
		// someone asked for one from the panel right now; this run is retried on the next pass
		_, _ = s.db.Exec(ctx, `UPDATE blog_runs SET attempts = attempts - 1, started_at = now() - interval '1 hour' WHERE day = $1`, day)
		return
	}
	post, err := s.writeBlogPost(ctx, "", s.cfg.BlogAutoPublish)
	s.blog.finish(err)
	if err != nil {
		log.Printf("blog: intento %d del %s: %v", attempt, day, err)
		return
	}
	_, _ = s.db.Exec(ctx, `UPDATE blog_runs SET done = true WHERE day = $1`, day)
	log.Printf("blog: nuevo artículo %q (%s)", post.Title, post.Status)
}

// nextBlogRun is when the autopilot writes next, or nil when it is off.
func (s *Server) nextBlogRun(ctx context.Context, now time.Time) *time.Time {
	if !s.cfg.BlogAutopilot || s.cfg.GeminiAPIKey == "" {
		return nil
	}
	now = now.In(blogLoc)
	for i := 0; i < 8; i++ {
		d := now.AddDate(0, 0, i)
		at := time.Date(d.Year(), d.Month(), d.Day(), blogHour, 0, 0, 0, blogLoc)
		if !blogDay(at) {
			continue
		}
		if i == 0 {
			var done bool
			_ = s.db.QueryRow(ctx, `SELECT done FROM blog_runs WHERE day = $1`, at.Format("2006-01-02")).Scan(&done)
			if done {
				continue
			}
		}
		return &at
	}
	return nil
}

// ---------------------------------------------------------------------------
// Public pages
// ---------------------------------------------------------------------------

func (s *Server) mountPublicBlog(r chi.Router) {
	reads := newRateLimiter(600, 10*time.Minute)
	r.Get("/public/blog", func(w http.ResponseWriter, r *http.Request) {
		if !limit(w, reads, "blog|"+clientIP(r)) {
			return
		}
		rows, err := s.db.Query(r.Context(), `SELECT `+blogCardCols+` FROM blog_posts WHERE status = 'published' ORDER BY published_at DESC LIMIT 200`)
		if err != nil {
			serverError(w, r, err)
			return
		}
		posts, err := pgx.CollectRows(rows, pgx.RowToStructByName[blogCard])
		if err != nil {
			serverError(w, r, err)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=300")
		writeJSON(w, http.StatusOK, map[string]any{"posts": posts})
	})
	r.Get("/public/blog/{slug}", func(w http.ResponseWriter, r *http.Request) {
		if !limit(w, reads, "blog|"+clientIP(r)) {
			return
		}
		rows, err := s.db.Query(r.Context(), `SELECT `+blogCols+` FROM blog_posts WHERE slug = $1 AND status = 'published'`, chi.URLParam(r, "slug"))
		if err != nil {
			serverError(w, r, err)
			return
		}
		post, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[blogPost])
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "Artículo no encontrado.")
			return
		}
		if err != nil {
			serverError(w, r, err)
			return
		}
		// para seguir leyendo: primero de la misma categoría, luego los más recientes
		rows, err = s.db.Query(r.Context(), `SELECT `+blogCardCols+` FROM blog_posts
			WHERE status = 'published' AND id <> $1 ORDER BY (category = $2) DESC, published_at DESC LIMIT 3`, post.ID, post.Category)
		if err != nil {
			serverError(w, r, err)
			return
		}
		related, err := pgx.CollectRows(rows, pgx.RowToStructByName[blogCard])
		if err != nil {
			serverError(w, r, err)
			return
		}
		post.Topic = ""
		w.Header().Set("Cache-Control", "public, max-age=300")
		writeJSON(w, http.StatusOK, map[string]any{"post": post, "related": related})
	})
}

// blogSitemapURLs lists the published posts for the sitemap.
func (s *Server) blogSitemapURLs(ctx context.Context, base string) ([]string, error) {
	rows, err := s.db.Query(ctx, `SELECT slug FROM blog_posts WHERE status = 'published' ORDER BY published_at DESC`)
	if err != nil {
		return nil, err
	}
	slugs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	urls := make([]string, len(slugs))
	for i, slug := range slugs {
		urls[i] = base + "/blog/" + slug
	}
	return urls, nil
}

// ---------------------------------------------------------------------------
// Platform panel
// ---------------------------------------------------------------------------

func (s *Server) platformBlog(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `SELECT `+blogCardCols+` FROM blog_posts ORDER BY created_at DESC`)
	if err != nil {
		serverError(w, r, err)
		return
	}
	posts, err := pgx.CollectRows(rows, pgx.RowToStructByName[blogCard])
	if err != nil {
		serverError(w, r, err)
		return
	}
	used := map[string]bool{}
	tr, err := s.db.Query(r.Context(), `SELECT topic FROM blog_posts WHERE topic <> ''`)
	if err != nil {
		serverError(w, r, err)
		return
	}
	topics, err := pgx.CollectRows(tr, pgx.RowTo[string])
	if err != nil {
		serverError(w, r, err)
		return
	}
	for _, t := range topics {
		used[t] = true
	}
	left := 0
	for _, t := range blogTopics {
		if !used[t.Key] {
			left++
		}
	}
	next := nextBlogTopic(used)
	busy, lastErr := s.blog.snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"posts":        posts,
		"ai_ready":     s.cfg.GeminiAPIKey != "",
		"autopilot":    s.cfg.BlogAutopilot && s.cfg.GeminiAPIKey != "",
		"auto_publish": s.cfg.BlogAutoPublish,
		"next_run":     s.nextBlogRun(r.Context(), time.Now()),
		"next_topic":   next.Keyword,
		"topics_left":  left,
		"generating":   busy,
		"last_error":   lastErr,
	})
}

// platformBlogPost returns one post in full, drafts included, so the team can read it before publishing.
func (s *Server) platformBlogPost(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusBadRequest, "Artículo inválido.")
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT `+blogCols+` FROM blog_posts WHERE id = $1`, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	post, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[blogPost])
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Artículo no encontrado.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"post": post})
}

// platformBlogGenerate starts writing an article in the background (it takes longer than a request may last).
func (s *Server) platformBlogGenerate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Topic   string `json:"topic"`
		Publish bool   `json:"publish"`
	}
	if !decode(w, r, &req) || !s.aiReady(w) {
		return
	}
	topic := clip(strings.TrimSpace(req.Topic), 300)
	if !s.blog.start() {
		writeError(w, http.StatusConflict, "Ya se está escribiendo un artículo. Espera a que termine.")
		return
	}
	p := principalFrom(r.Context())
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		post, err := s.writeBlogPost(ctx, topic, req.Publish)
		s.blog.finish(err)
		if err != nil {
			log.Printf("blog: artículo pedido por %s: %v", p.Username, err)
			return
		}
		log.Printf("blog: %s pidió el artículo %q (%s)", p.Username, post.Title, post.Status)
	}()
	writeJSON(w, http.StatusAccepted, map[string]any{"generating": true})
}

func (s *Server) platformBlogUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Status string `json:"status"`
	}
	if !validUUID(id) {
		writeError(w, http.StatusBadRequest, "Artículo inválido.")
		return
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Status != "published" && req.Status != "draft" {
		writeError(w, http.StatusBadRequest, "Estado inválido.")
		return
	}
	// la fecha de publicación es la primera vez que salió; ocultar y volver a publicar no la cambia
	tag, err := s.db.Exec(r.Context(), `UPDATE blog_posts SET status = $2, updated_at = now(),
		published_at = CASE WHEN $2 = 'published' THEN coalesce(published_at, now()) ELSE published_at END WHERE id = $1`, id, req.Status)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "Artículo no encontrado.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) platformBlogDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusBadRequest, "Artículo inválido.")
		return
	}
	tag, err := s.db.Exec(r.Context(), `DELETE FROM blog_posts WHERE id = $1`, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "Artículo no encontrado.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
