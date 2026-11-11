package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// fakeUnderlyingStore is an underlying.Store over fixed rows that records what Apply is given.
type fakeUnderlyingStore struct {
	rows []underlying.Row
	mu   sync.Mutex
	// applied is every Change Apply was given; err is what it returns when set.
	applied []underlying.Change
	err     error
	opened  int
}

func (s *fakeUnderlyingStore) Current(context.Context) ([]underlying.Row, error) { return s.rows, nil }
func (s *fakeUnderlyingStore) History(context.Context, int) ([]underlying.Row, error) {
	return s.rows, nil
}

func (s *fakeUnderlyingStore) Apply(_ context.Context, c underlying.Change) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return 0, s.err
	}
	s.applied = append(s.applied, c)
	return len(c.Adds) + len(c.Edits), nil
}

func (s *fakeUnderlyingStore) Pending(context.Context, time.Time) ([]underlying.Row, error) {
	return nil, nil
}

func (s *fakeUnderlyingStore) Settle(context.Context, underlying.Outcome) (bool, error) {
	return false, nil
}
func (s *fakeUnderlyingStore) Close() error { return nil }

func (s *fakeUnderlyingStore) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.applied)
}

// holdsAAA holds 10 shares of AAA on every day.
type holdsAAA struct{}

func (holdsAAA) Positions(context.Context, string, string) ([]store.Position, error) {
	return []store.Position{{Symbol: "AAA", SecType: "STK", Quantity: "10"}}, nil
}

func (holdsAAA) Values(context.Context, string, string) ([]store.AccountValue, error) {
	return nil, nil
}

func (holdsAAA) Executions(context.Context, string, string, string) ([]store.Execution, error) {
	return nil, nil
}

func underlyingRows() []underlying.Row {
	at := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	return []underlying.Row{
		{Key: "US:AAA", Symbol: "AAA", Market: "US", Kind: underlying.KindStock, Status: underlying.StatusActive,
			Issuer: "AAA Inc", Options: true, Fundamentals: true, SetAt: at},
		{Key: "US:BBB", Symbol: "BBB", Market: "US", Kind: underlying.KindStock, Status: underlying.StatusActive,
			Issuer: "BBB Inc", Options: true, SetAt: at},
	}
}

// underlyingsEnv is the env file of an app whose route may write: a form key and the settings line.
const underlyingsEnv = "OT_FORM_KEY=" + "abababababababababababababababab" + "\nOT_PG_DSN_SETTINGS=sentinel-dsn\n"

// underlyingsSite is the site over a fake store; withEdit turns the write side on. The returned buffer is the log.
func underlyingsSite(t *testing.T, withEdit bool) (http.Handler, *fakeUnderlyingStore, settingsapi.SettingsForm, *bytes.Buffer) {
	t.Helper()
	a, logs := testApp(t, underlyingsEnv)
	a.deps.newRuleParams = newDeps().newRuleParams
	st := &fakeUnderlyingStore{rows: underlyingRows()}
	a.deps.openUnderlying = func(context.Context, string) (underlying.Store, error) { st.opened++; return st, nil }
	form := newDeps().newSettingsForm([]byte(postKey), ownerTokenTTL, a.deps.newRuleParams())
	rd := siteReaders{underlying: st, account: holdsAAA{}, aliases: []string{"main"}}
	if withEdit {
		rd.edit = &ownerEdit{form: form, listen: postListen}
	}
	return siteHandler(a, fakeClock{postNow}, "", rd, echoHandler, http.NotFoundHandler()), st, form, logs
}

type underlyingsVM struct {
	Form *struct {
		PostPath string `json:"post_path"`
		Expires  string `json:"expires"`
		MAC      string `json:"mac"`
	} `json:"form"`
	Notice  string                    `json:"notice"`
	Rows    []struct{ Key string }    `json:"rows"`
	Confirm *struct{ Stops []string } `json:"confirm"`
}

func decodeUnderlyings(t *testing.T, rec *httptest.ResponseRecorder) underlyingsVM {
	t.Helper()
	var vm underlyingsVM
	if err := json.Unmarshal(rec.Body.Bytes(), &vm); err != nil {
		t.Fatalf("%v %.300q", err, rec.Body)
	}
	return vm
}

func TestUnderlyingsPageReadOnlyHasNullForm(t *testing.T) {
	t.Parallel()
	h, _, _, _ := underlyingsSite(t, false)
	rec := settingsGet(t, h, http.MethodGet, "/underlyings")
	if rec.Code != 200 {
		t.Fatalf("%d %.200q", rec.Code, rec.Body)
	}
	if vm := decodeUnderlyings(t, rec); vm.Form != nil || len(vm.Rows) != 2 {
		t.Fatalf("%+v", vm)
	}
	if !strings.Contains(rec.Body.String(), `"form":null`) {
		t.Fatalf("%.300q", rec.Body)
	}
	// without --owner-edit the post route is not there, and a saved line is not shown.
	rec = settingsGet(t, h, http.MethodGet, "/underlyings?saved=2")
	if vm := decodeUnderlyings(t, rec); vm.Notice != "" {
		t.Fatalf("notice %q", vm.Notice)
	}
}

func TestUnderlyingsPageWithEditCarriesTokenAndSavedLine(t *testing.T) {
	t.Parallel()
	h, _, form, _ := underlyingsSite(t, true)
	rec := settingsGet(t, h, http.MethodGet, "/underlyings?saved=2")
	vm := decodeUnderlyings(t, rec)
	tok := form.Token(postNow)
	if vm.Form == nil || vm.Form.PostPath != "/owner-underlyings" || vm.Form.Expires != tok.Expires || vm.Form.MAC != tok.MAC {
		t.Fatalf("%+v", vm.Form)
	}
	if vm.Notice != "Saved 2 row(s)." {
		t.Fatalf("notice %q", vm.Notice)
	}
	for _, bad := range []string{"2x", "02", "1234567"} {
		if vm := decodeUnderlyings(t, settingsGet(t, h, http.MethodGet, "/underlyings?saved="+bad)); vm.Notice != "" {
			t.Fatalf("saved=%s gave %q", bad, vm.Notice)
		}
	}
}

func TestUnderlyingsPageNilZoneIsAnError(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.profile.Zone = nil
	_, err := buildUnderlyingsPage(t.Context(), a, fakeClock{postNow}, "", siteReaders{}, underlyingsExtras{})
	if err == nil {
		t.Fatal("no error for a nil zone")
	}
}

func TestUnderlyingsPageNoReaderIsAGapNotAnError(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	b, err := buildUnderlyingsPage(t.Context(), a, fakeClock{postNow}, "", siteReaders{}, underlyingsExtras{})
	if err != nil || !strings.Contains(string(b), `"rows":[]`) || !strings.Contains(string(b), "no_reader") {
		t.Fatalf("%v %.300s", err, b)
	}
}

func TestUnderlyingsPageReaderErrorIsAnError(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	_, err := buildUnderlyingsPage(t.Context(), a, fakeClock{postNow}, "", siteReaders{underlying: errReader{}}, underlyingsExtras{})
	if err == nil {
		t.Fatal("no error")
	}
}

type errReader struct{}

func (errReader) Current(context.Context) ([]underlying.Row, error) { return nil, errors.New("down") }
func (errReader) History(context.Context, int) ([]underlying.Row, error) {
	return nil, errors.New("down")
}
