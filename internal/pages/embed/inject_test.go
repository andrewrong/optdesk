package embed

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/andrewrong/optdesk/internal/pages"
)

var _ pages.NewHandlerFunc = Handler

var indexHTML = indexFS()["index.html"].Data

var vmRe = regexp.MustCompile(`(?s)<script id="vm" type="application/json">(.*?)</script>`)

func TestInjectEscapes(t *testing.T) {
	t.Parallel()
	in := `{"note":"a </script><b>x</b> & end","n":[1,null]}`
	out, err := Render(indexHTML, []byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(out), "</script>"); got != 1 {
		t.Fatalf("closing script tags = %d, want 1 (the element's own)", got)
	}
	m := vmRe.FindSubmatch(out)
	if m == nil {
		t.Fatal("no vm element")
	}
	var got, want any
	if err := json.Unmarshal(m[1], &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(in), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("vm = %v, want %v", got, want)
	}
}

func TestRenderRejectsBadJSON(t *testing.T) {
	t.Parallel()
	if _, err := Render(indexHTML, []byte(`{"a":`)); err == nil {
		t.Fatal("want error for invalid JSON")
	}
}

func indexFS() fstest.MapFS {
	return fstest.MapFS{"index.html": {Data: []byte(`<html><script id="vm" type="application/json"></script></html>`)}}
}

func get(t *testing.T, h http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), method, target, nil))
	return rec
}

func TestHandlerServesIndex(t *testing.T) {
	t.Parallel()
	rec := get(t, newHandler(indexFS(), []byte(`{"k":"v"}`)), http.MethodGet, "/")
	body, _ := io.ReadAll(rec.Result().Body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content type = %q", ct)
	}
	m := vmRe.FindSubmatch(body)
	if m == nil || string(m[1]) != `{"k":"v"}` {
		t.Fatalf("body = %s", body)
	}
}

func TestHandlerBadViewModel(t *testing.T) {
	t.Parallel()
	if rec := get(t, newHandler(indexFS(), []byte(`nope`)), http.MethodGet, "/"); rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestHandlerNoBundle500(t *testing.T) {
	t.Parallel()
	rec := get(t, newHandler(fstest.MapFS{}, []byte(`{}`)), http.MethodGet, "/")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "web bundle not built: run pnpm --dir web build" {
		t.Fatalf("body = %q", got)
	}
}
