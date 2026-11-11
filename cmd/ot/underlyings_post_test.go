package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
)

// invalidErr is an underlying.InvalidError.
type invalidErr struct{ field, why string }

func (e invalidErr) Error() string { return "invalid " + e.field }
func (e invalidErr) Field() string { return e.field }
func (e invalidErr) Why() string   { return e.why }

func underlyingsPost(t *testing.T, form settingsapi.SettingsForm, fields url.Values) *http.Request {
	t.Helper()
	r := postRequest(t, form, fields)
	r.URL.Path = "/owner-underlyings"
	return r
}

func TestPostUnderlyingsWithoutOwnerEditIs404(t *testing.T) {
	t.Parallel()
	h, st, form, _ := underlyingsSite(t, false)
	rec := serve(h, underlyingsPost(t, form, url.Values{"key": {"US:BBB"}, "status": {"paused"}, "confirm": {"1"}}))
	if rec.Code != http.StatusNotFound || st.calls() != 0 {
		t.Fatalf("%d, %d applies", rec.Code, st.calls())
	}
}

func TestPostUnderlyingsFirstPostWritesNothingAndConfirms(t *testing.T) {
	t.Parallel()
	h, st, form, _ := underlyingsSite(t, true)
	// BBB is not held: pausing it stops bars, dividends and options.
	rec := serve(h, underlyingsPost(t, form, url.Values{"key": {"US:BBB"}, "status": {"paused"}, "options": {"on"}}))
	if rec.Code != 200 || st.calls() != 0 {
		t.Fatalf("%d, %d applies, %.300q", rec.Code, st.calls(), rec.Body)
	}
	vm := decodeUnderlyings(t, rec)
	if vm.Confirm == nil || len(vm.Confirm.Stops) == 0 {
		t.Fatalf("no confirm: %.400q", rec.Body)
	}
	all := strings.Join(vm.Confirm.Stops, "|")
	for _, g := range []string{"bars: stopped for BBB", "dividends: stopped for BBB", "options: stopped for BBB"} {
		if !strings.Contains(all, g) {
			t.Fatalf("stops %q lack %q", all, g)
		}
	}
	if vm.Form == nil {
		t.Fatal("the confirm page carries a fresh token")
	}
	// AAA is held: pausing it stops options and fundamentals but names no bars.
	rec = serve(h, underlyingsPost(t, form, url.Values{"key": {"US:AAA"}, "status": {"paused"}, "options": {"on"}, "fundamentals": {"on"}}))
	all = strings.Join(decodeUnderlyings(t, rec).Confirm.Stops, "|")
	if strings.Contains(all, "bars") || !strings.Contains(all, "options: stopped for AAA") || !strings.Contains(all, "fundamentals: stopped for AAA") {
		t.Fatalf("held stops %q", all)
	}
	if st.calls() != 0 {
		t.Fatalf("%d applies", st.calls())
	}
}

func TestPostUnderlyingsSecondPostAppliesAndRedirects(t *testing.T) {
	t.Parallel()
	h, st, form, _ := underlyingsSite(t, true)
	rec := serve(h, underlyingsPost(t, form, url.Values{"key": {"US:BBB"}, "status": {"paused"}, "options": {"on"}, "confirm": {"1"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("%d %.200q", rec.Code, rec.Body)
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil || loc.Path != "/underlyings" || loc.Query().Get("saved") != "1" {
		t.Fatalf("location %q", rec.Header().Get("Location"))
	}
	if st.calls() != 1 || len(st.applied[0].Edits) != 1 || *st.applied[0].Edits[0].Status != "paused" || st.applied[0].Edits[0].Options != nil {
		t.Fatalf("applied %+v", st.applied)
	}
	// an add is mapped too, and a post that changes nothing writes nothing.
	rec = serve(h, underlyingsPost(t, form, url.Values{"symbol": {"CCC"}, "market": {"US"}, "kind": {"stock"}, "options": {"on"}, "confirm": {"1"}}))
	if rec.Code != http.StatusSeeOther || st.calls() != 2 || st.applied[1].Adds[0].Symbol != "CCC" || !st.applied[1].Adds[0].Options {
		t.Fatalf("%d %+v", rec.Code, st.applied)
	}
	rec = serve(h, underlyingsPost(t, form, url.Values{"key": {"US:BBB"}, "status": {"active"}, "options": {"on"}, "confirm": {"1"}}))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/underlyings?saved=0" || st.calls() != 2 {
		t.Fatalf("%d %q %d", rec.Code, rec.Header().Get("Location"), st.calls())
	}
}

func TestPostUnderlyingsRefusedValueIs422NamingTheField(t *testing.T) {
	t.Parallel()
	h, st, form, _ := underlyingsSite(t, true)
	st.err = invalidErr{field: "buy_price", why: "not a decimal"}
	rec := serve(h, underlyingsPost(t, form, url.Values{"key": {"US:BBB"}, "options": {"on"}, "buy_price": {"abc"}, "confirm": {"1"}}))
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(decodeUnderlyings(t, rec).Notice, "buy_price") {
		t.Fatalf("%d %.300q", rec.Code, rec.Body)
	}
	// the route refuses a save that leaves no active name, and a key not on the list.
	st.err = nil
	rec = serve(h, underlyingsPost(t, form, url.Values{"key": {"US:AAA"}, "status": {"paused"}, "confirm": {"1"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("one active name is left: %d", rec.Code)
	}
	rec = serve(h, underlyingsPost(t, form, url.Values{"key": {"US:ZZZ"}, "status": {"paused"}, "confirm": {"1"}}))
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(decodeUnderlyings(t, rec).Notice, "key") || st.calls() != 1 {
		t.Fatalf("%d %.300q", rec.Code, rec.Body)
	}
}

func TestPostUnderlyingsLastActiveNameIs422(t *testing.T) {
	t.Parallel()
	h, st, form, _ := underlyingsSite(t, true)
	st.rows = st.rows[:1]
	rec := serve(h, underlyingsPost(t, form, url.Values{"key": {"US:AAA"}, "status": {"paused"}, "options": {"on"}, "fundamentals": {"on"}, "confirm": {"1"}}))
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(decodeUnderlyings(t, rec).Notice, "at least one name must stay active") || st.calls() != 0 {
		t.Fatalf("%d %.300q", rec.Code, rec.Body)
	}
}

func TestPostUnderlyingsGuardIs403WithNoPageAndNoApply(t *testing.T) {
	t.Parallel()
	fields := func() url.Values {
		return url.Values{"key": {"US:BBB"}, "status": {"paused"}, "options": {"on"}, "confirm": {"1"}}
	}
	cases := map[string]func(r *http.Request){
		"bad token":       func(r *http.Request) { r.Body = nil; r.PostForm = nil },
		"foreign host":    func(r *http.Request) { r.Host = "evil.example:8080" },
		"foreign origin":  func(r *http.Request) { r.Header.Set("Origin", "http://evil.example") },
		"foreign peer":    func(r *http.Request) { r.RemoteAddr = "10.1.2.3:5555" },
		"wrong type":      func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") },
		"host no loopbk":  func(r *http.Request) { r.Host = "example.com" },
		"host other port": func(r *http.Request) { r.Host = "127.0.0.1:9" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h, st, form, _ := underlyingsSite(t, true)
			r := underlyingsPost(t, form, fields())
			mutate(r)
			if name == "bad token" {
				r.Body = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader("mac=x&expires=1&key=US:BBB&confirm=1")).Body
			}
			rec := serve(h, r)
			if rec.Code != http.StatusForbidden || strings.Contains(rec.Body.String(), "schema") || st.calls() != 0 {
				t.Fatalf("%d %d applies %.200q", rec.Code, st.calls(), rec.Body)
			}
		})
	}
}

func TestPostUnderlyingsStoreErrorIs500WithoutValue(t *testing.T) {
	t.Parallel()
	h, st, form, logs := underlyingsSite(t, true)
	st.err = errors.New("boom zq7x9")
	rec := serve(h, underlyingsPost(t, form, url.Values{
		"symbol": {"ZQ7X9"}, "market": {"US"}, "kind": {"stock"}, "note": {"zq7x9-note"}, "confirm": {"1"}}))
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "internal error\n" {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
	out := strings.ToLower(logs.String())
	if !strings.Contains(out, "write failed") || strings.Contains(out, "zq7x9") || strings.Contains(out, "boom") || strings.Contains(out, "sentinel-dsn") {
		t.Fatalf("log %q", logs)
	}
	if n := strings.Count(strings.TrimSpace(logs.String()), "\n") + 1; n != 1 {
		t.Fatalf("%d log lines: %q", n, logs)
	}
}

func TestPostUnderlyingsMethodIs405(t *testing.T) {
	t.Parallel()
	h, st, _, _ := underlyingsSite(t, true)
	rec := settingsGet(t, h, http.MethodGet, "/owner-underlyings")
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != http.MethodPost || st.calls() != 0 {
		t.Fatalf("%d", rec.Code)
	}
}
