package api

import (
	"strings"
	"testing"
	"time"
)

// RFC 6238 appendix B (SHA-1, secret "12345678901234567890"): the 8-digit values end with the 6-digit ones.
func TestTOTPRFC6238Vectors(t *testing.T) {
	secret := []byte("12345678901234567890")
	for _, v := range []struct {
		unix int64
		want string
	}{{59, "287082"}, {1111111109, "081804"}, {1111111111, "050471"}, {1234567890, "005924"}, {2000000000, "279037"}, {20000000000, "353130"}} {
		if got := totpCode(secret, totpStep(time.Unix(v.unix, 0))); got != v.want {
			t.Errorf("t=%d: got %s want %s", v.unix, got, v.want)
		}
	}
}

func TestTOTPWindowAndReuse(t *testing.T) {
	secret := []byte("12345678901234567890")
	now := time.Unix(1111111109, 0)
	cur := totpStep(now)
	for _, d := range []int64{-1, 0, 1} {
		if _, ok := totpMatch(secret, totpCode(secret, cur+d), now, 0); !ok {
			t.Errorf("step %+d should be accepted", d)
		}
	}
	for _, d := range []int64{-2, 2, 10} {
		if _, ok := totpMatch(secret, totpCode(secret, cur+d), now, 0); ok {
			t.Errorf("step %+d should be refused", d)
		}
	}
	step, ok := totpMatch(secret, totpCode(secret, cur), now, 0)
	if !ok || step != cur {
		t.Fatal("current code must match")
	}
	if _, ok := totpMatch(secret, totpCode(secret, cur), now, step); ok {
		t.Error("the same step must not be accepted twice")
	}
	if _, ok := totpMatch(secret, totpCode(secret, cur-1), now, step); ok {
		t.Error("an older step must not be accepted after a newer one")
	}
	if _, ok := totpMatch(secret, "12345", now, 0); ok {
		t.Error("wrong length")
	}
}

func TestRecoveryCodeShape(t *testing.T) {
	c, err := newRecoveryCode()
	if err != nil || len(c) != 11 || c[5] != '-' {
		t.Fatalf("code %q err %v", c, err)
	}
	if normalizeRecovery(strings.ToUpper(c)) != normalizeRecovery(c) || len(normalizeRecovery(c)) != 10 {
		t.Fatal("normalization")
	}
	if recoveryHash("a", c) == recoveryHash("b", c) {
		t.Fatal("salt must change the hash")
	}
}

func TestTwoFactorRequiredPolicy(t *testing.T) {
	cases := []struct {
		policy, role string
		want         bool
	}{
		{"none", RoleAdmin, false}, {"admins", RoleAdmin, true}, {"admins", RoleDoctor, false},
		{"clinical", RoleDoctor, true}, {"clinical", RoleReception, false}, {"all", RoleCashier, true}, {"all", RolePlatformAdmin, false},
	}
	for _, c := range cases {
		if got := twoFactorRequired(c.policy, c.role); got != c.want {
			t.Errorf("%s/%s: got %v", c.policy, c.role, got)
		}
	}
}
