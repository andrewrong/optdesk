package sec

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
)

const secret = "Contact Name contact-text-marker"

func server(t *testing.T, status int) (*httptest.Server, *[]string, *[]string) {
	t.Helper()
	var mu sync.Mutex
	var paths, agents []string
	cf := readFile(t, "companyfacts_sample.json")
	sub := readFile(t, "submissions_sample.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		agents = append(agents, r.Header.Get("User-Agent"))
		mu.Unlock()
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/xbrl/companyfacts/CIK"):
			_, _ = w.Write(cf)
		case strings.HasPrefix(r.URL.Path, "/submissions/CIK"):
			_, _ = w.Write(sub)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &paths, &agents
}

func TestFetchBothDocuments(t *testing.T) {
	t.Parallel()
	srv, paths, agents := server(t, http.StatusOK)
	src, err := New(sources.SECConfig{BaseURL: srv.URL, UserAgent: secret, Clock: clockwork.NewFakeClockAt(retrieved)})
	if err != nil {
		t.Fatal(err)
	}
	got, err := src.Fetch(context.Background(), sources.FactPlan{Subjects: []string{"cik:0000320193"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(*paths) != 2 || (*paths)[0] != "/api/xbrl/companyfacts/CIK0000320193.json" || (*paths)[1] != "/submissions/CIK0000320193.json" {
		t.Fatalf("paths %v", *paths)
	}
	if (*agents)[0] != secret || (*agents)[1] != secret {
		t.Errorf("agents %v", *agents)
	}
	if len(find(got, "sec", "StandardIndustrialClassification")) != 1 || len(find(got, "dei", "EntityCommonStockSharesOutstanding")) != 1 {
		t.Errorf("facts %+v", got)
	}
}

func TestPacingFiveAPerSecond(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClock()
	p := newPacer(clk)
	for i := 0; i < 5; i++ {
		if d := p.reserve(); d != time.Duration(i)*200*time.Millisecond {
			t.Fatalf("request %d waits %v", i, d)
		}
	}
	clk.Advance(time.Second)
	if d := p.reserve(); d != 0 {
		t.Fatalf("after idle waits %v", d)
	}

	// Both endpoints count: one subject takes two slots, so the next request waits 400ms.
	srv, _, _ := server(t, http.StatusOK)
	c, err := New(sources.SECConfig{BaseURL: srv.URL, UserAgent: secret, Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Fetch(context.Background(), sources.FactPlan{Subjects: []string{"cik:0000320193"}}); err != nil {
		t.Fatal(err)
	}
	if d := c.(*client).pacer.reserve(); d != 400*time.Millisecond {
		t.Fatalf("after one subject waits %v, want 400ms", d)
	}
}

func TestErrorHidesUserAgent(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusForbidden, http.StatusTooManyRequests, http.StatusInternalServerError} {
		srv, _, _ := server(t, status)
		src, _ := New(sources.SECConfig{BaseURL: srv.URL, UserAgent: secret, Clock: clockwork.NewFakeClock()})
		_, err := src.Fetch(context.Background(), sources.FactPlan{Subjects: []string{"cik:0000320193"}})
		te, ok := err.(contracts.TypedError)
		if !ok {
			t.Fatalf("status %d: %v is not typed", status, err)
		}
		want := contracts.Permanent
		if status != http.StatusForbidden {
			want = contracts.Retryable
		}
		if te.Kind() != want || strings.Contains(err.Error(), "contact-text-marker") || strings.Contains(err.Error(), "Contact Name") {
			t.Errorf("status %d: kind %v err %q", status, te.Kind(), err)
		}
	}
	// A refused connection: the net/http error text is not passed on.
	srv, _, _ := server(t, http.StatusOK)
	base := srv.URL
	srv.Close()
	src, _ := New(sources.SECConfig{BaseURL: base, UserAgent: secret, Clock: clockwork.NewFakeClock()})
	_, err := src.Fetch(context.Background(), sources.FactPlan{Subjects: []string{"cik:0000320193"}})
	if err == nil || strings.Contains(err.Error(), "Contact Name") || strings.Contains(err.Error(), base) {
		t.Errorf("err %v", err)
	}
	// A subject that is no CIK is refused before any request.
	if _, err := src.Fetch(context.Background(), sources.FactPlan{Subjects: []string{"edinet:E1"}}); err == nil {
		t.Error("bad subject accepted")
	}
}
