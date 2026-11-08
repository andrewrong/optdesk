package serieshistory_test

import (
	"context"
	"errors"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/serieshistory"
	"github.com/andrewrong/optdesk/internal/sources"
)

var _ jobs.NewSeriesHistoryFunc = serieshistory.New

type fakeSource struct {
	plans []sources.FetchPlan
	err   error
}

func (s *fakeSource) Fetch(_ context.Context, p sources.FetchPlan) ([]contracts.SeriesPoint, error) {
	s.plans = append(s.plans, p)
	if s.err != nil {
		return nil, s.err
	}
	n := 2
	if p.SeriesIDs[0] == "VIXCLS" {
		n = 3
	}
	return make([]contracts.SeriesPoint, n), nil
}

type fakeWriter struct{ calls [][]contracts.SeriesPoint }

func (w *fakeWriter) InsertSeriesPoints(_ context.Context, rows []contracts.SeriesPoint) error {
	w.calls = append(w.calls, rows)
	return nil
}

type kindErr struct{ kind contracts.ErrorKind }

func (e kindErr) Error() string             { return "secret-detail" }
func (e kindErr) Kind() contracts.ErrorKind { return e.kind }

func newLoader(t *testing.T, s *fakeSource, w *fakeWriter) jobs.SeriesHistoryLoader {
	t.Helper()
	l, err := serieshistory.New(s, w)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func kindOf(t *testing.T, err error) contracts.ErrorKind {
	t.Helper()
	var te contracts.TypedError
	if !errors.As(err, &te) {
		t.Fatalf("not a TypedError: %v", err)
	}
	return te.Kind()
}

func TestLoadTwoSeries(t *testing.T) {
	t.Parallel()
	s, w := &fakeSource{}, &fakeWriter{}
	got, err := newLoader(t, s, w).Load(context.Background(),
		sources.FetchPlan{SeriesIDs: []string{"DGS10", "VIXCLS"}, From: "2023-01-02", To: "2026-01-02"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != (jobs.SeriesLoaded{SeriesID: "DGS10", Fetched: 2}) ||
		got[1] != (jobs.SeriesLoaded{SeriesID: "VIXCLS", Fetched: 3}) {
		t.Fatalf("got %+v", got)
	}
	if len(s.plans) != 2 || len(w.calls) != 2 || len(w.calls[0]) != 2 || len(w.calls[1]) != 3 {
		t.Fatalf("plans %d, writes %d", len(s.plans), len(w.calls))
	}
	for i, id := range []string{"DGS10", "VIXCLS"} {
		p := s.plans[i]
		if len(p.SeriesIDs) != 1 || p.SeriesIDs[0] != id || p.From != "2023-01-02" || p.To != "2026-01-02" {
			t.Fatalf("plan %d = %+v", i, p)
		}
	}
}

func TestLoadTwiceSamePoints(t *testing.T) {
	t.Parallel()
	s, w := &fakeSource{}, &fakeWriter{}
	l := newLoader(t, s, w)
	plan := sources.FetchPlan{SeriesIDs: []string{"DGS10"}, From: "2025-01-02", To: "2025-02-02"}
	a, err := l.Load(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	b, err := l.Load(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if a[0] != b[0] || len(w.calls) != 2 || len(w.calls[0]) != len(w.calls[1]) {
		t.Fatalf("a=%+v b=%+v writes=%d", a, b, len(w.calls))
	}
}

func TestRangeOverFiveYearsRefused(t *testing.T) {
	t.Parallel()
	s, w := &fakeSource{}, &fakeWriter{}
	l := newLoader(t, s, w)
	_, err := l.Load(context.Background(), sources.FetchPlan{SeriesIDs: []string{"DGS10"}, From: "2020-01-01", To: "2025-01-02"})
	if err == nil || kindOf(t, err) != contracts.Permanent {
		t.Fatalf("err = %v", err)
	}
	if _, err := l.Load(context.Background(), sources.FetchPlan{SeriesIDs: []string{"DGS10"}, From: "2020-01-01", To: "2025-01-01"}); err != nil {
		t.Fatalf("exactly 5 years refused: %v", err)
	}
	if len(s.plans) != 1 {
		t.Fatalf("fetches = %d", len(s.plans))
	}
}

func TestFromAfterToRefused(t *testing.T) {
	t.Parallel()
	s, w := &fakeSource{}, &fakeWriter{}
	_, err := newLoader(t, s, w).Load(context.Background(), sources.FetchPlan{SeriesIDs: []string{"DGS10"}, From: "2025-02-01", To: "2025-01-01"})
	if err == nil || kindOf(t, err) != contracts.Permanent || len(s.plans) != 0 {
		t.Fatalf("err = %v", err)
	}
}

func TestEmptySeriesRefused(t *testing.T) {
	t.Parallel()
	s, w := &fakeSource{}, &fakeWriter{}
	_, err := newLoader(t, s, w).Load(context.Background(), sources.FetchPlan{From: "2025-01-01", To: "2025-01-02"})
	if err == nil || kindOf(t, err) != contracts.Permanent || len(s.plans) != 0 {
		t.Fatalf("err = %v", err)
	}
}

func TestSourceErrorStopsBeforeWrite(t *testing.T) {
	t.Parallel()
	s, w := &fakeSource{err: kindErr{contracts.Retryable}}, &fakeWriter{}
	_, err := newLoader(t, s, w).Load(context.Background(), sources.FetchPlan{SeriesIDs: []string{"DGS10", "VIXCLS"}, From: "2025-01-01", To: "2025-01-02"})
	if err == nil || kindOf(t, err) != contracts.Retryable {
		t.Fatalf("err = %v", err)
	}
	if err.Error() == "secret-detail" || len(w.calls) != 0 || len(s.plans) != 1 {
		t.Fatalf("text %q, writes %d, fetches %d", err, len(w.calls), len(s.plans))
	}
	s.err = errors.New("plain")
	_, err = newLoader(t, s, w).Load(context.Background(), sources.FetchPlan{SeriesIDs: []string{"DGS10"}, From: "2025-01-01", To: "2025-01-02"})
	if err == nil || kindOf(t, err) != contracts.Permanent {
		t.Fatalf("untyped error: %v", err)
	}
}
