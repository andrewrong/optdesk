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

	"github.com/andrewrong/optdesk/internal/pages"
)

var _ pages.NewHandlerFunc = Handler

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

func TestHandlerServesStub(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	Handler([]byte(`{"k":"v"}`)).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content type = %q", ct)
	}
	m := vmRe.FindSubmatch(body)
	if m == nil || string(m[1]) != `{"k":"v"}` {
		t.Fatalf("body = %s", body)
	}
}

func TestHandlerBadViewModel(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	Handler([]byte(`nope`)).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
