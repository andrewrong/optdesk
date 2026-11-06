package setup

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// scramIterations is the PBKDF2 iteration count of the verifier, the PostgreSQL default.
const scramIterations = 4096

// scramVerifier returns the PostgreSQL SCRAM-SHA-256 verifier string of password for salt:
// SCRAM-SHA-256$4096:<salt>$<StoredKey>:<ServerKey>, with the RFC 5802 keys in base64.
// A server stores it as it is, so the plain password never travels in a statement.
//
// SASLprep (RFC 4013) is skipped. That is safe because Setup generates the passwords itself, as hex, which is
// ASCII, and SASLprep leaves printable ASCII unchanged. Do not call this with a password of another origin.
func scramVerifier(password string, salt []byte) string {
	salted, err := pbkdf2.Key(sha256.New, password, salt, scramIterations, sha256.Size)
	if err != nil { // only for a key length or a hash the standard library refuses; neither is possible here
		panic("setup: pbkdf2: " + err.Error())
	}
	mac := func(msg string) []byte {
		h := hmac.New(sha256.New, salted)
		h.Write([]byte(msg))
		return h.Sum(nil)
	}
	stored := sha256.Sum256(mac("Client Key"))
	b64 := base64.StdEncoding.EncodeToString
	return "SCRAM-SHA-256$4096:" + b64(salt) + "$" + b64(stored[:]) + ":" + b64(mac("Server Key"))
}

// newSalt returns 16 random bytes from crypto/rand.
func newSalt() []byte {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil { // crypto/rand.Read does not fail in Go 1.24 and later
		panic("setup: salt: " + err.Error())
	}
	return b
}
