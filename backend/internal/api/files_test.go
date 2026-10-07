package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madgrismad/miconsultorio/backend/internal/config"
)

var (
	jpegBytes  = append([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0x10, 'J', 'F', 'I', 'F', 0}, []byte(strings.Repeat("SECRET-RADIOGRAPH-PIXELS", 20))...)
	pdfBytes   = []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\nSECRET-LAB-RESULT\n%%EOF")
	dicomBytes = append(append(make([]byte, 128), []byte("DICM")...), []byte("SECRET-DICOM-DATA")...)
)

type upload struct {
	name, field string
	data        []byte
	fields      map[string]string
}

// post sends a multipart upload and returns status, JSON body.
func (c *client) upload(patientID string, u upload) (int, map[string]any) {
	c.e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range u.fields {
		_ = mw.WriteField(k, v)
	}
	field := u.field
	if field == "" {
		field = "file"
	}
	if u.data != nil {
		fw, _ := mw.CreateFormFile(field, u.name)
		_, _ = fw.Write(u.data)
	}
	_ = mw.Close()
	req, _ := http.NewRequest("POST", c.e.srv.URL+"/api/patients/"+patientID+"/files", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	res, err := c.c.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer res.Body.Close()
	return res.StatusCode, decodeBody(res.Body)
}

func decodeBody(r io.Reader) map[string]any {
	var out map[string]any
	b, _ := io.ReadAll(r)
	_ = json.Unmarshal(b, &out)
	return out
}

func (c *client) raw(method, path string) (*http.Response, []byte) {
	c.e.t.Helper()
	req, _ := http.NewRequest(method, c.e.srv.URL+path, nil)
	res, err := c.c.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res, b
}

func filesSetup(t *testing.T) (*env, string, string) {
	dir := t.TempDir()
	e := setupWith(t, func(c *config.Config) { c.UploadsDir = dir; c.MaxUploadBytes = 64 << 10 })
	doc := e.login("doc_a")
	pid := sub(doc.expect(201, "POST", "/api/patients/", person(nil)), "patient")["id"].(string)
	return e, pid, dir
}

func TestFilesUploadDownloadEncrypted(t *testing.T) {
	e, pid, dir := filesSetup(t)
	doc := e.login("doc_a")

	st, out := doc.upload(pid, upload{name: "../../etc/Rx <1>.jpeg", data: jpegBytes, fields: map[string]string{"kind": "xray", "title": "Panorámica", "note": "Pre-op"}})
	if st != 201 {
		t.Fatalf("upload: %d %v", st, out)
	}
	f := sub(out, "file")
	if f["mime"] != "image/jpeg" || f["kind"] != "xray" || f["title"] != "Panorámica" || f["original_name"] != "Rx 1.jpeg" && f["original_name"] != "Rx 1.jpg" {
		t.Fatalf("file: %v", f)
	}
	if strings.Contains(f["original_name"].(string), "/") {
		t.Fatalf("name not sanitized: %v", f["original_name"])
	}
	id := f["id"].(string)

	// on disk: random key under the clinic, no clear bytes, no user name, restricted permissions
	var found string
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			found = p
		}
		return nil
	})
	if found == "" || !strings.Contains(found, e.clinicA) || strings.Contains(found, "Rx") {
		t.Fatalf("storage path: %q", found)
	}
	raw, _ := os.ReadFile(found)
	if bytes.Contains(raw, []byte("SECRET-RADIOGRAPH")) || bytes.Contains(raw, jpegBytes[:8]) {
		t.Fatal("file is stored in clear")
	}
	if info, _ := os.Stat(found); info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode %v", info.Mode().Perm())
	}
	if info, _ := os.Stat(filepath.Dir(found)); info.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode %v", info.Mode().Perm())
	}

	// download round trip with safe headers
	res, body := doc.raw("GET", "/api/files/"+id)
	if res.StatusCode != 200 || !bytes.Equal(body, jpegBytes) {
		t.Fatalf("download: %d", res.StatusCode)
	}
	if res.Header.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(res.Header.Get("Content-Security-Policy"), "sandbox") ||
		res.Header.Get("Cache-Control") != "private, no-store" || !strings.HasPrefix(res.Header.Get("Content-Disposition"), "inline") {
		t.Fatalf("headers: %v", res.Header)
	}
	res, _ = doc.raw("GET", "/api/files/"+id+"?download=1")
	if !strings.HasPrefix(res.Header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("download disposition: %v", res.Header)
	}

	// tampering with the stored bytes is detected
	raw[len(raw)-1] ^= 0xFF
	_ = os.WriteFile(found, raw, 0o600)
	if res, _ := doc.raw("GET", "/api/files/"+id); res.StatusCode != 500 {
		t.Fatalf("tampered file served: %d", res.StatusCode)
	}

	// pdf and dicom
	st, out = doc.upload(pid, upload{name: "lab.pdf", data: pdfBytes, fields: map[string]string{"kind": "lab"}})
	pdf := sub(out, "file")
	if st != 201 || pdf["mime"] != "application/pdf" {
		t.Fatalf("pdf: %d %v", st, out)
	}
	res, _ = doc.raw("GET", "/api/files/"+pdf["id"].(string))
	if res.Header.Get("Content-Security-Policy") == "" || res.Header.Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Fatalf("pdf headers: %v", res.Header)
	}
	st, out = doc.upload(pid, upload{name: "scan.dcm", data: dicomBytes, fields: map[string]string{"kind": "xray"}})
	dcm := sub(out, "file")
	if st != 201 || dcm["mime"] != "application/dicom" {
		t.Fatalf("dicom: %d %v", st, out)
	}
	res, _ = doc.raw("GET", "/api/files/"+dcm["id"].(string))
	if !strings.HasPrefix(res.Header.Get("Content-Disposition"), "attachment") {
		t.Fatal("dicom must download, not render")
	}

	list := doc.expect(200, "GET", "/api/patients/"+pid+"/files", nil)
	if len(list["files"].([]any)) != 3 || sub(list, "usage")["used_bytes"].(float64) <= 0 {
		t.Fatalf("list: %v", list)
	}

	// access trail: record_access and audit
	var n int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM record_access WHERE patient_id=$1 AND action IN ('file_upload','file_view','file_download')`, pid).Scan(&n); err != nil || n < 5 {
		t.Fatalf("record_access rows: %d %v", n, err)
	}
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM activity_log WHERE clinic_id=$1 AND type IN ('file_upload','file_view','file_download')`, e.clinicA).Scan(&n); err != nil || n < 5 {
		t.Fatalf("audit rows: %d %v", n, err)
	}
}

func TestFilesRejectedContent(t *testing.T) {
	e, pid, _ := filesSetup(t)
	doc := e.login("doc_a")
	bad := map[string][]byte{
		"a.svg":  []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>`),
		"b.jpg":  []byte("<html><script>alert(1)</script></html>"),
		"c.pdf":  []byte("MZ\x90\x00\x03\x00\x00\x00 executable"),
		"d.png":  []byte("plain text pretending to be png"),
		"f.html": []byte("<!DOCTYPE html><html></html>"),
		"g.zip":  []byte("PK\x03\x04 zipped"),
	}
	for name, data := range bad {
		if st, out := doc.upload(pid, upload{name: name, data: data}); st != 415 {
			t.Errorf("%s accepted: %d %v", name, st, out)
		}
	}
}

func TestFilesValidationLimitsAndQuota(t *testing.T) {
	e, pid, _ := filesSetup(t)
	doc := e.login("doc_a")

	if st, _ := doc.upload(pid, upload{name: "x.jpg", data: nil}); st != 400 {
		t.Errorf("missing file: %d", st)
	}
	if st, _ := doc.upload(pid, upload{name: "x.jpg", data: jpegBytes, fields: map[string]string{"kind": "bogus"}}); st != 400 {
		t.Errorf("bad kind: %d", st)
	}
	if st, _ := doc.upload(pid, upload{name: "x.jpg", data: jpegBytes, fields: map[string]string{"encounter_id": "11111111-1111-1111-1111-111111111111"}}); st != 400 {
		t.Errorf("unknown encounter: %d", st)
	}
	big := append(append([]byte{}, jpegBytes...), make([]byte, 70<<10)...)
	if st, out := doc.upload(pid, upload{name: "big.jpg", data: big}); st != 413 || out["code"] != "FILE_TOO_LARGE" {
		t.Errorf("too big: %d %v", st, out)
	}

	// encounter of this patient is accepted
	enc := sub(doc.expect(201, "POST", "/api/patients/"+pid+"/encounters", map[string]any{"reason": "Dolor"}), "encounter")["id"].(string)
	st, out := doc.upload(pid, upload{name: "x.jpg", data: jpegBytes, fields: map[string]string{"encounter_id": enc}})
	if st != 201 || sub(out, "file")["encounter_id"] != enc {
		t.Fatalf("encounter link: %d %v", st, out)
	}

	// quota: pretend the plan's space is used
	e.exec(`UPDATE clinics SET plan = 'basico' WHERE id = $1`, e.clinicA)
	e.exec(`INSERT INTO attachments (clinic_id, patient_id, kind, mime, size_bytes, sha256, storage_key) VALUES ($1,$2,'other','application/pdf',$3,'x',$4)`,
		e.clinicA, pid, int64(1<<30), e.clinicA+"/aa/"+strings.Repeat("a", 32))
	if st, out := doc.upload(pid, upload{name: "x.jpg", data: jpegBytes}); st != 413 || out["code"] != "QUOTA_EXCEEDED" {
		t.Errorf("quota: %d %v", st, out)
	}
	e.exec(`UPDATE clinics SET plan = 'pro' WHERE id = $1`, e.clinicA)
	if st, _ := doc.upload(pid, upload{name: "x.jpg", data: jpegBytes}); st != 201 {
		t.Errorf("bigger plan: %d", st)
	}

	// archived patient takes no more files
	doc.expect(200, "POST", "/api/patients/"+pid+"/archive", map[string]any{"reason": "Cambio de consultorio"})
	if st, _ := doc.upload(pid, upload{name: "x.jpg", data: jpegBytes}); st != 409 {
		t.Errorf("archived patient: %d", st)
	}
}

func TestFilesPermissionsAndIsolation(t *testing.T) {
	e, pid, _ := filesSetup(t)
	doc, recep, cash := e.login("doc_a"), e.login("recep_a"), e.login("cash_a")
	st, out := doc.upload(pid, upload{name: "x.jpg", data: jpegBytes})
	id := sub(out, "file")["id"].(string)
	if st != 201 {
		t.Fatal(st)
	}

	// front desk and cashier see nothing clinical
	for _, c := range []*client{recep, cash} {
		c.expect(403, "GET", "/api/patients/"+pid+"/files", nil)
		c.expect(403, "GET", "/api/files/"+id, nil)
		if st, _ := c.upload(pid, upload{name: "x.jpg", data: jpegBytes}); st != 403 {
			t.Errorf("non clinical upload: %d", st)
		}
		c.expect(403, "POST", "/api/files/"+id+"/archive", map[string]any{"reason": "no"})
	}

	// another clinic can neither list nor read nor archive nor upload
	other := e.login("doc_b")
	other.expect(404, "GET", "/api/patients/"+pid+"/files", nil)
	other.expect(404, "GET", "/api/files/"+id, nil)
	other.expect(404, "POST", "/api/files/"+id+"/archive", map[string]any{"reason": "intruso"})
	if st, _ := other.upload(pid, upload{name: "x.jpg", data: jpegBytes}); st != 404 {
		t.Errorf("cross clinic upload: %d", st)
	}
	e.login("admin_b").expect(404, "GET", "/api/files/"+id, nil)
	e.anon().expect(401, "GET", "/api/files/"+id, nil)
	e.login("root").expect(403, "GET", "/api/files/"+id, nil)
	doc.expect(404, "GET", "/api/files/not-a-uuid", nil)
}

func TestFilesArchive(t *testing.T) {
	e, pid, dir := filesSetup(t)
	e.addUser(e.clinicA, "doc_a2", "doctor")
	doc, doc2, admin := e.login("doc_a"), e.login("doc_a2"), e.login("admin_a")
	_, out := doc.upload(pid, upload{name: "x.jpg", data: jpegBytes})
	id := sub(out, "file")["id"].(string)

	// a colleague who did not upload it cannot archive; a reason is required
	doc2.expect(403, "POST", "/api/files/"+id+"/archive", map[string]any{"reason": "porque sí"})
	doc.expect(400, "POST", "/api/files/"+id+"/archive", map[string]any{"reason": ""})
	got := sub(doc.expect(200, "POST", "/api/files/"+id+"/archive", map[string]any{"reason": "Imagen duplicada"}), "file")
	if got["archived_at"] == nil || got["archive_reason"] != "Imagen duplicada" || got["archived_by_name"] != "DOC_A" {
		t.Fatalf("archived: %v", got)
	}
	doc.expect(409, "POST", "/api/files/"+id+"/archive", map[string]any{"reason": "otra vez"})

	// hidden from the normal list and from doctors; admin sees it, can download it, and the bytes are kept
	if n := len(doc.expect(200, "GET", "/api/patients/"+pid+"/files", nil)["files"].([]any)); n != 0 {
		t.Fatalf("archived file still listed: %d", n)
	}
	doc.expect(403, "GET", "/api/patients/"+pid+"/files?archived=1", nil)
	doc.expect(403, "GET", "/api/files/"+id, nil)
	if n := len(admin.expect(200, "GET", "/api/patients/"+pid+"/files?archived=1", nil)["files"].([]any)); n != 1 {
		t.Fatalf("admin archived list: %d", n)
	}
	if res, body := admin.raw("GET", "/api/files/"+id); res.StatusCode != 200 || !bytes.Equal(body, jpegBytes) {
		t.Fatalf("admin download: %d", res.StatusCode)
	}
	doc.expect(405, "DELETE", "/api/files/"+id, nil)
	var count int
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			count++
		}
		return nil
	})
	if count != 1 {
		t.Fatalf("archiving must keep the file on disk, found %d", count)
	}
	var acts int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM record_access WHERE patient_id=$1 AND action='file_archive'`, pid).Scan(&acts)
	if acts != 1 {
		t.Fatalf("archive not in the access log: %d", acts)
	}

	// an admin can archive anyone's file
	_, out = doc2.upload(pid, upload{name: "y.jpg", data: jpegBytes})
	admin.expect(200, "POST", "/api/files/"+sub(out, "file")["id"].(string)+"/archive", map[string]any{"reason": "Depuración"})
}

func TestFilesRateLimit(t *testing.T) {
	e, pid, _ := filesSetup(t)
	doc := e.login("doc_a")
	last := 0
	for i := 0; i < 70 && last != 429; i++ {
		last, _ = doc.upload(pid, upload{name: "x.jpg", data: jpegBytes})
	}
	if last != 429 {
		t.Fatal("uploads are not rate limited")
	}
}
