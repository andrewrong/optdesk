package settingsform

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
)

// postPath is the constant post_path of the settings page schema; the MAC
// binds a token to it.
const postPath = "/owner-settings"

type form struct {
	key []byte
	ttl time.Duration
	p   settingsapi.RuleParams
}

// New builds a settings form for a MAC key, a token lifetime and a registry.
func New(key []byte, ttl time.Duration, p settingsapi.RuleParams) settingsapi.SettingsForm {
	return form{key: append([]byte(nil), key...), ttl: ttl, p: p}
}

// sign is the HMAC-SHA256 of the expiry text and the post path.
func (f form) sign(expires string) []byte {
	h := hmac.New(sha256.New, f.key)
	h.Write([]byte(expires))
	h.Write([]byte{'\n'})
	h.Write([]byte(postPath))
	return h.Sum(nil)
}

// Token returns the token valid at now: the expiry in Unix seconds and the
// lower-case hex MAC.
func (f form) Token(now time.Time) settingsapi.FormToken {
	exp := strconv.FormatInt(now.Add(f.ttl).Unix(), 10)
	return settingsapi.FormToken{Expires: exp, MAC: hex.EncodeToString(f.sign(exp))}
}
