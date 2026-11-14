package macroevents

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// testKey is built at run time so that no key-shaped literal sits in the source.
var testKey = strings.Repeat("ab", 8) + "-" + strings.Repeat("0123", 4)

func newRelease(t *testing.T, h http.HandlerFunc) ReleaseSource {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	s, err := NewRelease(srv.URL, testKey)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestReleaseQuery(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("testdata/release_dates.json")
	if err != nil {
		t.Fatal(err)
	}
	var path string
	var q url.Values
	s := newRelease(t, func(w http.ResponseWriter, r *http.Request) {
		path, q = r.URL.Path, r.URL.Query()
		_, _ = w.Write(body)
	})
	got, err := s.Dates(context.Background(), "10", "2026-11-14")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"release_id": "10", "realtime_start": "2026-11-14", "include_release_dates_with_no_data": "true",
		"sort_order": "asc", "file_type": "json", "api_key": testKey}
	if path != "/fred/release/dates" || len(q) != len(want) {
		t.Fatalf("path %s query %v", path, q)
	}
	for k, v := range want {
		if q.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, q.Get(k), v)
		}
	}
	// 2026-11-10 is before from and dropped; the day itself is kept.
	if len(got) != 3 || got[0] != "2026-11-14" || got[2] != "2027-01-13" {
		t.Fatalf("dates = %v", got)
	}
	empty, _ := os.ReadFile("testdata/release_dates_empty.json")
	s2 := newRelease(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(empty) })
	if got, err := s2.Dates(context.Background(), "10", "2026-11-14"); err != nil || len(got) != 0 {
		t.Fatalf("empty: %v %v", got, err)
	}
}

func TestReleaseErrorKinds(t *testing.T) {
	t.Parallel()
	for code, want := range map[int]contracts.ErrorKind{429: contracts.Retryable, 500: contracts.Retryable,
		503: contracts.Retryable, 400: contracts.Permanent, 404: contracts.Permanent} {
		s := newRelease(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) })
		_, err := s.Dates(context.Background(), "10", "2026-11-14")
		var te contracts.TypedError
		if !errors.As(err, &te) || te.Kind() != want {
			t.Errorf("status %d: %v", code, err)
		}
	}
}

func TestReleaseErrorNeverHoldsKey(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.NotFoundHandler())
	base := srv.URL
	srv.Close()
	dead, err := NewRelease(base, testKey)
	if err != nil {
		t.Fatal(err)
	}
	s500 := newRelease(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("error " + testKey))
	})
	sBad := newRelease(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("{" + testKey)) })
	sDate := newRelease(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"release_dates":[{"date":"` + testKey + `"}]}`))
	})
	for name, s := range map[string]ReleaseSource{"transport": dead, "500": s500, "body": sBad, "date": sDate} {
		_, err := s.Dates(context.Background(), "10", "2026-11-14")
		if err == nil || strings.Contains(err.Error(), testKey) || strings.Contains(err.Error(), "http") {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := NewRelease("", testKey); err == nil {
		t.Error("empty base accepted")
	}
}
