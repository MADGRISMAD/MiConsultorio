// Package fieldcrypt encrypts free-text clinical columns at rest (AES-256-GCM).
//
// A sealed text value looks like "enc:v1:<base64url>" and a sealed jsonb profile like {"_enc":"v1:<base64url>"}
// (plus the keys listed in ProfileClearKeys, kept readable because SQL reports group by them). The binary payload
// is keyID(1 byte) || nonce(12) || ciphertext+tag. The table, column and row id are bound as additional
// authenticated data, so a value copied to another row or column fails to open. Values without the prefix are
// legacy plaintext and are returned as they are, which makes the migration lazy and reading fully compatible.
package fieldcrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	// Prefix marks a sealed text value.
	Prefix = "enc:v1:"
	// ProfileKey holds the sealed profile inside the jsonb column.
	ProfileKey = "_enc"
	profileTag = "v1:"
	// KeyLabel is the HMAC label used to derive the first field key from the master key.
	KeyLabel = "clinical-fields-v1"
)

// ProfileClearKeys stay readable next to the sealed profile: reports and lists group patients by species.
var ProfileClearKeys = []string{"species"}

// ErrNoKey is returned when sealing without a key ring.
var ErrNoKey = errors.New("fieldcrypt: no encryption key")

// DecryptError reports a value that could not be opened (wrong or rotated key, tampering or a moved value).
type DecryptError struct {
	Table, Column, RowID string
	Err                  error
}

func (e *DecryptError) Error() string {
	return fmt.Sprintf("fieldcrypt: cannot decrypt %s.%s of row %s: %v", e.Table, e.Column, e.RowID, e.Err)
}

func (e *DecryptError) Unwrap() error { return e.Err }

// Ring is the set of keys that can open values; the current one seals. Key ids fit in one byte so new
// versions can be appended without changing the format.
type Ring struct {
	current byte
	keys    map[byte]cipher.AEAD
}

// NewRing derives key id 1 from the master key (the one that already protects attachments and provider tokens).
func NewRing(master []byte) (*Ring, error) {
	if len(master) == 0 {
		return nil, ErrNoKey
	}
	r := &Ring{keys: map[byte]cipher.AEAD{}}
	if err := r.Add(1, deriveKey(master, KeyLabel)); err != nil {
		return nil, err
	}
	r.current = 1
	return r, nil
}

func deriveKey(master []byte, label string) []byte {
	mac := hmac.New(sha256.New, master)
	mac.Write([]byte(label))
	return mac.Sum(nil)
}

// Add registers a 32-byte key under an id (1..255); SetCurrent makes it the sealing key.
func (r *Ring) Add(id byte, key []byte) error {
	if id == 0 || len(key) != 32 {
		return errors.New("fieldcrypt: invalid key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	r.keys[id] = g
	return nil
}

// SetCurrent picks the key that seals new values.
func (r *Ring) SetCurrent(id byte) error {
	if _, ok := r.keys[id]; !ok {
		return errors.New("fieldcrypt: unknown key id")
	}
	r.current = id
	return nil
}

// Current is the id of the key that seals new values.
func (r *Ring) Current() byte { return r.current }

func aad(table, column, rowID string) []byte { return []byte(table + "." + column + ":" + rowID) }

func (r *Ring) seal(table, column, rowID string, plain []byte) (string, error) {
	if r == nil || len(r.keys) == 0 {
		return "", ErrNoKey
	}
	g := r.keys[r.current]
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := append([]byte{r.current}, nonce...)
	out = g.Seal(out, nonce, plain, aad(table, column, rowID))
	return base64.RawURLEncoding.EncodeToString(out), nil
}

func (r *Ring) open(table, column, rowID, b64 string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(b64)
	if err != nil || len(raw) < 1 {
		return nil, errors.New("malformed value")
	}
	if r == nil {
		return nil, ErrNoKey
	}
	g, ok := r.keys[raw[0]]
	if !ok {
		return nil, errors.New("unknown key id")
	}
	ns := g.NonceSize()
	if len(raw) < 1+ns+g.Overhead() {
		return nil, errors.New("malformed value")
	}
	return g.Open(nil, raw[1:1+ns], raw[1+ns:], aad(table, column, rowID))
}

// IsSealed reports whether a stored text value carries the sealed prefix.
func IsSealed(v string) bool { return strings.HasPrefix(v, Prefix) }

// KeyID returns the key id of a sealed text value (0 when it is not sealed or malformed).
func KeyID(v string) byte {
	if !IsSealed(v) {
		return 0
	}
	raw, err := base64.RawURLEncoding.DecodeString(v[len(Prefix):])
	if err != nil || len(raw) == 0 {
		return 0
	}
	return raw[0]
}

// Seal encrypts a text value for a row. Empty text stays empty (nothing to protect, and the empty-string default stays valid).
func (r *Ring) Seal(table, column, rowID, plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	s, err := r.seal(table, column, rowID, []byte(plain))
	if err != nil {
		return "", err
	}
	return Prefix + s, nil
}

// Open returns the plain text of a stored value: legacy plaintext as it is, sealed values decrypted.
func (r *Ring) Open(table, column, rowID, stored string) (string, error) {
	if !IsSealed(stored) {
		return stored, nil
	}
	b, err := r.open(table, column, rowID, stored[len(Prefix):])
	if err != nil {
		return "", &DecryptError{Table: table, Column: column, RowID: rowID, Err: err}
	}
	return string(b), nil
}

// SealProfile turns a profile map into the jsonb to store. Empty profiles are stored as {}.
func (r *Ring) SealProfile(table, column, rowID string, profile map[string]any) ([]byte, error) {
	if len(profile) == 0 {
		return []byte("{}"), nil
	}
	plain, err := json.Marshal(profile)
	if err != nil {
		return nil, err
	}
	s, err := r.seal(table, column, rowID, plain)
	if err != nil {
		return nil, err
	}
	out := map[string]any{ProfileKey: profileTag + s}
	for _, k := range ProfileClearKeys {
		if v, ok := profile[k].(string); ok && v != "" {
			out[k] = v
		}
	}
	return json.Marshal(out)
}

// IsProfileSealed reports whether stored jsonb is a sealed profile, and with which key id.
func IsProfileSealed(raw []byte) (bool, byte) {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return false, 0
	}
	var enc string
	if v, ok := m[ProfileKey]; !ok || json.Unmarshal(v, &enc) != nil || !strings.HasPrefix(enc, profileTag) {
		return false, 0
	}
	b, err := base64.RawURLEncoding.DecodeString(enc[len(profileTag):])
	if err != nil || len(b) == 0 {
		return true, 0
	}
	return true, b[0]
}

// OpenProfile returns the profile map of stored jsonb (legacy plain profiles as they are).
func (r *Ring) OpenProfile(table, column, rowID string, raw []byte) (map[string]any, error) {
	out := map[string]any{}
	if len(raw) == 0 {
		return out, nil
	}
	if sealed, _ := IsProfileSealed(raw); !sealed {
		_ = json.Unmarshal(raw, &out)
		return out, nil
	}
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw, &m)
	var enc string
	_ = json.Unmarshal(m[ProfileKey], &enc)
	b, err := r.open(table, column, rowID, enc[len(profileTag):])
	if err != nil {
		return nil, &DecryptError{Table: table, Column: column, RowID: rowID, Err: err}
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, &DecryptError{Table: table, Column: column, RowID: rowID, Err: err}
	}
	return out, nil
}
