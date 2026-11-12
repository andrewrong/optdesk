package settingsform

import (
	"crypto/hmac"
	"mime"
	"net"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
)

const uploadContentType = "multipart/form-data"

// CheckUpload applies the guard of Check to a multipart upload: remote
// address, host, origin, content type multipart/form-data, body size from 0 to
// maxBody, token format, MAC, expiry, in that order. The helpers and the
// error texts are those of Check.
func (f form) CheckUpload(r settingsapi.FormRequest, maxBody int64, now time.Time) error {
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
	if mt, _, err := mime.ParseMediaType(r.ContentType); err != nil || mt != uploadContentType {
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
