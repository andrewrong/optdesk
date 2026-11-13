package nyfed

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
)

var clk = clockwork.NewFakeClock()

type fixture struct {
	RefRates  []json.RawMessage          `json:"refRates"`
	Malformed map[string]json.RawMessage `json:"malformedRecords"`
}

func loadFixture(t *testing.T) (body []byte, fx fixture) {
	t.Helper()
	body, err := os.ReadFile("testdata/sofr.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &fx); err != nil {
		t.Fatal(err)
	}
	return body, fx
}

func serve(t *testing.T, status int, body []byte) (sources.Source, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	src, err := New(Config{BaseURL: srv.URL, HTTP: srv.Client(), Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	return src, &hits
}

func kindOf(t *testing.T, err error) contracts.ErrorKind {
	t.Helper()
	var te contracts.TypedError
	if !errors.As(err, &te) {
		t.Fatalf("not a TypedError: %v", err)
	}
	return te.Kind()
}

func TestMapping(t *testing.T) {
	t.Parallel()
	body, _ := loadFixture(t)
	src, _ := serve(t, 200, body)
	pts, err := src.Fetch(context.Background(), sources.FetchPlan{SeriesIDs: []string{"SOFR"}, From: "2026-11-06", To: "2026-11-09"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 2 {
		t.Fatalf("got %d points, want 2 (the day before From is dropped)", len(pts))
	}
	p := pts[0] // 2026-11-09
	if p.SeriesID != "SOFR" || p.Source != "nyfed" || p.Unit != "percent" || p.Status != "present" {
		t.Fatalf("bad point %+v", p)
	}
	if p.Value == nil || *p.Value != 4.05 {
		t.Fatalf("value %v, want 4.05", p.Value)
	}
	for _, d := range []string{p.ObservationDate.Format("2006-01-02"), p.RealtimeStart.Format("2006-01-02"), p.RealtimeEnd.Format("2006-01-02")} {
		if d != "2026-11-09" {
			t.Fatalf("date %s, want 2026-11-09", d)
		}
	}
	if got := pts[1].ObservationDate.Format("2006-01-02"); got != "2026-11-06" {
		t.Fatalf("second point %s; the missing weekend must stay missing", got)
	}
}

func TestUnknownIDIsPermanent(t *testing.T) {
	t.Parallel()
	body, _ := loadFixture(t)
	src, hits := serve(t, 200, body)
	pts, err := src.Fetch(context.Background(), sources.FetchPlan{SeriesIDs: []string{"SOFR", "DGS10"}})
	if err == nil || kindOf(t, err) != contracts.Permanent || !strings.Contains(err.Error(), "DGS10") {
		t.Fatalf("err %v", err)
	}
	if pts != nil || hits.Load() != 0 {
		t.Fatalf("points %v, hits %d; want none", pts, hits.Load())
	}
}

func TestMalformedBodyIsPermanent(t *testing.T) {
	t.Parallel()
	src, _ := serve(t, 200, []byte("<html>not json"))
	pts, err := src.Fetch(context.Background(), sources.FetchPlan{SeriesIDs: []string{"SOFR"}})
	if err == nil || kindOf(t, err) != contracts.Permanent || pts != nil {
		t.Fatalf("pts %v err %v", pts, err)
	}
}

func TestServiceUnavailableIsRetryable(t *testing.T) {
	t.Parallel()
	src, _ := serve(t, 503, nil)
	pts, err := src.Fetch(context.Background(), sources.FetchPlan{SeriesIDs: []string{"SOFR"}})
	if err == nil || kindOf(t, err) != contracts.Retryable || pts != nil {
		t.Fatalf("pts %v err %v", pts, err)
	}
}

func TestTransportFailureIsRetryable(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	src, err := New(Config{BaseURL: url, Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := src.Fetch(context.Background(), sources.FetchPlan{SeriesIDs: []string{"SOFR"}}); err == nil || kindOf(t, err) != contracts.Retryable {
		t.Fatalf("err %v", err)
	}
}

// A bad record makes the whole answer Permanent, even with good records beside it.
func TestBadRecordFailsWholeAnswer(t *testing.T) {
	t.Parallel()
	_, fx := loadFixture(t)
	if len(fx.Malformed) != 9 {
		t.Fatalf("fixture has %d malformed records, want 9", len(fx.Malformed))
	}
	for name, bad := range fx.Malformed {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			recs := append([]json.RawMessage{}, fx.RefRates...)
			recs = append(recs, bad)
			body, err := json.Marshal(map[string]any{"refRates": recs})
			if err != nil {
				t.Fatal(err)
			}
			src, _ := serve(t, 200, body)
			pts, err := src.Fetch(context.Background(), sources.FetchPlan{SeriesIDs: []string{"SOFR"}})
			if err == nil || kindOf(t, err) != contracts.Permanent {
				t.Fatalf("err %v, want Permanent", err)
			}
			if pts != nil {
				t.Fatalf("got %d points, want none", len(pts))
			}
		})
	}
}

func TestNewRequiresClock(t *testing.T) {
	t.Parallel()
	if _, err := New(Config{}); err == nil || kindOf(t, err) != contracts.Permanent {
		t.Fatalf("err %v", err)
	}
}

func TestNaNRateIsPermanent(t *testing.T) {
	t.Parallel()
	body := []byte(`{"refRates":[{"effectiveDate":"2026-11-09","percentRate":"NaN"}]}`)
	src, _ := serve(t, 200, body)
	pts, err := src.Fetch(context.Background(), sources.FetchPlan{SeriesIDs: []string{"SOFR"}})
	if err == nil || kindOf(t, err) != contracts.Permanent || pts != nil {
		t.Fatalf("pts %v err %v; a NaN rate must fail the whole answer", pts, err)
	}
}
