package fieldcrypt

import (
	"bytes"
	"crypto/rand"
	"errors"
	"strings"
	"testing"
)

func ring(t *testing.T, master string) *Ring {
	t.Helper()
	r, err := NewRing([]byte(strings.Repeat(master, 32)))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestSealOpenRoundTrip(t *testing.T) {
	r := ring(t, "k")
	for _, plain := range []string{"Refiere dolor de cabeza", "acentos: ñ á ü 🙂", strings.Repeat("x", 20000), "enc:v1:parece-cifrado"} {
		sealed, err := r.Seal("encounters", "notes", "row-1", plain)
		if err != nil {
			t.Fatal(err)
		}
		if !IsSealed(sealed) || strings.Contains(sealed, "dolor") {
			t.Fatalf("not sealed: %.40s", sealed)
		}
		got, err := r.Open("encounters", "notes", "row-1", sealed)
		if err != nil || got != plain {
			t.Fatalf("open: %q %v", got, err)
		}
	}
	// a random nonce: the same text never seals to the same value
	a, _ := r.Seal("t", "c", "1", "igual")
	b, _ := r.Seal("t", "c", "1", "igual")
	if a == b {
		t.Fatal("nonce reused")
	}
}

func TestEmptyAndLegacyPlaintext(t *testing.T) {
	r := ring(t, "k")
	if s, err := r.Seal("t", "c", "1", ""); s != "" || err != nil {
		t.Fatalf("empty stays empty: %q %v", s, err)
	}
	// rows written before encryption existed are read as they are
	if got, err := r.Open("t", "c", "1", "texto viejo en claro"); got != "texto viejo en claro" || err != nil {
		t.Fatalf("legacy: %q %v", got, err)
	}
	if got, err := r.Open("t", "c", "1", ""); got != "" || err != nil {
		t.Fatal("empty open")
	}
	m, err := r.OpenProfile("patients", "profile", "1", []byte(`{"allergies_text":"Penicilina"}`))
	if err != nil || m["allergies_text"] != "Penicilina" {
		t.Fatalf("legacy profile: %v %v", m, err)
	}
}

func TestAADBindsRowAndColumn(t *testing.T) {
	r := ring(t, "k")
	sealed, _ := r.Seal("encounters", "notes", "row-1", "secreto")
	for _, c := range [][3]string{{"encounters", "notes", "row-2"}, {"encounters", "exam", "row-1"}, {"prescriptions", "notes", "row-1"}} {
		_, err := r.Open(c[0], c[1], c[2], sealed)
		var de *DecryptError
		if !errors.As(err, &de) {
			t.Fatalf("%v must not open: %v", c, err)
		}
	}
	// tampering is detected
	raw := []byte(sealed)
	raw[len(raw)-2] ^= 1
	if _, err := r.Open("encounters", "notes", "row-1", string(raw)); err == nil {
		t.Fatal("tampered value opened")
	}
	if _, err := r.Open("encounters", "notes", "row-1", Prefix+"!!!"); err == nil {
		t.Fatal("garbage opened")
	}
}

func TestWrongKeyFailsWithDecryptError(t *testing.T) {
	a, b := ring(t, "a"), ring(t, "b")
	sealed, _ := a.Seal("encounters", "notes", "1", "secreto")
	_, err := b.Open("encounters", "notes", "1", sealed)
	var de *DecryptError
	if !errors.As(err, &de) || de.Table != "encounters" || de.RowID != "1" {
		t.Fatalf("want DecryptError: %v", err)
	}
	if strings.Contains(err.Error(), "secreto") {
		t.Fatal("the error must not carry content")
	}
	var nilRing *Ring
	if _, err := nilRing.Seal("t", "c", "1", "x"); !errors.Is(err, ErrNoKey) {
		t.Fatalf("no key: %v", err)
	}
	if _, err := nilRing.Open("t", "c", "1", sealed); err == nil {
		t.Fatal("nil ring opened")
	}
}

func TestKeyRotation(t *testing.T) {
	old := ring(t, "k")
	sealedOld, _ := old.Seal("t", "c", "1", "viejo")
	if KeyID(sealedOld) != 1 {
		t.Fatalf("key id: %d", KeyID(sealedOld))
	}
	next := ring(t, "k")
	key2 := make([]byte, 32)
	_, _ = rand.Read(key2)
	if err := next.Add(2, key2); err != nil {
		t.Fatal(err)
	}
	if err := next.SetCurrent(2); err != nil {
		t.Fatal(err)
	}
	sealedNew, _ := next.Seal("t", "c", "1", "nuevo")
	if KeyID(sealedNew) != 2 {
		t.Fatalf("key id: %d", KeyID(sealedNew))
	}
	// the ring with both keys opens both; the old ring cannot open the new value
	if got, err := next.Open("t", "c", "1", sealedOld); err != nil || got != "viejo" {
		t.Fatalf("old value: %q %v", got, err)
	}
	if got, err := next.Open("t", "c", "1", sealedNew); err != nil || got != "nuevo" {
		t.Fatalf("new value: %q %v", got, err)
	}
	if _, err := old.Open("t", "c", "1", sealedNew); err == nil {
		t.Fatal("old ring opened a newer key")
	}
	if next.SetCurrent(9) == nil || next.Add(0, key2) == nil || next.Add(3, key2[:5]) == nil {
		t.Fatal("invalid keys accepted")
	}
}

func TestProfileSealing(t *testing.T) {
	r := ring(t, "k")
	profile := map[string]any{"species": "Perro", "allergies_text": "Penicilina", "chronic_conditions": []any{"Diabetes"}}
	raw, err := r.SealProfile("patients", "profile", "p1", profile)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("Penicilina")) || bytes.Contains(raw, []byte("Diabetes")) {
		t.Fatalf("profile in clear: %s", raw)
	}
	if !bytes.Contains(raw, []byte(`"species":"Perro"`)) || !bytes.Contains(raw, []byte(`"_enc":"v1:`)) {
		t.Fatalf("shape: %s", raw)
	}
	if sealed, kid := IsProfileSealed(raw); !sealed || kid != 1 {
		t.Fatalf("IsProfileSealed: %v %d", sealed, kid)
	}
	got, err := r.OpenProfile("patients", "profile", "p1", raw)
	if err != nil || got["allergies_text"] != "Penicilina" || got["species"] != "Perro" {
		t.Fatalf("open: %v %v", got, err)
	}
	if _, err := r.OpenProfile("patients", "profile", "p2", raw); err == nil {
		t.Fatal("profile moved to another patient opened")
	}
	if _, err := ring(t, "z").OpenProfile("patients", "profile", "p1", raw); err == nil {
		t.Fatal("wrong key opened the profile")
	}
	// empty profiles stay {}
	if raw, _ := r.SealProfile("patients", "profile", "p1", map[string]any{}); string(raw) != "{}" {
		t.Fatalf("empty: %s", raw)
	}
}
