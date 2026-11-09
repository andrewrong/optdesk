package embed

import (
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
)

func TestAssets(t *testing.T) {
	t.Parallel()
	h := newAssets(fstest.MapFS{
		"assets/a.js": {Data: []byte("console.log(1)")},
		"index.html":  {Data: []byte("<html>")},
		"x":           {Data: []byte("secret")},
	})
	t.Run("file", func(t *testing.T) {
		t.Parallel()
		rec := get(t, h, http.MethodGet, "/assets/a.js")
		if rec.Code != http.StatusOK || rec.Body.String() != "console.log(1)" {
			t.Fatalf("status = %d body = %q", rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
			t.Fatalf("content type = %q", ct)
		}
	})
	for _, p := range []string{"/assets/missing", "/index.html", "/assets/../x", "/assets/"} {
		t.Run("404 "+p, func(t *testing.T) {
			t.Parallel()
			if rec := get(t, h, http.MethodGet, p); rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", rec.Code)
			}
		})
	}
	t.Run("POST", func(t *testing.T) {
		t.Parallel()
		if rec := get(t, h, http.MethodPost, "/assets/a.js"); rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want 405", rec.Code)
		}
	})
}
