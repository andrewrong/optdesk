package settingsform

import (
	"crypto/hmac"
	"encoding/hex"
	"errors"
	"mime"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
)

const (
	maxBody     = 64 << 10
	contentType = "application/x-www-form-urlencoded"
)

// The texts are fixed: none names the key, the expected MAC or a value.
var (
	errRemote  = errors.New("settings form: remote address refused")
	errHost    = errors.New("settings form: host refused")
	errOrigin  = errors.New("settings form: origin refused")
	errType    = errors.New("settings form: content type refused")
	errBody    = errors.New("settings form: body size refused")
	errFormat  = errors.New("settings form: token format refused")
	errToken   = errors.New("settings form: token refused")
	errExpired = errors.New("settings form: token expired")
	errAhead   = errors.New("settings form: token too far ahead")
)

// Check applies the guard in a fixed order: remote address, host, origin,
// content type, body size, token format, MAC, expiry.
func (f form) Check(r settingsapi.FormRequest, now time.Time) error {
	if ip := net.ParseIP(hostOf(r.RemoteAddr)); ip == nil || !ip.IsLoopback() {
		return errRemote
	}
	_, port, err := net.SplitHostPort(r.Listen)
	if err != nil || !loopbackName(r.Host, port) {
		return errHost
	}
	if r.Origin != "" {
		rest, ok := strings.CutPrefix(r.Origin, "http://")
		if !ok || !loopbackName(rest, port) {
			return errOrigin
		}
	}
	if mt, _, err := mime.ParseMediaType(r.ContentType); err != nil || mt != contentType {
		return errType
	}
	if r.BodySize < 0 || r.BodySize > maxBody {
		return errBody
	}
	exp, got, ok := parseToken(r.Token)
	if !ok {
		return errFormat
	}
	if !hmac.Equal(got, f.sign(r.Token.Expires)) {
		return errToken
	}
	if exp < now.Unix() {
		return errExpired
	}
	if exp > now.Add(f.ttl).Unix() {
		return errAhead
	}
	return nil
}

func hostOf(addr string) string {
	h, _, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	return h
}

// loopbackName reports whether hp is one of the three loopback names with the
// given port.
func loopbackName(hp, port string) bool {
	h, p, err := net.SplitHostPort(hp)
	if err != nil || p != port {
		return false
	}
	return h == "127.0.0.1" || h == "localhost" || h == "::1"
}

// parseToken checks the format: digits only for the expiry, 64 lower-case hex
// digits for the MAC. It returns the expiry and the decoded MAC.
func parseToken(t settingsapi.FormToken) (int64, []byte, bool) {
	if t.Expires == "" || len(t.Expires) > 18 || len(t.MAC) != 64 {
		return 0, nil, false
	}
	for _, c := range t.Expires {
		if c < '0' || c > '9' {
			return 0, nil, false
		}
	}
	if t.MAC != strings.ToLower(t.MAC) {
		return 0, nil, false
	}
	b, err := hex.DecodeString(t.MAC)
	if err != nil {
		return 0, nil, false
	}
	exp, err := strconv.ParseInt(t.Expires, 10, 64)
	if err != nil {
		return 0, nil, false
	}
	return exp, b, true
}
