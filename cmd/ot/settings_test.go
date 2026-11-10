package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
