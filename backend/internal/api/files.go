package api

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Attachments of a patient's record. The bytes are encrypted at rest with AES-256-GCM under a key derived from
// TOKEN_ENC_KEY, stored under random keys (never the user's file name), and checked against their SHA-256 on every read.
// Clinical files are archived with a reason, never deleted (NOM-004-SSA3-2012).

const (
	defaultMaxUpload = 15 << 20
	multipartSlack   = 1 << 20 // form fields and part headers on top of the file itself
	maxFieldBytes    = 8 << 10
	maxTitleRunes    = 120
	maxNoteRunes     = 1000
	maxNameRunes     = 120
	encVersion       = 1
)

var fileKinds = []string{"xray", "lab", "ultrasound", "consent", "photo", "document", "other"}

// storageQuota is the space each plan includes for attachments.
var storageQuota = map[string]int64{"basico": 1 << 30, "crecimiento": 5 << 30, "pro": 20 << 30}

// uploadLimiter bounds uploads per user so a stolen session cannot be used to flood the disk.
var uploadLimiter = newRateLimiter(60, 10*time.Minute)

var storageKeyRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/[0-9a-f]{2}/[0-9a-f]{32}$`)

type attachment struct {
	ID             string     `json:"id"`
	PatientID      string     `json:"patient_id"`
	EncounterID    *string    `json:"encounter_id"`
	Kind           string     `json:"kind"`
	Title          string     `json:"title"`
	Note           string     `json:"note"`
	OriginalName   string     `json:"original_name"`
	Mime           string     `json:"mime"`
	SizeBytes      int64      `json:"size_bytes"`
	SHA256         string     `json:"sha256"`
	UploadedByName string     `json:"uploaded_by_name"`
	CreatedAt      time.Time  `json:"created_at"`
	ArchivedAt     *time.Time `json:"archived_at"`
	ArchivedBy     string     `json:"archived_by_name"`
	ArchiveReason  string     `json:"archive_reason"`
	CanArchive     bool       `json:"can_archive"`

	uploadedBy string
	storageKey string
}

const attachmentCols = `id, patient_id::text, encounter_id::text, kind, title, note, original_name, mime, size_bytes, sha256, storage_key,
	coalesce(uploaded_by::text, ''), uploaded_by_name, created_at, archived_at, archived_by_name, archive_reason`

func scanAttachment(row pgx.Row, p *Principal) (attachment, error) {
	var a attachment
	err := row.Scan(&a.ID, &a.PatientID, &a.EncounterID, &a.Kind, &a.Title, &a.Note, &a.OriginalName, &a.Mime, &a.SizeBytes, &a.SHA256, &a.storageKey,
		&a.uploadedBy, &a.UploadedByName, &a.CreatedAt, &a.ArchivedAt, &a.ArchivedBy, &a.ArchiveReason)
	if err == nil {
		a.CanArchive = a.ArchivedAt == nil && canArchiveFile(p, a)
	}
	return a, err
}

func canArchiveFile(p *Principal, a attachment) bool {
	if hasPermission(p.Permissions, PermAdminUsers) {
		return true
	}
	return hasPermission(p.Permissions, PermAdminHistorials) && a.uploadedBy == p.UserID
}

func (s *Server) maxUpload() int64 {
	if s.cfg.MaxUploadBytes > 0 {
		return s.cfg.MaxUploadBytes
	}
	return defaultMaxUpload
}

// ---- content detection ------------------------------------------------------------------------------------------

// sniffFile identifies an allowed file by its real content, never by the name or the declared type.
// Anything else (SVG, HTML, scripts, executables, archives...) is refused.
func sniffFile(data []byte) (mimeType, ext string, ok bool) {
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	detected := http.DetectContentType(head)
	switch {
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF && detected == "image/jpeg":
		return "image/jpeg", ".jpg", true
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) && detected == "image/png":
		return "image/png", ".png", true
	case (bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a"))) && detected == "image/gif":
		return "image/gif", ".gif", true
	case len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) && detected == "image/webp":
		return "image/webp", ".webp", true
	case bytes.HasPrefix(data, []byte("%PDF-")) && detected == "application/pdf":
		return "application/pdf", ".pdf", true
	case len(data) >= 132 && bytes.Equal(data[128:132], []byte("DICM")):
		return "application/dicom", ".dcm", true
	}
	return "", "", false
}

func isInline(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/") || mimeType == "application/pdf"
}

// cleanFileName makes the name the user gave safe to show and to put in a header: no path, no control or
// bidirectional characters, bounded length, and an extension that matches the real content.
func cleanFileName(name, ext string) string {
	name = strings.ReplaceAll(name, `\`, "/")
	name = path.Base(name)
	name = strings.Map(func(r rune) rune {
		switch {
		case unicode.IsControl(r), unicode.Is(unicode.Cf, r), r == '"', r == '<', r == '>', r == ':', r == '|', r == '?', r == '*', r == 0xFFFD:
			return -1
		}
		return r
	}, name)
	name = strings.Trim(strings.TrimSpace(name), ".")
	if name == "" || name == "/" {
		name = "archivo"
	}
	have := strings.ToLower(filepath.Ext(name))
	ok := have == ext || (ext == ".jpg" && have == ".jpeg") || (ext == ".dcm" && have == ".dicom")
	if !ok {
		name = strings.TrimSuffix(name, filepath.Ext(name)) + ext
		if name == ext {
			name = "archivo" + ext
		}
	}
	if utf8.RuneCountInString(name) > maxNameRunes {
		stem := []rune(strings.TrimSuffix(name, filepath.Ext(name)))
		name = string(stem[:maxNameRunes-len(ext)]) + ext
	}
	return name
}

// ---- encrypted storage ------------------------------------------------------------------------------------------

func (s *Server) filesAEAD() (cipher.AEAD, error) {
	if len(s.cfg.TokenEncKey) == 0 {
		return nil, errors.New("TOKEN_ENC_KEY is not set")
	}
	mac := hmac.New(sha256.New, s.cfg.TokenEncKey)
	mac.Write([]byte("attachments-v1"))
	block, err := aes.NewCipher(mac.Sum(nil))
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (s *Server) blobPath(key string) (string, error) {
	if s.cfg.UploadsDir == "" || !storageKeyRe.MatchString(key) {
		return "", errors.New("invalid storage key")
	}
	return filepath.Join(s.cfg.UploadsDir, filepath.FromSlash(key)), nil
}

func newStorageKey(clinicID string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	id := hex.EncodeToString(b)
	return clinicID + "/" + id[:2] + "/" + id, nil
}

// putBlob encrypts data and writes it atomically (temporary file in the same directory, then rename).
func (s *Server) putBlob(key string, data []byte) error {
	gcm, err := s.filesAEAD()
	if err != nil {
		return err
	}
	dest, err := s.blobPath(key)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	out := append([]byte{encVersion}, nonce...)
	out = gcm.Seal(out, nonce, data, []byte(key))

	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	fail := func(err error) error {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		return fail(err)
	}
	if _, err := tmp.Write(out); err != nil {
		return fail(err)
	}
	if err := tmp.Sync(); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, dest); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

var errBlobMissing = errors.New("file missing from storage")

// getBlob reads and decrypts a file and verifies it against the stored SHA-256.
func (s *Server) getBlob(key, wantSum string) ([]byte, error) {
	gcm, err := s.filesAEAD()
	if err != nil {
		return nil, err
	}
	src, err := s.blobPath(key)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(src)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errBlobMissing
	}
	if err != nil {
		return nil, err
	}
	if len(raw) < 1+gcm.NonceSize() || raw[0] != encVersion {
		return nil, errors.New("stored file has an unknown format")
	}
	plain, err := gcm.Open(nil, raw[1:1+gcm.NonceSize()], raw[1+gcm.NonceSize():], []byte(key))
	if err != nil {
		return nil, errors.New("stored file failed authentication")
	}
	sum := sha256.Sum256(plain)
	if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(wantSum)) != 1 {
		return nil, errors.New("stored file does not match its checksum")
	}
	return plain, nil
}

func (s *Server) removeBlob(key string) {
	if p, err := s.blobPath(key); err == nil {
		_ = os.Remove(p)
	}
}

// ---- quota ------------------------------------------------------------------------------------------------------

func quotaFor(p *Principal) int64 {
	plan := "basico"
	if p.Billing != nil {
		plan = p.Billing.Plan
	}
	if q, ok := storageQuota[plan]; ok {
		return q
	}
	return storageQuota["basico"]
}

type queryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func clinicUsage(ctx context.Context, q queryer, clinicID string) (int64, error) {
	var used int64
	err := q.QueryRow(ctx, `SELECT coalesce(sum(size_bytes), 0)::bigint FROM attachments WHERE clinic_id = $1`, clinicID).Scan(&used)
	return used, err
}

func quotaError() *httpError {
	e := fail(http.StatusRequestEntityTooLarge, "Tu consultorio alcanzó el espacio de archivos de su plan. Archiva no libera espacio (los archivos clínicos se conservan); contacta a Caresia para ampliar tu plan.")
	e.Code = "QUOTA_EXCEEDED"
	return e
}

// ---- handlers ---------------------------------------------------------------------------------------------------

func (s *Server) mountFilesRoutes(r chi.Router) {
	r.With(require(PermAdminHistorials)).Post("/patients/{id}/files", s.uploadFile)
	r.With(require(PermNavHistorials, PermAdminHistorials)).Get("/patients/{id}/files", s.listFiles)
	r.With(require(PermNavHistorials, PermAdminHistorials)).Get("/files/{id}", s.downloadFile)
	r.With(require(PermAdminHistorials)).Post("/files/{id}/archive", s.archiveFile)
}

// activePatient checks the patient belongs to the clinic. archivedOK allows reading an archived record.
func (s *Server) activePatient(ctx context.Context, clinicID, id string, archivedOK bool) error {
	var archived *time.Time
	err := s.db.QueryRow(ctx, `SELECT archived_at FROM patients WHERE clinic_id=$1 AND id=$2`, clinicID, id).Scan(&archived)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(http.StatusNotFound, "Paciente no encontrado.")
	}
	if err != nil {
		return err
	}
	if archived != nil && !archivedOK {
		return fail(http.StatusConflict, "El expediente está archivado: no se pueden agregar archivos.")
	}
	return nil
}

func (s *Server) listFiles(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	archived := r.URL.Query().Get("archived") == "1"
	if archived && !hasPermission(p.Permissions, PermAdminUsers) {
		writeError(w, http.StatusForbidden, "Solo el administrador puede ver los archivos archivados.")
		return
	}
	if err := s.activePatient(r.Context(), p.ClinicID, id, true); err != nil {
		writeFailure(w, r, err)
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT `+attachmentCols+` FROM attachments
		WHERE clinic_id=$1 AND patient_id=$2 AND (archived_at IS NOT NULL) = $3 ORDER BY created_at DESC LIMIT 500`, p.ClinicID, id, archived)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	list := []attachment{}
	for rows.Next() {
		a, err := scanAttachment(rows, p)
		if err != nil {
			serverError(w, r, err)
			return
		}
		list = append(list, a)
	}
	if rows.Err() != nil {
		serverError(w, r, rows.Err())
		return
	}
	rows.Close()
	used, err := clinicUsage(r.Context(), s.db, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": list, "usage": map[string]any{"used_bytes": used, "quota_bytes": quotaFor(p)}, "max_upload_bytes": s.maxUpload()})
}

type uploadForm struct {
	kind, title, note, encounter string
	name                         string
	data                         []byte
	hasFile                      bool
}

func tooBig(err error) bool {
	var mb *http.MaxBytesError
	return errors.As(err, &mb)
}

// readUpload streams the multipart body into memory, bounded by the configured limit.
func (s *Server) readUpload(w http.ResponseWriter, r *http.Request) (*uploadForm, error) {
	limit := s.maxUpload()
	tooLarge := func() error {
		e := fail(http.StatusRequestEntityTooLarge, fmt.Sprintf("El archivo supera el límite de %d MB.", limit>>20))
		e.Code = "FILE_TOO_LARGE"
		return e
	}
	if r.ContentLength > limit+multipartSlack {
		return nil, tooLarge()
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit+multipartSlack)
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, fail(http.StatusBadRequest, "Envía el archivo como formulario (multipart).")
	}
	f := &uploadForm{}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			if tooBig(err) {
				return nil, tooLarge()
			}
			return nil, fail(http.StatusBadRequest, "No se pudo leer el formulario.")
		}
		if err := f.take(part, limit); err != nil {
			if tooBig(err) {
				return nil, tooLarge()
			}
			return nil, err
		}
	}
	return f, nil
}

func (f *uploadForm) take(part *multipart.Part, limit int64) error {
	defer part.Close()
	if part.FormName() == "file" {
		if f.hasFile {
			return fail(http.StatusBadRequest, "Sube un archivo a la vez.")
		}
		data, err := io.ReadAll(io.LimitReader(part, limit+1))
		if err != nil {
			return err
		}
		if int64(len(data)) > limit {
			e := fail(http.StatusRequestEntityTooLarge, fmt.Sprintf("El archivo supera el límite de %d MB.", limit>>20))
			e.Code = "FILE_TOO_LARGE"
			return e
		}
		f.data, f.name, f.hasFile = data, part.FileName(), true
		return nil
	}
	val, err := io.ReadAll(io.LimitReader(part, maxFieldBytes+1))
	if err != nil {
		return err
	}
	if len(val) > maxFieldBytes {
		return fail(http.StatusBadRequest, "Uno de los campos es demasiado largo.")
	}
	s := strings.TrimSpace(string(val))
	switch part.FormName() {
	case "kind":
		f.kind = s
	case "title":
		f.title = s
	case "note":
		f.note = s
	case "encounter_id":
		f.encounter = s
	}
	return nil
}

func cleanText(s string, max int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' {
			return ' '
		}
		return r
	}, strings.TrimSpace(s))
	if utf8.RuneCountInString(s) > max {
		return string([]rune(s)[:max])
	}
	return s
}

func (s *Server) uploadFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	if !uploadLimiter.allow(p.UserID) {
		writeJSON(w, http.StatusTooManyRequests, errorBody{Code: "RATE_LIMITED", Message: "Demasiadas subidas seguidas. Espera unos minutos e inténtalo de nuevo."})
		return
	}
	if err := s.activePatient(r.Context(), p.ClinicID, id, false); err != nil {
		writeFailure(w, r, err)
		return
	}
	if s.cfg.UploadsDir == "" {
		writeError(w, http.StatusServiceUnavailable, "El almacenamiento de archivos no está configurado.")
		return
	}
	f, err := s.readUpload(w, r)
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	if !f.hasFile || len(f.data) == 0 {
		writeError(w, http.StatusBadRequest, "Selecciona un archivo.")
		return
	}
	uploadLimiter.fail(p.UserID)

	if f.kind == "" {
		f.kind = "document"
	}
	if !slices.Contains(fileKinds, f.kind) {
		writeError(w, http.StatusBadRequest, "Tipo de archivo inválido.")
		return
	}
	mimeType, ext, ok := sniffFile(f.data)
	if !ok {
		writeJSON(w, http.StatusUnsupportedMediaType, errorBody{Code: "UNSUPPORTED_TYPE", Message: "Tipo de archivo no permitido. Usa imágenes (JPG, PNG, WebP, GIF), PDF o DICOM."})
		return
	}
	var encounter any
	if f.encounter != "" {
		var ok bool
		if validUUID(f.encounter) {
			err := s.db.QueryRow(r.Context(), `SELECT true FROM encounters WHERE clinic_id=$1 AND patient_id=$2 AND id=$3`, p.ClinicID, id, f.encounter).Scan(&ok)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				serverError(w, r, err)
				return
			}
		}
		if !ok {
			writeError(w, http.StatusBadRequest, "La consulta indicada no existe en este expediente.")
			return
		}
		encounter = f.encounter
	}
	name := cleanFileName(f.name, ext)
	title := cleanText(f.title, maxTitleRunes)
	if title == "" {
		title = strings.TrimSuffix(name, filepath.Ext(name))
	}
	note := cleanText(f.note, maxNoteRunes)

	// Cheap early refusal; the authoritative check runs again under a lock when the row is inserted.
	quota := quotaFor(p)
	if used, err := clinicUsage(r.Context(), s.db, p.ClinicID); err != nil {
		serverError(w, r, err)
		return
	} else if used+int64(len(f.data)) > quota {
		writeFailure(w, r, quotaError())
		return
	}

	key, err := newStorageKey(p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if err := s.putBlob(key, f.data); err != nil {
		serverError(w, r, err)
		return
	}
	sum := sha256.Sum256(f.data)
	var out attachment
	err = inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "attachments:"+p.ClinicID); err != nil {
			return err
		}
		used, err := clinicUsage(r.Context(), tx, p.ClinicID)
		if err != nil {
			return err
		}
		if used+int64(len(f.data)) > quota {
			return quotaError()
		}
		out, err = scanAttachment(tx.QueryRow(r.Context(), `
			INSERT INTO attachments (clinic_id, patient_id, encounter_id, kind, title, note, original_name, mime, size_bytes, sha256, storage_key, uploaded_by, uploaded_by_name)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING `+attachmentCols,
			p.ClinicID, id, encounter, f.kind, title, note, name, mimeType, len(f.data), hex.EncodeToString(sum[:]), key, p.UserID, p.actorName()), p)
		if err != nil {
			return err
		}
		audit(r.Context(), tx, p.ClinicID, p, "file_upload", "Subió un archivo al expediente de un paciente", map[string]any{"patient": id, "file": out.ID, "kind": f.kind, "mime": mimeType, "size": len(f.data)})
		return nil
	})
	if err != nil {
		s.removeBlob(key)
		writeFailure(w, r, err)
		return
	}
	s.logAccess(r.Context(), p.ClinicID, id, p, "file_upload")
	writeJSON(w, http.StatusCreated, map[string]any{"file": out})
}

func (s *Server) loadAttachment(ctx context.Context, p *Principal, id string) (attachment, error) {
	if !validUUID(id) {
		return attachment{}, fail(http.StatusNotFound, "Archivo no encontrado.")
	}
	a, err := scanAttachment(s.db.QueryRow(ctx, `SELECT `+attachmentCols+` FROM attachments WHERE clinic_id=$1 AND id=$2`, p.ClinicID, id), p)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, fail(http.StatusNotFound, "Archivo no encontrado.")
	}
	return a, err
}

func (s *Server) downloadFile(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	a, err := s.loadAttachment(r.Context(), p, chi.URLParam(r, "id"))
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	if a.ArchivedAt != nil && !hasPermission(p.Permissions, PermAdminUsers) {
		writeError(w, http.StatusForbidden, "Este archivo está archivado: solo el administrador puede abrirlo.")
		return
	}
	data, err := s.getBlob(a.storageKey, a.SHA256)
	if errors.Is(err, errBlobMissing) {
		logf(r, "attachment %s: %v", a.ID, err)
		writeError(w, http.StatusGone, "El archivo no está disponible en el almacenamiento. Avisa al administrador.")
		return
	}
	if err != nil {
		logf(r, "attachment %s: %v", a.ID, err)
		writeJSON(w, http.StatusInternalServerError, errorBody{Code: "INTEGRITY_ERROR", Message: "No se pudo verificar la integridad del archivo. Avisa al administrador."})
		return
	}
	download := r.URL.Query().Get("download") == "1"
	inline := isInline(a.Mime) && !download
	disp := "attachment"
	if inline {
		disp = "inline"
	}
	h := w.Header()
	h.Set("Content-Type", a.Mime)
	h.Set("Content-Disposition", mime.FormatMediaType(disp, map[string]string{"filename": a.OriginalName}))
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "private, no-store")
	h.Set("X-Frame-Options", "SAMEORIGIN") // the preview embeds PDFs from the same origin
	if a.Mime == "application/pdf" {
		// Chrome's PDF viewer does not run inside a sandboxed document; everything else is fully sandboxed.
		h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src data:; frame-ancestors 'self'")
	} else {
		h.Set("Content-Security-Policy", "sandbox; default-src 'none'; frame-ancestors 'self'")
	}
	h.Set("Content-Length", strconv.Itoa(len(data)))
	action := "file_download"
	if inline {
		action = "file_view"
	}
	s.logAccess(r.Context(), p.ClinicID, a.PatientID, p, action)
	audit(r.Context(), s.db, p.ClinicID, p, action, "Abrió un archivo del expediente de un paciente", map[string]any{"patient": a.PatientID, "file": a.ID})
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(data)
	}
}

func (s *Server) archiveFile(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	a, err := s.loadAttachment(r.Context(), p, chi.URLParam(r, "id"))
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Reason = cleanText(in.Reason, 300)
	if utf8.RuneCountInString(in.Reason) < 3 {
		writeError(w, http.StatusBadRequest, "Indica el motivo para archivar el archivo.")
		return
	}
	if !canArchiveFile(p, a) {
		writeError(w, http.StatusForbidden, "Solo el administrador o quien subió el archivo puede archivarlo.")
		return
	}
	var out attachment
	err = inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		out, err = scanAttachment(tx.QueryRow(r.Context(), `
			UPDATE attachments SET archived_at = now(), archived_by_name = $3, archive_reason = $4
			WHERE clinic_id=$1 AND id=$2 AND archived_at IS NULL RETURNING `+attachmentCols, p.ClinicID, a.ID, p.actorName(), in.Reason), p)
		if errors.Is(err, pgx.ErrNoRows) {
			return fail(http.StatusConflict, "El archivo ya estaba archivado.")
		}
		if err != nil {
			return err
		}
		audit(r.Context(), tx, p.ClinicID, p, "file_archive", "Archivó un archivo del expediente de un paciente", map[string]any{"patient": a.PatientID, "file": a.ID, "reason": in.Reason})
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	s.logAccess(r.Context(), p.ClinicID, a.PatientID, p, "file_archive")
	writeJSON(w, http.StatusOK, map[string]any{"file": out})
}
