package settingsform

import (
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
)

var _ settingsapi.UploadChecker = form{}

const uploadCap = 1 << 20

func uploadChecker(t *testing.T) (settingsapi.UploadChecker, settingsapi.SettingsForm) {
	t.Helper()
	f := newForm()
	u, ok := f.(settingsapi.UploadChecker)
	if !ok {
		t.Fatal("form does not implement UploadChecker")
	}
	return u, f
}

func goodUpload(tok settingsapi.FormToken) settingsapi.FormRequest {
	r := goodRequest(tok)
	r.ContentType = "multipart/form-data; boundary=xyz"
	r.BodySize = 5000
	return r
}

func TestCheckUploadAcceptsValidUpload(t *testing.T) {
	t.Parallel()
	u, f := uploadChecker(t)
	r := goodUpload(f.Token(t0))
	if err := u.CheckUpload(r, uploadCap, t0); err != nil {
		t.Fatalf("valid upload refused: %v", err)
	}
	r.BodySize = uploadCap // exactly the cap is allowed
	if err := u.CheckUpload(r, uploadCap, t0); err != nil {
		t.Fatalf("body at the cap refused: %v", err)
	}
	r.BodySize = 0
	if err := u.CheckUpload(r, uploadCap, t0); err != nil {
		t.Fatalf("empty body refused: %v", err)
	}
}

func TestCheckUploadRefusals(t *testing.T) {
	t.Parallel()
	u, f := uploadChecker(t)
	tok := f.Token(t0)
	old := f.Token(t0.Add(-time.Hour))
	ahead := f.Token(t0.Add(time.Hour))
	bad := tok
	bad.MAC = tok.MAC[:63] + "0"
	if bad.MAC == tok.MAC {
		bad.MAC = tok.MAC[:63] + "1"
	}
	cases := []struct {
		name string
		mut  func(*settingsapi.FormRequest)
	}{
		{"remote address", func(r *settingsapi.FormRequest) { r.RemoteAddr = "192.168.1.5:5000" }},
		{"host", func(r *settingsapi.FormRequest) { r.Host = "evil.example:8080" }},
		{"origin", func(r *settingsapi.FormRequest) { r.Origin = "http://evil.example" }},
		{"urlencoded type", func(r *settingsapi.FormRequest) { r.ContentType = "application/x-www-form-urlencoded" }},
		{"empty type", func(r *settingsapi.FormRequest) { r.ContentType = "" }},
		{"size over cap", func(r *settingsapi.FormRequest) { r.BodySize = uploadCap + 1 }},
		{"negative size", func(r *settingsapi.FormRequest) { r.BodySize = -1 }},
		{"bad token", func(r *settingsapi.FormRequest) { r.Token = bad }},
		{"malformed token", func(r *settingsapi.FormRequest) { r.Token = settingsapi.FormToken{} }},
		{"expired token", func(r *settingsapi.FormRequest) { r.Token = old }},
		{"token ahead", func(r *settingsapi.FormRequest) { r.Token = ahead }},
	}
	for _, c := range cases {
		r := goodUpload(tok)
		c.mut(&r)
		if err := u.CheckUpload(r, uploadCap, t0); err == nil {
			t.Errorf("%s: upload accepted", c.name)
		}
	}
}

func TestCheckUploadUsesCallerCap(t *testing.T) {
	t.Parallel()
	u, f := uploadChecker(t)
	r := goodUpload(f.Token(t0))
	r.BodySize = 200 << 10 // over the 64 KiB of Check, under this cap
	if err := u.CheckUpload(r, 1<<20, t0); err != nil {
		t.Fatalf("under cap refused: %v", err)
	}
	if err := u.CheckUpload(r, 100<<10, t0); err == nil {
		t.Fatal("over a smaller cap accepted")
	}
}
