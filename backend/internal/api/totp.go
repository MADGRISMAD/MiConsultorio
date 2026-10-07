package api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// TOTP (RFC 6238) with HMAC-SHA1, 30 s steps and 6 digits: what every authenticator app speaks.
const (
	totpPeriod = 30
	totpDigits = 6
	totpWindow = 1 // steps accepted on each side of "now" (clock drift)
	totpIssuer = "Caresia"
)

var totpB32 = base32.StdEncoding.WithPadding(base32.NoPadding)

func newTOTPSecret() ([]byte, error) {
	b := make([]byte, 20)
	_, err := rand.Read(b)
	return b, err
}

func totpStep(t time.Time) int64 { return t.Unix() / totpPeriod }

// totpCode is the code for one time step (RFC 4226 dynamic truncation).
func totpCode(secret []byte, step int64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))
	mac := hmac.New(sha1.New, secret)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	n := (uint32(sum[off]&0x7f) << 24) | uint32(sum[off+1])<<16 | uint32(sum[off+2])<<8 | uint32(sum[off+3])
	var mod uint32 = 1
	for i := 0; i < totpDigits; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", totpDigits, n%mod)
}

// totpMatch returns the step whose code equals the input, looking at the drift window. Steps at or
// before lastStep are refused, so a code already used (or an older one) never works twice.
func totpMatch(secret []byte, input string, now time.Time, lastStep int64) (int64, bool) {
	if len(input) != totpDigits {
		return 0, false
	}
	var found int64
	ok := false
	cur := totpStep(now)
	for d := int64(-totpWindow); d <= totpWindow; d++ { // no early exit: same work whatever matches
		step := cur + d
		if subtle.ConstantTimeCompare([]byte(totpCode(secret, step)), []byte(input)) == 1 && step > lastStep {
			found, ok = step, true
		}
	}
	return found, ok
}

// normalizeCode keeps digits only ("123 456" and "123-456" are common ways to type it).
func normalizeCode(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func totpURI(secret []byte, account string) string {
	label := url.PathEscape(totpIssuer + ":" + account)
	q := url.Values{}
	q.Set("secret", totpB32.EncodeToString(secret))
	q.Set("issuer", totpIssuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprint(totpDigits))
	q.Set("period", fmt.Sprint(totpPeriod))
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// ---- secret encryption --------------------------------------------------------------------------------------

func (s *Server) totpAEAD() (cipher.AEAD, error) {
	if len(s.cfg.TokenEncKey) == 0 {
		return nil, errors.New("TOKEN_ENC_KEY is not set")
	}
	mac := hmac.New(sha256.New, s.cfg.TokenEncKey)
	mac.Write([]byte("totp-secret-v1"))
	block, err := aes.NewCipher(mac.Sum(nil))
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// sealTOTP binds the ciphertext to the user, so a secret copied to another row never decrypts.
func (s *Server) sealTOTP(userID string, secret []byte) ([]byte, error) {
	g, err := s.totpAEAD()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return g.Seal(nonce, nonce, secret, []byte(userID)), nil
}

func (s *Server) openTOTP(userID string, blob []byte) ([]byte, error) {
	g, err := s.totpAEAD()
	if err != nil {
		return nil, err
	}
	if len(blob) < g.NonceSize() {
		return nil, errors.New("totp secret unreadable")
	}
	return g.Open(nil, blob[:g.NonceSize()], blob[g.NonceSize():], []byte(userID))
}

// ---- recovery codes -----------------------------------------------------------------------------------------

const recoveryAlphabet = "abcdefghjkmnpqrstuvwxyz23456789" // no look-alikes (i, l, o, 0, 1)

// newRecoveryCode is 10 characters shown as xxxxx-xxxxx (about 49 bits).
func newRecoveryCode() (string, error) {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, 0, 11)
	for i, x := range b {
		if i == 5 {
			out = append(out, '-')
		}
		out = append(out, recoveryAlphabet[int(x)%len(recoveryAlphabet)])
	}
	return string(out), nil
}

// normalizeRecovery lowercases and drops everything that is not part of the alphabet.
func normalizeRecovery(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if strings.ContainsRune(recoveryAlphabet, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func recoveryHash(salt, code string) string {
	sum := sha256.Sum256([]byte(salt + ":" + normalizeRecovery(code)))
	return hex.EncodeToString(sum[:])
}

func newSalt() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}
