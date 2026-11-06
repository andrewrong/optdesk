package setup

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

const rfcSalt = "W22ZaJ0SNY7soEsUEjb6gQ=="

func hmac256(key []byte, msg string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(msg))
	return h.Sum(nil)
}

// TestScramVerifierVector derives the keys step by step by RFC 5802 for the RFC 7677 example.
func TestScramVerifierVector(t *testing.T) {
	t.Parallel()
	salt, err := base64.StdEncoding.DecodeString(rfcSalt)
	if err != nil {
		t.Fatal(err)
	}
	salted, err := pbkdf2.Key(sha256.New, "pencil", salt, 4096, 32)
	if err != nil {
		t.Fatal(err)
	}
	clientKey := hmac256(salted, "Client Key")
	stored := sha256.Sum256(clientKey)
	serverKey := hmac256(salted, "Server Key")
	want := "SCRAM-SHA-256$4096:" + rfcSalt + "$" +
		base64.StdEncoding.EncodeToString(stored[:]) + ":" + base64.StdEncoding.EncodeToString(serverKey)
	if got := scramVerifier("pencil", salt); got != want {
		t.Errorf("verifier = %q, want %q", got, want)
	}
}

func TestScramVerifierShape(t *testing.T) {
	t.Parallel()
	salt := []byte("0123456789abcdef")
	const pw = "s3cretpassw0rd"
	v := scramVerifier(pw, salt)
	rest, ok := strings.CutPrefix(v, "SCRAM-SHA-256$4096:")
	if !ok {
		t.Fatalf("prefix: %q", v)
	}
	saltPart, keys, ok := strings.Cut(rest, "$")
	if !ok {
		t.Fatalf("no $: %q", v)
	}
	sk, srv, ok := strings.Cut(keys, ":")
	if !ok {
		t.Fatalf("no ':': %q", v)
	}
	for _, p := range []string{saltPart, sk, srv} {
		if _, err := base64.StdEncoding.DecodeString(p); err != nil {
			t.Errorf("part %q is not base64: %v", p, err)
		}
	}
	if got, _ := base64.StdEncoding.DecodeString(saltPart); string(got) != string(salt) {
		t.Errorf("salt part decodes to %q", got)
	}
	if strings.Contains(v, pw) {
		t.Error("the password is in the verifier")
	}
}

func TestNewSalt(t *testing.T) {
	t.Parallel()
	a, b := newSalt(), newSalt()
	if len(a) != 16 || len(b) != 16 {
		t.Fatalf("lengths %d, %d", len(a), len(b))
	}
	if string(a) == string(b) {
		t.Error("two salts are equal")
	}
}
