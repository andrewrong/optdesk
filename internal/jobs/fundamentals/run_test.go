package fundamentals

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
)

type fakeClock struct{}

func (fakeClock) Now() time.Time { return time.Unix(0, 0) }

type fakeSource struct {
	facts []contracts.FundamentalFact
	err   error
	plans []sources.FactPlan
}

func (s *fakeSource) Fetch(_ context.Context, p sources.FactPlan) ([]contracts.FundamentalFact, error) {
	s.plans = append(s.plans, p)
	return s.facts, s.err
}

// fakeWriter keys rows like the store: the eight-column key, a rerun adds nothing.
type fakeWriter struct {
	rows  map[string]contracts.FundamentalFact
	calls int
	err   error
}

func (w *fakeWriter) InsertFacts(_ context.Context, rows []contracts.FundamentalFact) error {
	w.calls++
	if w.err != nil {
		return w.err
	}
	if w.rows == nil {
		w.rows = map[string]contracts.FundamentalFact{}
	}
	for _, r := range rows {
		start := "0001-01-01"
		if r.PeriodStart != nil {
			start = fmt.Sprint(*r.PeriodStart)
		}
		k := fmt.Sprint(r.Source, "|", r.Subject, "|", r.DocID, "|", r.Taxonomy, "|", r.Concept, "|", r.Unit, "|", start, "|", r.PeriodEnd)
		if _, ok := w.rows[k]; !ok {
			w.rows[k] = r
		}
	}
	return nil
}

type kindErr struct{ kind contracts.ErrorKind }

func (e *kindErr) Error() string             { return "boom" }
func (e *kindErr) Kind() contracts.ErrorKind { return e.kind }

func fact(subject, concept string) contracts.FundamentalFact {
	return contracts.FundamentalFact{
		Source: "sec", Subject: subject, DocID: "0001", Taxonomy: "us-gaap",
		Concept: concept, Unit: "USD", Value: "1",
	}
}

func entry(issuer, market, symbol string) jobs.WatchEntry {
	return jobs.WatchEntry{Issuer: issuer, Market: market, Contract: broker.Contract{Symbol: symbol}}
}

func build(t *testing.T, src sources.FactSource, w *fakeWriter, entries ...jobs.WatchEntry) contracts.Job {
	t.Helper()
	j, err := New(src, w, entries, fakeClock{})
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func TestPlanOneKeyPerCik(t *testing.T) {
	t.Parallel()
	j := build(t, &fakeSource{}, &fakeWriter{},
		entry("cik:0000789019", "US", "MSFT"),
		entry("cik:0000320193", "US", "AAPL"),
		entry("cik:0000111111", "JP", "X"),
		entry("edinet:E01", "US", "Y"),
		entry("", "US", "Z"),
	)
	keys, hash, err := j.Plan(context.Background(), "2026-11-05")
	if err != nil || hash == "" {
		t.Fatalf("hash %q err %v", hash, err)
	}
	want := []contracts.ExpectedKey{
		{DatasetID: "obs.fundamental_fact", SubjectKey: "cik:0000320193"},
		{DatasetID: "obs.fundamental_fact", SubjectKey: "cik:0000789019"},
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %v", keys)
	}
	_, other, _ := j.Plan(context.Background(), "2026-11-06")
	if other == hash {
		t.Fatal("hash ignores the day")
	}
	j2 := build(t, &fakeSource{}, &fakeWriter{}, entry("cik:0000320193", "US", "AAPL"))
	_, h2, _ := j2.Plan(context.Background(), "2026-11-05")
	if h2 == hash {
		t.Fatal("hash ignores the issuers")
	}
}

func TestNoCikSkippedAndNamed(t *testing.T) {
	t.Parallel()
	j := build(t, &fakeSource{}, &fakeWriter{},
		entry("cik:0000320193", "US", "AAPL"),
		entry("", "US", "NOCIK"),
		entry("sec:123", "US", "ODD"),
	)
	want := []jobs.FundamentalsSkip{{Subject: "NOCIK", Reason: "no_cik"}, {Subject: "sec:123", Reason: "no_cik"}}
	got := j.(jobs.FundamentalsSkips).Skipped()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("skipped = %v", got)
	}
}

func TestEdinetSkippedWhileNil(t *testing.T) {
	t.Parallel()
	j := build(t, &fakeSource{}, &fakeWriter{}, entry("edinet:E01234", "US", "SONY"), entry("cik:0000320193", "US", "AAPL"))
	want := []jobs.FundamentalsSkip{{Subject: "edinet:E01234", Reason: "edinet_not_fetched"}}
	if got := j.(jobs.FundamentalsSkips).Skipped(); !reflect.DeepEqual(got, want) {
		t.Fatalf("skipped = %v", got)
	}
	keys, _, _ := j.Plan(context.Background(), "2026-11-05")
	if len(keys) != 1 || keys[0].SubjectKey != "cik:0000320193" {
		t.Fatalf("keys = %v", keys)
	}
}

func TestRunTwiceAddsZero(t *testing.T) {
	t.Parallel()
	src := &fakeSource{facts: []contracts.FundamentalFact{fact("cik:0000320193", "Revenues"), fact("cik:0000320193", "Assets")}}
	w := &fakeWriter{}
	j := build(t, src, w, entry("cik:0000320193", "US", "AAPL"))
	ctx := context.Background()
	keys, _, _ := j.Plan(ctx, "2026-11-05")
	res := j.Run(ctx, "2026-11-05", keys)
	if res.Err != nil || res.Returned != 2 || res.Usable != 2 || len(w.rows) != 2 {
		t.Fatalf("result %+v rows %d", res, len(w.rows))
	}
	if len(src.plans) != 1 || src.plans[0].From != "2026-10-22" || src.plans[0].To != "2026-11-05" ||
		!reflect.DeepEqual(src.plans[0].Subjects, []string{"cik:0000320193"}) {
		t.Fatalf("plan = %+v", src.plans)
	}
	j.Run(ctx, "2026-11-05", keys)
	if len(w.rows) != 2 || w.calls != 2 {
		t.Fatalf("second run left %d rows after %d calls", len(w.rows), w.calls)
	}
}

func TestFetchErrorKinds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	keys := []contracts.ExpectedKey{{DatasetID: "obs.fundamental_fact", SubjectKey: "cik:0000320193"}}
	for _, tc := range []struct {
		err  error
		want contracts.ErrorKind
	}{
		{errors.New("plain"), contracts.Permanent},
		{&kindErr{contracts.Retryable}, contracts.Retryable},
	} {
		w := &fakeWriter{}
		j := build(t, &fakeSource{err: tc.err}, w, entry("cik:0000320193", "US", "AAPL"))
		res := j.Run(ctx, "2026-11-05", keys)
		var te contracts.TypedError
		if !errors.As(res.Err, &te) || te.Kind() != tc.want {
			t.Fatalf("err %v want %s", res.Err, tc.want)
		}
		if w.calls != 0 {
			t.Fatal("wrote after a failed fetch")
		}
	}
}

func TestWriterErrorRetryable(t *testing.T) {
	t.Parallel()
	src := &fakeSource{facts: []contracts.FundamentalFact{fact("cik:0000320193", "Revenues")}}
	j := build(t, src, &fakeWriter{err: errors.New("db down")}, entry("cik:0000320193", "US", "AAPL"))
	keys, _, _ := j.Plan(context.Background(), "2026-11-05")
	res := j.Run(context.Background(), "2026-11-05", keys)
	var te contracts.TypedError
	if !errors.As(res.Err, &te) || te.Kind() != contracts.Retryable || res.Usable != 0 || res.Returned != 1 {
		t.Fatalf("result %+v", res)
	}
}

func TestEmbeddedManifest(t *testing.T) {
	t.Parallel()
	j := build(t, &fakeSource{}, &fakeWriter{})
	if j.ID() != "fundamentals" {
		t.Fatalf("id %q", j.ID())
	}
	fj := j.(*job)
	if fj.m.Version != 1 || fj.m.LookbackDays != 14 {
		t.Fatalf("manifest %+v", fj.m)
	}
	cal, open, closeOff := fj.Window()
	if cal != "XNYS" || open != 0 || closeOff != 96*time.Hour {
		t.Fatalf("window %s %v %v", cal, open, closeOff)
	}
	c, anchor, zone, _ := fj.Schedule()
	if c != "XNYS" || anchor != "time" || zone != "America/New_York" || fj.RunBound() <= 0 {
		t.Fatalf("schedule %s %s %s", c, anchor, zone)
	}
	if _, err := New(nil, &fakeWriter{}, nil, fakeClock{}); err == nil {
		t.Fatal("nil source accepted")
	}
	if _, err := scheduleOf(scheduleTable{Calendar: "XNYS", Anchor: "time"}); err == nil {
		t.Fatal("time anchor without zone accepted")
	}
}

func TestEdinetJPEntrySkippedAndNamed(t *testing.T) {
	t.Parallel()
	j := build(t, &fakeSource{}, &fakeWriter{},
		entry("edinet:E01777", "JP", "SONY"),
		entry("cik:0000320193", "US", "AAPL"))
	want := []jobs.FundamentalsSkip{{Subject: "edinet:E01777", Reason: "edinet_not_fetched"}}
	if got := j.(jobs.FundamentalsSkips).Skipped(); !reflect.DeepEqual(got, want) {
		t.Fatalf("skipped = %v, want %v", got, want)
	}
}

func TestJPEntryWithoutIssuerNamed(t *testing.T) {
	t.Parallel()
	j := build(t, &fakeSource{}, &fakeWriter{}, entry("", "JP", "7203"))
	want := []jobs.FundamentalsSkip{{Subject: "7203", Reason: "no_cik"}}
	if got := j.(jobs.FundamentalsSkips).Skipped(); !reflect.DeepEqual(got, want) {
		t.Fatalf("skipped = %v, want %v", got, want)
	}
}
