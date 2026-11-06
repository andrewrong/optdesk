package fred

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
)

// testKey is built at run time so that no key-shaped literal sits in the source.
var testKey = strings.Repeat("ab", 8) + "-" + strings.Repeat("0123", 4)

func newSource(t *testing.T, h http.HandlerFunc) sources.Source {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	s, err := New(sources.FREDConfig{BaseURL: srv.URL, APIKey: testKey, Clock: clockwork.NewFakeClock()})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

var plan = sources.FetchPlan{SeriesIDs: []string{"DGS10"}, From: "2024-01-01", To: "2024-01-31"}

func TestFetchFixture(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("fixtures/observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var q map[string][]string
	s := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		_, _ = w.Write(body)
	})
	got, err := s.Fetch(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d", len(got))
	}
	if q["series_id"][0] != "DGS10" || q["observation_start"][0] != "2024-01-01" ||
		q["observation_end"][0] != "2024-01-31" || q["file_type"][0] != "json" || q["api_key"][0] != testKey {
		t.Errorf("query = %v", q)
	}
}

func kindOf(t *testing.T, err error) contracts.ErrorKind {
	t.Helper()
	var te contracts.TypedError
	if !errors.As(err, &te) {
		t.Fatalf("not a TypedError: %v", err)
	}
	return te.Kind()
}

func TestErrorKinds(t *testing.T) {
	t.Parallel()
	for code, want := range map[int]contracts.ErrorKind{
		429: contracts.Retryable, 500: contracts.Retryable, 503: contracts.Retryable,
		400: contracts.Permanent, 404: contracts.Permanent,
	} {
		s := newSource(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) })
		_, err := s.Fetch(context.Background(), plan)
		if err == nil || kindOf(t, err) != want {
			t.Errorf("status %d: err %v, want kind %s", code, err, want)
		}
	}
}

func TestErrorKindsCancelledContext(t *testing.T) {
	t.Parallel()
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	s := newSource(t, func(_ http.ResponseWriter, _ *http.Request) { <-done })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	errc := make(chan error, 1)
	go func() {
		_, err := s.Fetch(ctx, plan)
		errc <- err
	}()
	select {
	case err := <-errc:
		if err == nil || kindOf(t, err) != contracts.Retryable {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled fetch was not prompt")
	}
}

func TestRedaction(t *testing.T) {
	t.Parallel()
	// Transport failure: the url.Error of net/http would carry the query with the key.
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	s, err := New(sources.FREDConfig{BaseURL: url, APIKey: testKey, Clock: clockwork.NewFakeClock()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Fetch(context.Background(), plan)
	if err == nil || strings.Contains(err.Error(), testKey) {
		t.Fatalf("transport error: %v", err)
	}
	// Status failure and bad body.
	s2 := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("series_id") == "BAD" {
			_, _ = w.Write([]byte("{" + testKey))
			return
		}
		w.WriteHeader(500)
		_, _ = w.Write([]byte("error " + testKey))
	})
	for _, id := range []string{"DGS10", "BAD"} {
		_, err = s2.Fetch(context.Background(), sources.FetchPlan{SeriesIDs: []string{id}})
		if err == nil || strings.Contains(err.Error(), testKey) {
			t.Fatalf("%s: %v", id, err)
		}
	}
}

func TestNewValidates(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClock()
	for _, c := range []sources.FREDConfig{
		{APIKey: "x", Clock: clk},
		{BaseURL: "http://x", Clock: clk},
		{BaseURL: "http://x", APIKey: "x"},
	} {
		if _, err := New(c); err == nil {
			t.Errorf("no error for %+v", c)
		}
	}
}
