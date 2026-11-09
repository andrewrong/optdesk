package settingsform

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
)

var testKey = []byte("0123456789abcdef0123456789abcdef")

var _ settingsapi.NewSettingsFormFunc = New

var t0 = time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)

// fakeParams is a registry of two keys: an int and a decimal, trimmed text.
type fakeParams struct{}

func (fakeParams) Specs() []settingsapi.RuleParamSpec {
	return []settingsapi.RuleParamSpec{{Key: "min_dte"}, {Key: "max_spread"}}
}

func (fakeParams) Normalize(key, text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}
	if _, err := strconv.ParseFloat(text, 64); err != nil {
		return "", errBad
	}
	return strings.TrimRight(strings.TrimRight(text, "0"), "."), nil
}

func (fakeParams) Resolve(s map[string]string) map[string]string { return s }

type badErr struct{}

func (badErr) Error() string { return "bad value" }

var errBad = badErr{}

func newForm() settingsapi.SettingsForm { return New(testKey, 15*time.Minute, fakeParams{}) }

func goodRequest(tok settingsapi.FormToken) settingsapi.FormRequest {
	return settingsapi.FormRequest{
		RemoteAddr:  "127.0.0.1:50000",
		Host:        "127.0.0.1:8080",
		Origin:      "http://127.0.0.1:8080",
		Listen:      "127.0.0.1:8080",
		ContentType: "application/x-www-form-urlencoded",
		BodySize:    100,
		Token:       tok,
	}
}

func TestTokenRoundTrip(t *testing.T) {
	t.Parallel()
	f := newForm()
	tok := f.Token(t0)
	if len(tok.MAC) != 64 || tok.MAC != strings.ToLower(tok.MAC) {
		t.Fatalf("mac %q is not 64 lower-case hex", tok.MAC)
	}
	if err := f.Check(goodRequest(tok), t0); err != nil {
		t.Fatalf("fresh token refused: %v", err)
	}
	if err := f.Check(goodRequest(tok), t0.Add(15*time.Minute)); err != nil {
		t.Fatalf("token at its expiry refused: %v", err)
	}
	if f.Token(t0) != tok {
		t.Fatal("same instant gave another token")
	}
	other := New(bytes.Repeat([]byte{7}, 32), time.Minute, fakeParams{}).Token(t0)
	if other.MAC == tok.MAC {
		t.Fatal("another key gave the same mac")
	}
}

func TestTokenExpired(t *testing.T) {
	t.Parallel()
	f := newForm()
	tok := f.Token(t0)
	if err := f.Check(goodRequest(tok), t0.Add(15*time.Minute+time.Second)); err == nil {
		t.Fatal("expired token accepted")
	}
}

func TestTokenTooFarAhead(t *testing.T) {
	t.Parallel()
	long := New(testKey, time.Hour, fakeParams{}).Token(t0)
	if err := newForm().Check(goodRequest(long), t0); err == nil {
		t.Fatal("token more than the lifetime ahead accepted")
	}
}

func TestTokenBadMAC(t *testing.T) {
	t.Parallel()
	f := newForm()
	tok := f.Token(t0)
	flipped := []byte(tok.MAC)
	if flipped[0] == '0' {
		flipped[0] = '1'
	} else {
		flipped[0] = '0'
	}
	for name, bad := range map[string]settingsapi.FormToken{
		"flipped":     {Expires: tok.Expires, MAC: string(flipped)},
		"other time":  {Expires: strconv.FormatInt(t0.Add(time.Minute).Unix(), 10), MAC: tok.MAC},
		"upper case":  {Expires: tok.Expires, MAC: strings.ToUpper(tok.MAC)},
		"short":       {Expires: tok.Expires, MAC: tok.MAC[:62]},
		"not hex":     {Expires: tok.Expires, MAC: strings.Repeat("z", 64)},
		"empty":       {},
		"signed time": {Expires: "+" + tok.Expires, MAC: tok.MAC},
	} {
		if err := f.Check(goodRequest(bad), t0); err == nil {
			t.Errorf("%s: bad token accepted", name)
		}
	}
}

func TestErrorTextNamesNoSecret(t *testing.T) {
	t.Parallel()
	f := newForm()
	tok := f.Token(t0)
	bad := tok
	bad.MAC = strings.Repeat("a", 64)
	for _, err := range []error{
		f.Check(goodRequest(bad), t0),
		f.Check(goodRequest(tok), t0.Add(time.Hour)),
		f.Check(goodRequest(settingsapi.FormToken{Expires: "x", MAC: "y"}), t0),
	} {
		if err == nil {
			t.Fatal("refusal expected")
		}
		text := err.Error()
		for _, secret := range []string{string(testKey), tok.MAC, bad.MAC} {
			if strings.Contains(text, secret) {
				t.Errorf("error %q names a secret", text)
			}
		}
	}
	again := newForm()
	if f.Check(goodRequest(bad), t0).Error() != again.Check(goodRequest(bad), t0).Error() {
		t.Fatal("error text is not fixed")
	}
}
