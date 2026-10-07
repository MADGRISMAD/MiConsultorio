package api_test

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// secCode computes the current authenticator code the way an app would, from the otpauth URI's secret.
func secCode(t *testing.T, secret string, offset int64) string {
	t.Helper()
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		t.Fatal(err)
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(time.Now().Unix()/30+offset))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	n := (uint32(sum[off]&0x7f) << 24) | uint32(sum[off+1])<<16 | uint32(sum[off+2])<<8 | uint32(sum[off+3])
	return fmt.Sprintf("%06d", n%1000000)
}

// secEnable turns two-step verification on for the signed-in client and returns its secret and recovery codes.
func secEnable(t *testing.T, c *client) (string, []string) {
	t.Helper()
	setup := c.expect(200, "POST", "/api/me/2fa/setup", nil)
	secret := setup["secret"].(string)
	u, err := url.Parse(setup["otpauth_uri"].(string))
	if err != nil || u.Scheme != "otpauth" || u.Host != "totp" || u.Query().Get("secret") != secret || u.Query().Get("issuer") != "Caresia" {
		t.Fatalf("otpauth uri: %v %v", setup["otpauth_uri"], err)
	}
	c.expect(400, "POST", "/api/me/2fa/enable", map[string]string{"code": "000000"})
	out := c.expect(200, "POST", "/api/me/2fa/enable", map[string]string{"code": secCode(t, secret, 0)})
	var codes []string
	for _, x := range out["recovery_codes"].([]any) {
		codes = append(codes, x.(string))
	}
	if len(codes) != 10 || sub(out, "session")["twoFactorEnabled"] != true {
		t.Fatalf("enable result: %v", out)
	}
	return secret, codes
}

// secFree lets the next code be accepted again (a code is single-use inside its 30 s step).
func secFree(e *env, user string) {
	e.exec(`UPDATE users SET totp_last_step = 0 WHERE username = $1`, user)
}

func secChallenge(t *testing.T, e *env, user string) (*client, string) {
	t.Helper()
	c := e.anon()
	out := c.expect(200, "POST", "/api/login", map[string]string{"identifier": user, "password": pw})
	if out["needs_2fa"] != true || out["session"] != nil || out["challenge"] == "" {
		t.Fatalf("login with 2FA: %v", out)
	}
	c.expect(401, "GET", "/api/session", nil) // a correct password alone is not a session
	return c, out["challenge"].(string)
}

func TestTwoFactorLoginFlow(t *testing.T) {
	e := setup(t)
	c := e.login("doc_a")
	secret, codes := secEnable(t, c)
	c.expect(409, "POST", "/api/me/2fa/setup", nil)

	// the challenge is not a session
	anon, ch := secChallenge(t, e, "doc_a")
	req, _ := http.NewRequest("GET", e.srv.URL+"/api/session", nil)
	req.AddCookie(&http.Cookie{Name: "caresia_session", Value: ch})
	if res, err := http.DefaultClient.Do(req); err != nil || res.StatusCode != 401 {
		t.Fatalf("challenge accepted as a cookie: %v %v", res, err)
	}
	anon.expect(401, "POST", "/api/login/2fa", map[string]string{"challenge": ch, "code": "123456"})
	anon.expect(401, "POST", "/api/login/2fa", map[string]string{"challenge": "garbage", "code": secCode(t, secret, 0)})
	anon.expect(401, "POST", "/api/login/2fa", map[string]string{"challenge": ch})

	secFree(e, "doc_a")
	good := secCode(t, secret, 0)
	out := anon.expect(200, "POST", "/api/login/2fa", map[string]string{"challenge": ch, "code": good})
	if s := sub(out, "session"); s["username"] != "doc_a" || s["twoFactorEnabled"] != true || s["mustSetup2fa"] != false {
		t.Fatalf("session: %v", s)
	}
	anon.expect(200, "GET", "/api/session", nil)

	// the same code never works twice
	anon2, ch2 := secChallenge(t, e, "doc_a")
	anon2.expect(401, "POST", "/api/login/2fa", map[string]string{"challenge": ch2, "code": good})

	// recovery code: once
	anon3, ch3 := secChallenge(t, e, "doc_a")
	out = anon3.expect(200, "POST", "/api/login/2fa", map[string]string{"challenge": ch3, "recovery_code": strings.ToUpper(codes[0])})
	if out["recovery_codes_left"].(float64) != 9 {
		t.Fatalf("left: %v", out["recovery_codes_left"])
	}
	anon4, ch4 := secChallenge(t, e, "doc_a")
	anon4.expect(401, "POST", "/api/login/2fa", map[string]string{"challenge": ch4, "recovery_code": codes[0]})

	// a wrong password never reaches the second step
	e.anon().expect(401, "POST", "/api/login", map[string]string{"identifier": "doc_a", "password": "nope-nope-1"})

	// regenerating needs password and code, and kills the old codes
	secFree(e, "doc_a")
	c.expect(400, "POST", "/api/me/2fa/recovery-codes", map[string]string{"password": "wrong-pass", "code": secCode(t, secret, 0)})
	secFree(e, "doc_a")
	fresh := c.expect(200, "POST", "/api/me/2fa/recovery-codes", map[string]string{"password": pw, "code": secCode(t, secret, 0)})
	if len(fresh["recovery_codes"].([]any)) != 10 {
		t.Fatal("new codes")
	}
	anon5, ch5 := secChallenge(t, e, "doc_a")
	anon5.expect(401, "POST", "/api/login/2fa", map[string]string{"challenge": ch5, "recovery_code": codes[1]})

	// disabling: password + code, then plain login again
	c.expect(400, "POST", "/api/me/2fa/disable", map[string]string{"password": pw, "code": "000000"})
	secFree(e, "doc_a")
	c.expect(200, "POST", "/api/me/2fa/disable", map[string]string{"password": pw, "code": secCode(t, secret, 0)})
	e.login("doc_a").expect(200, "GET", "/api/session", nil)
	var n int
	if err := e.pool.QueryRow(t.Context(), `SELECT count(*) FROM totp_recovery_codes WHERE user_id = $1`, e.userID("doc_a")).Scan(&n); err != nil || n != 0 {
		t.Fatalf("recovery codes left after disabling: %d %v", n, err)
	}
}

func TestTwoFactorSecretEncryptedAndPlatformUsers(t *testing.T) {
	e := setup(t)
	c := e.login("root")
	secret, _ := secEnable(t, c)
	var enc []byte
	if err := e.pool.QueryRow(t.Context(), `SELECT totp_secret_enc FROM users WHERE username = 'root'`).Scan(&enc); err != nil || len(enc) < 30 || strings.Contains(string(enc), secret) {
		t.Fatalf("secret must be stored encrypted: %v", err)
	}
	_, ch := secChallenge(t, e, "root")
	secFree(e, "root")
	e.anon().expect(200, "POST", "/api/login/2fa", map[string]string{"challenge": ch, "code": secCode(t, secret, 0)})
}

func TestTwoFactorBruteForceLocksOut(t *testing.T) {
	e := setup(t)
	secEnable(t, e.login("doc_a"))
	c, ch := secChallenge(t, e, "doc_a")
	for i := 0; i < 8; i++ {
		c.expect(401, "POST", "/api/login/2fa", map[string]string{"challenge": ch, "code": fmt.Sprintf("00000%d", i)})
	}
	c.expect(429, "POST", "/api/login/2fa", map[string]string{"challenge": ch, "code": "999999"})
}

func TestTwoFactorChallengeDiesWithPasswordChange(t *testing.T) {
	e := setup(t)
	secret, _ := secEnable(t, e.login("doc_a"))
	c, ch := secChallenge(t, e, "doc_a")
	e.exec(`UPDATE users SET token_version = token_version + 1 WHERE username = 'doc_a'`)
	secFree(e, "doc_a")
	c.expect(401, "POST", "/api/login/2fa", map[string]string{"challenge": ch, "code": secCode(t, secret, 0)})
}

func TestTwoFactorPolicy(t *testing.T) {
	e := setup(t)
	admin := e.login("admin_a")
	if got := admin.expect(200, "GET", "/api/security/policy", nil)["require_2fa"]; got != "none" {
		t.Fatalf("default policy %v", got)
	}
	admin.expect(400, "PUT", "/api/security/policy", map[string]string{"require_2fa": "everyone"})
	e.login("doc_a").expect(403, "PUT", "/api/security/policy", map[string]string{"require_2fa": "all"})
	e.login("doc_a").expect(403, "GET", "/api/security/members", nil)
	e.login("root").expect(403, "GET", "/api/security/policy", nil)

	admin.expect(200, "PUT", "/api/security/policy", map[string]string{"require_2fa": "clinical"})
	var col string
	if err := e.pool.QueryRow(t.Context(), `SELECT require_2fa FROM clinics WHERE id = $1`, e.clinicA).Scan(&col); err != nil || col != "clinical" {
		t.Fatalf("column: %q %v", col, err)
	}

	doc := e.login("doc_a")
	s := sub(doc.expect(200, "GET", "/api/session", nil), "session")
	if s["mustSetup2fa"] != true || s["twoFactorEnabled"] != false {
		t.Fatalf("session: %v", s)
	}
	for _, p := range []string{"/api/patients", "/api/patients/lookup?q=a", "/api/files/00000000-0000-0000-0000-000000000000", "/api/prescriptions/00000000-0000-0000-0000-000000000000"} {
		if out := doc.expect(403, "GET", p, nil); out["code"] != "SETUP_2FA" {
			t.Fatalf("%s: %v", p, out)
		}
	}
	doc.expect(200, "GET", "/api/appointments", nil) // the rest of the app keeps working
	// reception is outside "clinical"; clinic B has no policy
	e.login("recep_a").expect(200, "GET", "/api/patients/lookup?q=a", nil)
	e.login("doc_b").expect(200, "GET", "/api/patients", nil)

	secEnable(t, doc)
	doc.expect(200, "GET", "/api/patients", nil)
	if s := sub(doc.expect(200, "GET", "/api/session", nil), "session"); s["mustSetup2fa"] != false {
		t.Fatalf("after enabling: %v", s)
	}
}

func TestAdminResetsTwoFactorOfMember(t *testing.T) {
	e := setup(t)
	doc := e.login("doc_a")
	secEnable(t, doc)
	secEnable(t, e.login("doc_b"))
	admin := e.login("admin_a")

	members := admin.expect(200, "GET", "/api/security/members", nil)["members"].([]any)
	seen := map[string]bool{}
	for _, m := range members {
		mm := m.(map[string]any)
		seen[mm["username"].(string)] = mm["two_factor_enabled"].(bool)
	}
	if len(members) != 4 || !seen["doc_a"] || seen["admin_a"] {
		t.Fatalf("members: %v", seen)
	}

	// another clinic's people are out of reach
	admin.expect(404, "POST", "/api/security/members/"+e.userID("doc_b")+"/2fa/reset", nil)
	e.login("recep_a").expect(403, "POST", "/api/security/members/"+e.userID("doc_a")+"/2fa/reset", nil)
	admin.expect(400, "POST", "/api/security/members/"+e.userID("admin_a")+"/2fa/reset", nil)

	admin.expect(200, "POST", "/api/security/members/"+e.userID("doc_a")+"/2fa/reset", nil)
	doc.expect(401, "GET", "/api/session", nil) // their sessions ended
	e.login("doc_a").expect(200, "GET", "/api/session", nil)
	var audits, still int
	e.pool.QueryRow(t.Context(), `SELECT count(*) FROM activity_log WHERE clinic_id = $1 AND type = '2fa_reset'`, e.clinicA).Scan(&audits)
	e.pool.QueryRow(t.Context(), `SELECT count(*) FROM users WHERE username = 'doc_b' AND totp_enabled`).Scan(&still)
	if audits != 1 || still != 1 {
		t.Fatalf("audits %d, clinic B untouched %d", audits, still)
	}
}

func TestPatientExport(t *testing.T) {
	e := setup(t)
	doc := e.login("doc_a")
	id := sub(doc.expect(201, "POST", "/api/patients", person(nil)), "patient")["id"].(string)
	doc.expect(201, "POST", "/api/patients/"+id+"/encounters", map[string]any{"kind": "consulta", "reason": "Dolor de cabeza"})
	e.exec(`INSERT INTO attachments (clinic_id, patient_id, mime, size_bytes, sha256, storage_key, title) VALUES ($1,$2,'image/png',1,'x','secret/key/1','Rayos X')`, e.clinicA, id)

	out := doc.expect(200, "GET", "/api/patients/"+id+"/export", nil)
	if sub(out, "patient")["names"] != "Jorge" || len(out["encounters"].([]any)) != 1 || len(out["files"].([]any)) != 1 {
		t.Fatalf("export: %v", out)
	}
	for _, k := range []string{"prescriptions", "vaccinations", "treatment_plans", "consents", "appointments", "access_log", "charts", "clinic"} {
		if _, ok := out[k]; !ok {
			t.Errorf("missing %s", k)
		}
	}
	if f := out["files"].([]any)[0].(map[string]any); f["title"] != "Rayos X" || f["storage_key"] != nil {
		t.Fatalf("file metadata must not leak the storage key: %v", f)
	}

	var exports, audits int
	e.pool.QueryRow(t.Context(), `SELECT count(*) FROM record_access WHERE patient_id = $1 AND action = 'export'`, id).Scan(&exports)
	e.pool.QueryRow(t.Context(), `SELECT count(*) FROM activity_log WHERE clinic_id = $1 AND type = 'patient_export'`, e.clinicA).Scan(&audits)
	if exports != 1 || audits != 1 {
		t.Fatalf("exports %d audits %d", exports, audits)
	}
	e.login("admin_a").expect(200, "GET", "/api/patients/"+id+"/export", nil)
	e.login("recep_a").expect(403, "GET", "/api/patients/"+id+"/export", nil)
	e.login("cash_a").expect(403, "GET", "/api/patients/"+id+"/export", nil)
	e.login("admin_b").expect(404, "GET", "/api/patients/"+id+"/export", nil)
	e.login("doc_b").expect(404, "GET", "/api/patients/"+id+"/export", nil)
	e.anon().expect(401, "GET", "/api/patients/"+id+"/export", nil)
	doc.expect(404, "GET", "/api/patients/not-a-uuid/export", nil)
}

func TestHealth(t *testing.T) {
	e := setup(t)
	out := e.anon().expect(200, "GET", "/api/health", nil)
	if out["status"] != "ok" || sub(out, "db")["ok"] != true || sub(out, "migrations")["ok"] != true {
		t.Fatalf("health: %v", out)
	}
	if out["backup"] != nil {
		t.Fatal("no backup file configured")
	}
	e.anon().expect(401, "GET", "/api/platform/health", nil)
	e.login("admin_a").expect(403, "GET", "/api/platform/health", nil)
	e.login("help").expect(403, "GET", "/api/platform/health", nil)
	ph := e.login("root").expect(200, "GET", "/api/platform/health", nil)
	if ph["active_clinics"].(float64) != 2 || ph["db_size_bytes"].(float64) <= 0 {
		t.Fatalf("platform health: %v", ph)
	}
}
