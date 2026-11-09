package settingsform

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
)

func refused(t *testing.T, name string, mut func(*settingsapi.FormRequest)) {
	t.Helper()
	f := newForm()
	r := goodRequest(f.Token(t0))
	mut(&r)
	if err := f.Check(r, t0); err == nil {
		t.Errorf("%s: request accepted", name)
	}
}

func TestCheckRefusesNonLoopbackRemote(t *testing.T) {
	t.Parallel()
	for _, a := range []string{"192.168.1.5:5000", "10.0.0.1:1", "[2001:db8::1]:80", "garbage", ""} {
		refused(t, a, func(r *settingsapi.FormRequest) { r.RemoteAddr = a })
	}
	f := newForm()
	for _, a := range []string{"127.0.0.1:1", "[::1]:9", "127.5.5.5:2"} {
		r := goodRequest(f.Token(t0))
		r.RemoteAddr = a
		if err := f.Check(r, t0); err != nil {
			t.Errorf("%s refused: %v", a, err)
		}
	}
}

func TestCheckRefusesWrongHost(t *testing.T) {
	t.Parallel()
	for _, h := range []string{"evil.example:8080", "127.0.0.1:9999", "127.0.0.1", "", "localhost.evil.com:8080"} {
		refused(t, h, func(r *settingsapi.FormRequest) { r.Host = h })
	}
	f := newForm()
	for _, h := range []string{"localhost:8080", "[::1]:8080", "127.0.0.1:8080"} {
		r := goodRequest(f.Token(t0))
		r.Host = h
		if err := f.Check(r, t0); err != nil {
			t.Errorf("host %s refused: %v", h, err)
		}
	}
}

func TestCheckRefusesForeignOrigin(t *testing.T) {
	t.Parallel()
	for _, o := range []string{"http://evil.example", "https://127.0.0.1:8080", "http://127.0.0.1:9", "null", "http://localhost:8080/x"} {
		refused(t, o, func(r *settingsapi.FormRequest) { r.Origin = o })
	}
	f := newForm()
	for _, o := range []string{"", "http://localhost:8080", "http://[::1]:8080"} {
		r := goodRequest(f.Token(t0))
		r.Origin = o
		if err := f.Check(r, t0); err != nil {
			t.Errorf("origin %q refused: %v", o, err)
		}
	}
}

func TestCheckRefusesWrongContentType(t *testing.T) {
	t.Parallel()
	for _, c := range []string{"application/json", "multipart/form-data; boundary=x", "text/plain", ""} {
		refused(t, c, func(r *settingsapi.FormRequest) { r.ContentType = c })
	}
	f := newForm()
	r := goodRequest(f.Token(t0))
	r.ContentType = "application/x-www-form-urlencoded; charset=UTF-8"
	if err := f.Check(r, t0); err != nil {
		t.Errorf("charset parameter refused: %v", err)
	}
}

func TestCheckRefusesBigBody(t *testing.T) {
	t.Parallel()
	refused(t, "big", func(r *settingsapi.FormRequest) { r.BodySize = 64<<10 + 1 })
	refused(t, "negative", func(r *settingsapi.FormRequest) { r.BodySize = -1 })
	f := newForm()
	r := goodRequest(f.Token(t0))
	r.BodySize = 64 << 10
	if err := f.Check(r, t0); err != nil {
		t.Errorf("64 KiB refused: %v", err)
	}
}
