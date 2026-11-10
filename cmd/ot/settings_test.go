package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
	"github.com/andrewrong/optdesk/internal/compute/settingsform"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
)

// fakeOwnerReader is an ownerset.Reader with no stored row.
type fakeOwnerReader struct{}

func (fakeOwnerReader) Params(context.Context, time.Time) ([]ownerset.Param, error) { return nil, nil }
func (fakeOwnerReader) Exclusions(context.Context, time.Time) ([]ownerset.Exclusion, error) {
	return nil, nil
}
func (fakeOwnerReader) History(context.Context, int) ([]ownerset.HistoryRow, error) { return nil, nil }
func (fakeOwnerReader) Version(context.Context, time.Time) (int64, error)           { return 0, nil }
func (fakeOwnerReader) Close() error                                                { return nil }

type settingsVM struct {
	Form   json.RawMessage `json:"form"`
	Params []any           `json:"params"`
	Gap    *struct {
		Reason string `json:"reason"`
	} `json:"gap"`
}

func settingsGet(t *testing.T, h http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), method, target, nil))
	return rec
}

// settingsApp is a test app with the three deps of the route filled as newDeps fills them.
func settingsApp(t *testing.T) *app {
	t.Helper()
	a, _ := testApp(t, "")
	d := newDeps()
	a.deps.settingsPage, a.deps.newRuleParams, a.deps.openOwnerSet = d.settingsPage, d.newRuleParams, d.openOwnerSet
	return a
}

func settingsSite(t *testing.T, rd siteReaders) http.Handler {
	t.Helper()
	a := settingsApp(t)
	return siteHandler(a, fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}, "", rd, echoHandler, http.NotFoundHandler())
}

func TestGetSettingsReadOnly(t *testing.T) {
	t.Parallel()
	rd := siteReaders{owner: fakeOwnerReader{}, catalog: demoOption{}, names: demoOption{}}
	for _, m := range []string{http.MethodGet, http.MethodHead} {
		rec := settingsGet(t, settingsSite(t, rd), m, "/settings")
		if rec.Code != 200 {
			t.Fatalf("%s: %d %.200q", m, rec.Code, rec.Body)
		}
		if m == http.MethodHead {
			continue
		}
		var vm settingsVM
		if err := json.Unmarshal(rec.Body.Bytes(), &vm); err != nil {
			t.Fatal(err)
		}
		if string(vm.Form) != "null" || len(vm.Params) == 0 {
			t.Fatalf("form %s params %d", vm.Form, len(vm.Params))
		}
	}
}

func TestGetSettingsNoReaderIsGap(t *testing.T) {
	t.Parallel()
	a := settingsApp(t)
	a.deps.openOwnerSet = nil
	a.profile.Name = "demo"
	rd, closeFn, err := openSiteReaders(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	h := siteHandler(a, fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}, "", rd, echoHandler, http.NotFoundHandler())
	rec := settingsGet(t, h, http.MethodGet, "/settings")
	var vm settingsVM
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &vm) != nil || vm.Gap == nil || vm.Gap.Reason != "no_reader" {
		t.Fatalf("%d %.300q", rec.Code, rec.Body)
	}
}

func TestSettingsRouteIsGetOnly(t *testing.T) {
	t.Parallel()
	h := settingsSite(t, siteReaders{owner: fakeOwnerReader{}, catalog: demoOption{}, names: demoOption{}})
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		rec := settingsGet(t, h, m, "/settings")
		if rec.Code != 405 || rec.Header().Get("Allow") != "GET, HEAD" {
			t.Fatalf("%s: %d", m, rec.Code)
		}
	}
	if rec := settingsGet(t, h, http.MethodGet, "/settings/x"); rec.Code != 404 {
		t.Fatalf("settings/x: %d", rec.Code)
	}
}

func TestSettingsBuildErrorIs500(t *testing.T) {
	t.Parallel()
	a := settingsApp(t)
	a.profile.Zone = nil
	h := siteHandler(a, fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}, "", siteReaders{}, echoHandler, http.NotFoundHandler())
	rec := settingsGet(t, h, http.MethodGet, "/settings")
	if rec.Code != 500 || !strings.Contains(rec.Body.String(), "internal error") {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
}

// fakeOwnerStore is an ownerset.Store with no stored row that records what Apply is given.
type fakeOwnerStore struct {
	fakeOwnerReader
	mu      sync.Mutex
	applied []ownerset.Change
	err     error
}

func (s *fakeOwnerStore) Apply(_ context.Context, c ownerset.Change) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return 0, s.err
	}
	s.applied = append(s.applied, c)
	return len(c.Params) + len(c.Exclusions), nil
}

func (s *fakeOwnerStore) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.applied)
}

const postListen = "127.0.0.1:8080"

// postKey is a 32-byte test key, built so that no key-shaped literal sits in the source.
var postKey = strings.Repeat("ab", 16)

var postNow = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

// postSite is the site with the route on, over a fake store and settingsform.New with a test key.
func postSite(t *testing.T, withRoute bool) (http.Handler, *fakeOwnerStore, settingsapi.SettingsForm) {
	t.Helper()
	a := settingsApp(t)
	st := &fakeOwnerStore{}
	form := settingsform.New([]byte(postKey), ownerTokenTTL, a.deps.newRuleParams())
	rd := siteReaders{owner: st, catalog: demoOption{}, names: demoOption{}}
	if withRoute {
		rd.edit = &ownerEdit{form: form, writer: st, listen: postListen}
	}
	return siteHandler(a, fakeClock{postNow}, "", rd, echoHandler, http.NotFoundHandler()), st, form
}

// postRequest is a well-formed post: loopback peer, loopback Host, form content type, a fresh token.
func postRequest(t *testing.T, form settingsapi.SettingsForm, fields url.Values) *http.Request {
	t.Helper()
	tok := form.Token(postNow)
	fields.Set("expires", tok.Expires)
	fields.Set("mac", tok.MAC)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/owner-settings", strings.NewReader(fields.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.RemoteAddr = "127.0.0.1:5555"
	r.Host = postListen
	return r
}

func serve(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func TestPostPreviewWritesNothing(t *testing.T) {
	t.Parallel()
	h, st, form := postSite(t, true)
	rec := serve(h, postRequest(t, form, url.Values{"param.min_dte": {"30"}}))
	if rec.Code != 200 || st.calls() != 0 {
		t.Fatalf("%d, %d applies, %.300q", rec.Code, st.calls(), rec.Body)
	}
	var vm struct {
		Confirm *struct {
			Rows []struct{ Label, Old, New string }
		} `json:"confirm"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &vm); err != nil || vm.Confirm == nil || len(vm.Confirm.Rows) != 1 {
		t.Fatalf("confirm: %v %.300q", err, rec.Body)
	}
	if r := vm.Confirm.Rows[0]; r.Old != "21" || r.New != "30" {
		t.Fatalf("row %+v", r)
	}
	// no difference: the page answers nothing_changed and writes nothing.
	rec = serve(h, postRequest(t, form, url.Values{"param.min_dte": {"21"}}))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "nothing_changed") || st.calls() != 0 {
		t.Fatalf("%d %.300q", rec.Code, rec.Body)
	}
}

func TestPostConfirmSavesAndRedirects(t *testing.T) {
	t.Parallel()
	h, st, form := postSite(t, true)
	rec := serve(h, postRequest(t, form, url.Values{"param.min_dte": {"30"}, "confirm": {"1"}}))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/settings?saved=1" {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Location"))
	}
	if st.calls() != 1 || len(st.applied[0].Params) != 1 || st.applied[0].Params[0] != (ownerset.ParamValue{Key: "min_dte", Value: "30"}) {
		t.Fatalf("applied %+v", st.applied)
	}
	// an exclusion of a shown instrument is mapped too.
	id := strconv.FormatInt(demoOptionIDs[0], 10)
	rec = serve(h, postRequest(t, form, url.Values{"row": {id}, "excluded": {id}, "confirm": {"1"}}))
	if rec.Code != http.StatusSeeOther || st.calls() != 2 || len(st.applied[1].Exclusions) != 1 {
		t.Fatalf("%d %+v", rec.Code, st.applied)
	}
	// the page of the redirect carries the notice and the token.
	rd := siteReaders{owner: st, catalog: demoOption{}, names: demoOption{}, edit: &ownerEdit{form: form, writer: st, listen: postListen}}
	a := settingsApp(t)
	g := siteHandler(a, fakeClock{postNow}, "", rd, echoHandler, http.NotFoundHandler())
	rec = settingsGet(t, g, http.MethodGet, "/settings?saved=1")
	var vm struct {
		Form   *struct{ PostPath string } `json:"form"`
		Notice *struct{ Kind string }     `json:"notice"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &vm); err != nil || vm.Form == nil || vm.Notice == nil || vm.Notice.Kind != "saved" {
		t.Fatalf("%v %.400q", err, rec.Body)
	}
}

func TestPostStoreErrorIs500WithoutValue(t *testing.T) {
	t.Parallel()
	h, st, form := postSite(t, true)
	st.err = errors.New("boom 30")
	rec := serve(h, postRequest(t, form, url.Values{"param.min_dte": {"30"}, "confirm": {"1"}}))
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "30") || strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
}

func TestPostBadTokenIs403AndWritesNothing(t *testing.T) {
	t.Parallel()
	h, st, form := postSite(t, true)
	r := postRequest(t, form, url.Values{"param.min_dte": {"30"}, "confirm": {"1"}})
	body := strings.NewReader(url.Values{"param.min_dte": {"30"}, "confirm": {"1"}, "expires": {"1"}, "mac": {"x"}}.Encode())
	r.Body = io.NopCloser(body)
	r.ContentLength = int64(body.Len())
	rec := serve(h, r)
	if rec.Code != 403 || st.calls() != 0 || strings.Contains(rec.Body.String(), "schema") {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
	// an expired token.
	old := settingsform.New([]byte(postKey), ownerTokenTTL, settingsApp(t).deps.newRuleParams()).Token(postNow.Add(-time.Hour))
	f := url.Values{"param.min_dte": {"30"}, "confirm": {"1"}, "expires": {old.Expires}, "mac": {old.MAC}}
	r = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/owner-settings", strings.NewReader(f.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.RemoteAddr, r.Host = "127.0.0.1:5555", postListen
	if rec := serve(h, r); rec.Code != 403 || st.calls() != 0 {
		t.Fatalf("expired: %d", rec.Code)
	}
}

func TestPostRefusedValueIs422(t *testing.T) {
	t.Parallel()
	h, st, form := postSite(t, true)
	for _, f := range []url.Values{
		{"param.min_dte": {"1"}, "confirm": {"1"}},
	} {
		rec := serve(h, postRequest(t, form, f))
		if rec.Code != 422 || !strings.Contains(rec.Body.String(), `"error"`) || st.calls() != 0 {
			t.Fatalf("%v: %d %.300q", f, rec.Code, rec.Body)
		}
	}
}

func TestPostUnknownKeyIs400(t *testing.T) {
	t.Parallel()
	h, st, form := postSite(t, true)
	for _, f := range []url.Values{
		{"param.nope": {"1"}, "confirm": {"1"}},
		{"param.nope": {"1"}},
		{"row": {"999"}, "confirm": {"1"}},
		{"row": {"x"}},
		{"excluded": {"999"}, "confirm": {"1"}},
	} {
		rec := serve(h, postRequest(t, form, f))
		if rec.Code != 400 || st.calls() != 0 {
			t.Fatalf("%v: %d %.200q, %d applies", f, rec.Code, rec.Body, st.calls())
		}
	}
}

func TestPostRouteAbsentWithoutFlag(t *testing.T) {
	t.Parallel()
	h, st, form := postSite(t, false)
	rec := serve(h, postRequest(t, form, url.Values{"param.min_dte": {"30"}, "confirm": {"1"}}))
	if rec.Code != 404 || st.calls() != 0 {
		t.Fatalf("%d", rec.Code)
	}
	// without the route the page is read-only.
	rec = settingsGet(t, h, http.MethodGet, "/settings")
	var vm settingsVM
	if err := json.Unmarshal(rec.Body.Bytes(), &vm); err != nil || string(vm.Form) != "null" {
		t.Fatalf("%v form %s", err, vm.Form)
	}
}

func TestPostOtherMethodIs405(t *testing.T) {
	t.Parallel()
	h, _, _ := postSite(t, true)
	for _, m := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		rec := settingsGet(t, h, m, "/owner-settings")
		if rec.Code != 405 || rec.Header().Get("Allow") != "POST" {
			t.Fatalf("%s: %d %q", m, rec.Code, rec.Header().Get("Allow"))
		}
	}
}

func TestPostHostNamesAndOrigin(t *testing.T) {
	t.Parallel()
	h, st, form := postSite(t, true)
	fields := func() url.Values { return url.Values{"param.min_dte": {"30"}, "confirm": {"1"}} }
	cases := []struct {
		name   string
		mutate func(*http.Request)
		code   int
	}{
		{"ipv4", func(r *http.Request) {}, 303},
		{"localhost", func(r *http.Request) { r.Host = "localhost:8080" }, 303},
		{"ipv6", func(r *http.Request) { r.Host = "[::1]:8080" }, 303},
		{"origin ok", func(r *http.Request) { r.Header.Set("Origin", "http://localhost:8080") }, 303},
		{"foreign host", func(r *http.Request) { r.Host = "example.com:8080" }, 403},
		{"other port", func(r *http.Request) { r.Host = "127.0.0.1:9090" }, 403},
		{"foreign origin", func(r *http.Request) { r.Header.Set("Origin", "http://evil.example") }, 403},
		{"https origin", func(r *http.Request) { r.Header.Set("Origin", "https://localhost:8080") }, 403},
		{"remote peer", func(r *http.Request) { r.RemoteAddr = "192.0.2.7:1" }, 403},
		{"json body", func(r *http.Request) { r.Header.Set("Content-Type", "application/json") }, 403},
	}
	for _, c := range cases {
		before := st.calls()
		r := postRequest(t, form, fields())
		c.mutate(r)
		rec := serve(h, r)
		if rec.Code != c.code {
			t.Errorf("%s: %d, want %d", c.name, rec.Code, c.code)
		}
		if c.code == 403 && st.calls() != before {
			t.Errorf("%s: a refused request wrote", c.name)
		}
	}
	// a body over 64 KiB is refused.
	big := url.Values{"pad": {strings.Repeat("a", 70<<10)}}
	if rec := serve(h, postRequest(t, form, big)); rec.Code != 403 {
		t.Errorf("big body: %d", rec.Code)
	}
}
