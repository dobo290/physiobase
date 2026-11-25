// Package otp erzeugt und prüft TOTP-Codes (RFC 6238) für die optionale
// Zwei-Faktor-Anmeldung: zeitbasierte Einmalkennwörter, wie sie Google
// Authenticator, Aegis, 1Password oder Vaultwarden erzeugen.
package otp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // G505: TOTP (RFC 6238) schreibt HMAC-SHA1 als Standardalgorithmus vor
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	// Period ist die Gültigkeitsdauer eines Codes in Sekunden.
	Period = 30
	// Digits ist die Stellenzahl der Codes.
	Digits = 6
	// Skew ist die erlaubte Abweichung in Zeitschritten (± ein Schritt = 30 s),
	// damit leicht abweichende Gerätezeit nicht zur Sperre führt.
	Skew = 1
	// secretLen ist die Länge neuer Geheimnisse (160 Bit, wie RFC 4226 empfiehlt).
	secretLen = 20
)

var enc = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewSecret erzeugt ein neues Base32-Geheimnis ohne Füllzeichen.
func NewSecret() (string, error) {
	b := make([]byte, secretLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return enc.EncodeToString(b), nil
}

// Normalize macht ein eingetipptes Geheimnis vergleichbar: ohne Leerzeichen,
// Bindestriche und Kleinschreibung. Die Authenticator-Apps zeigen das Geheimnis
// in Vierergruppen an.
func Normalize(secret string) string {
	r := strings.NewReplacer(" ", "", "-", "", "\t", "")
	return strings.ToUpper(r.Replace(secret))
}

// SecretBytes dekodiert das Base32-Geheimnis.
func SecretBytes(secret string) ([]byte, error) {
	return enc.DecodeString(Normalize(secret))
}

// Step liefert den Zeitschritt zu einem Zeitpunkt.
func Step(t time.Time) int64 { return t.Unix() / Period }

// codeFor berechnet den Code für einen Zeitschritt mit beliebiger Stellenzahl.
func codeFor(secret string, step int64, digits int) (string, error) {
	key, err := SecretBytes(secret)
	if err != nil {
		return "", err
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step)) //nolint:gosec // G115: Zeitschritte sind positiv
	m := hmac.New(sha1.New, key)
	m.Write(msg[:])
	sum := m.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	n := binary.BigEndian.Uint32(sum[off:]) & 0x7fffffff
	mod := uint32(1)
	for range digits {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", digits, n%mod), nil
}

// Code liefert den sechsstelligen Code für einen Zeitschritt.
func Code(secret string, step int64) (string, error) {
	return codeFor(secret, step, Digits)
}

// CodeAt liefert den Code, der zu diesem Zeitpunkt gilt.
func CodeAt(secret string, t time.Time) (string, error) {
	return Code(secret, Step(t))
}

// Verify prüft einen Code gegen das Zeitfenster um now. Leerzeichen und
// Bindestriche im Code werden ignoriert ("123 456" ist gültig).
func Verify(secret, code string, now time.Time) bool {
	code = strings.NewReplacer(" ", "", "-", "", "\t", "").Replace(strings.TrimSpace(code))
	if len(code) != Digits {
		return false
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			return false
		}
	}
	step := Step(now)
	for d := -Skew; d <= Skew; d++ {
		want, err := Code(secret, step+int64(d))
		if err != nil {
			return false
		}
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

// URL baut den otpauth-Link, den Authenticator-Apps aus dem QR-Code lesen.
// issuer ist der Name der Praxis, account der Benutzername.
func URL(issuer, account, secret string) string {
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{
		"secret":    {Normalize(secret)},
		"issuer":    {issuer},
		"algorithm": {"SHA1"},
		"digits":    {fmt.Sprint(Digits)},
		"period":    {fmt.Sprint(Period)},
	}
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// alphabet für Wiederherstellungscodes: ohne 0/O und 1/I/L, damit nichts
// verwechselt wird.
const alphabet = "23456789abcdefghjkmnpqrstuvwxyz"

// NewRecoveryCode erzeugt einen Wiederherstellungscode im Format xxxx-xxxx.
func NewRecoveryCode() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b[:4]) + "-" + string(b[4:]), nil
}

// NormalizeRecoveryCode entfernt Bindestriche und Kleinschreibung, damit ein
// abgetippter Code vergleichbar ist.
func NormalizeRecoveryCode(code string) string {
	r := strings.NewReplacer("-", "", " ", "", "\t", "")
	return strings.ToLower(r.Replace(code))
}
