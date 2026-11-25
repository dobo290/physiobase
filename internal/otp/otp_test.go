package otp

import (
	"strings"
	"testing"
	"time"
)

// rfcSecret ist das Beispielgeheimnis aus RFC 6238 (ASCII "12345678901234567890").
const rfcSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

// TestRFC6238 prüft die Testvektoren aus RFC 6238, Anhang B (SHA1).
func TestRFC6238(t *testing.T) {
	cases := []struct {
		unix int64
		want string // achtstellig, wie im RFC
	}{
		{59, "94287082"},
		{1111111109, "07081804"},
		{1111111111, "14050471"},
		{1234567890, "89005924"},
		{2000000000, "69279037"},
		{20000000000, "65353130"},
	}
	for _, c := range cases {
		step := Step(time.Unix(c.unix, 0))
		got, err := codeFor(rfcSecret, step, 8)
		if err != nil {
			t.Fatalf("%d: %v", c.unix, err)
		}
		if got != c.want {
			t.Errorf("%d: Code = %s, erwartet %s", c.unix, got, c.want)
		}
	}
}

func TestNewSecret(t *testing.T) {
	s, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 32 { // 20 Byte Base32 ohne Padding
		t.Errorf("Länge = %d, erwartet 32", len(s))
	}
	if _, err := SecretBytes(s); err != nil {
		t.Errorf("Geheimnis nicht dekodierbar: %v", err)
	}
	other, _ := NewSecret()
	if s == other {
		t.Error("zwei Geheimnisse sind gleich")
	}
}

func TestNormalize(t *testing.T) {
	if got := Normalize(" gezd gnbv-gy3t qojq gez dgnb vgy3 tqojq "); got != rfcSecret {
		t.Errorf("Normalize = %q", got)
	}
	if NormalizeRecoveryCode(" AB2C-D3EF ") != "ab2cd3ef" {
		t.Error("Wiederherstellungscode nicht normalisiert")
	}
}

func TestVerifyWindow(t *testing.T) {
	now := time.Unix(1111111111, 0) // Zeitschritt 37037037
	prev := time.Unix(1111111111-30, 0)
	next := time.Unix(1111111111+30, 0)
	far := time.Unix(1111111111+120, 0)

	for _, tm := range []time.Time{now, prev, next} {
		code, err := CodeAt(rfcSecret, tm)
		if err != nil {
			t.Fatal(err)
		}
		if !Verify(rfcSecret, code, now) {
			t.Errorf("Code von %s wurde abgelehnt", tm)
		}
	}

	code, _ := CodeAt(rfcSecret, far)
	if Verify(rfcSecret, code, now) {
		t.Error("Code außerhalb des Fensters wurde akzeptiert")
	}
	if Verify(rfcSecret, "000000", now) && code != "000000" {
		t.Error("falscher Code wurde akzeptiert")
	}
	for _, bad := range []string{"", "12345", "1234567", "abcdef", "12 345"} {
		if Verify(rfcSecret, bad, now) {
			t.Errorf("ungültiger Code %q wurde akzeptiert", bad)
		}
	}
	if Verify("kein-base32!", "123456", now) {
		t.Error("kaputtes Geheimnis wurde akzeptiert")
	}
	// Das Geheimnis darf in Vierergruppen eingetippt werden.
	code, _ = CodeAt(rfcSecret, now)
	grouped := code[:3] + " " + code[3:]
	if !Verify("GEZD GNBV GY3T QOJQ GEZD GNBV GY3T QOJQ", grouped, now) {
		t.Error("gruppiertes Geheimnis wurde abgelehnt")
	}
}

func TestURL(t *testing.T) {
	u := URL("Praxis Müller", "dobo", "GEZD GNBV GY3T QOJQ GEZD GNBV GY3T QOJQ")
	if !strings.HasPrefix(u, "otpauth://totp/") {
		t.Fatalf("Schema fehlt: %s", u)
	}
	for _, want := range []string{"secret=" + rfcSecret, "issuer=Praxis", "digits=6", "period=30", "algorithm=SHA1"} {
		if !strings.Contains(u, want) {
			t.Errorf("%q fehlt in %s", want, u)
		}
	}
}

func TestNewRecoveryCode(t *testing.T) {
	seen := map[string]bool{}
	for range 20 {
		c, err := NewRecoveryCode()
		if err != nil {
			t.Fatal(err)
		}
		if len(c) != 9 || c[4] != '-' {
			t.Fatalf("Format = %q", c)
		}
		if seen[c] {
			t.Fatal("Wiederherstellungscode doppelt")
		}
		seen[c] = true
		for _, r := range strings.ReplaceAll(c, "-", "") {
			if !strings.ContainsRune(alphabet, r) {
				t.Errorf("unerwartetes Zeichen %q", r)
			}
		}
	}
}
