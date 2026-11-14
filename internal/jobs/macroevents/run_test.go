package macroevents

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/macroevent"
)

func newClock() clock.Clock { return clockwork.NewFakeClock() }

// fakeSource answers by release id: a date list or an error; an id not listed answers empty.
type fakeSource struct {
	dates map[string][]contracts.Day
	errs  map[string]error
	calls []string
}

func (s *fakeSource) Dates(_ context.Context, id string, _ contracts.Day) ([]contracts.Day, error) {
	s.calls = append(s.calls, id)
	return s.dates[id], s.errs[id]
}

type call struct {
	kind, source string
	runDay       contracts.Day
	events       []macroevent.Event
}

// fakeWriter keeps the stored dates per kind and applies the first two delete rules of the store contract
// enough to report Written, Deleted and Held.
type fakeWriter struct {
	mu      sync.Mutex
	calls   []call
	stored  map[string]map[contracts.Day]bool
	fail    map[string]error
	deleted int
}

func (w *fakeWriter) ReplaceFuture(_ context.Context, kind, source string, runDay contracts.Day, evs []macroevent.Event) (macroevent.Outcome, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.calls = append(w.calls, call{kind, source, runDay, evs})
	if err := w.fail[kind]; err != nil {
		return macroevent.Outcome{}, err
	}
	if w.stored == nil {
		w.stored = map[string]map[contracts.Day]bool{}
	}
	have := w.stored[kind]
	if have == nil {
		have = map[contracts.Day]bool{}
		w.stored[kind] = have
	}
	var out macroevent.Outcome
	var latestNew, latestOld contracts.Day
	for d := range have {
		latestOld = max(latestOld, d)
	}
	in := map[contracts.Day]bool{}
	for _, e := range evs {
		in[e.Date] = true
		latestNew = max(latestNew, e.Date)
		if !have[e.Date] {
			have[e.Date] = true
			out.Written++
		}
	}
	if len(evs) == 0 || latestNew < latestOld {
		out.Held = true
		return out, nil
	}
	for d := range have {
		if d > runDay && !in[d] {
			delete(have, d)
			out.Deleted++
			w.deleted++
		}
	}
	return out, nil
}

type logSink struct {
	mu   sync.Mutex
	text []string
}

func (l *logSink) Enabled(context.Context, slog.Level) bool { return true }
func (l *logSink) Handle(_ context.Context, r slog.Record) error {
	l.mu.Lock()
	l.text = append(l.text, r.Message)
	l.mu.Unlock()
	return nil
}
func (l *logSink) WithAttrs([]slog.Attr) slog.Handler { return l }
func (l *logSink) WithGroup(string) slog.Handler      { return l }

func newJob(t *testing.T, src ReleaseSource, w macroevent.Writer) (*job, *logSink) {
	t.Helper()
	j, err := New(src, w, newClock())
	if err != nil {
		t.Fatal(err)
	}
	sink := &logSink{}
	jj := j.(*job)
	jj.log = slog.New(sink)
	return jj, sink
}

func runAll(j *job, day contracts.Day) contracts.JobResult {
	keys, _, _ := j.Plan(context.Background(), day)
	return j.Run(context.Background(), day, keys)
}

func (w *fakeWriter) callOf(kind string) *call {
	for i := range w.calls {
		if w.calls[i].kind == kind {
			return &w.calls[i]
		}
	}
	return nil
}

func TestRunWritesKinds(t *testing.T) {
	t.Parallel()
	src := &fakeSource{dates: map[string][]contracts.Day{"10": {"2026-12-10", "2027-01-13"}, "50": {"2026-12-04"}}}
	w := &fakeWriter{}
	j, sink := newJob(t, src, w)
	res := runAll(j, "2026-11-14")
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if want := []string{"10", "50", "54", "53", "46", "9", "192", "20"}; strings.Join(src.calls, ",") != strings.Join(want, ",") {
		t.Fatalf("fetch order %v", src.calls)
	}
	cpi := w.callOf("cpi")
	if cpi == nil || cpi.source != "fred" || cpi.runDay != "2026-11-14" || len(cpi.events) != 2 ||
		cpi.events[0] != (macroevent.Event{Kind: "cpi", Source: "fred", SourceRef: "10", Date: "2026-12-10", RunDay: "2026-11-14"}) {
		t.Fatalf("cpi call %+v", cpi)
	}
	checkFileKinds(t, w)
	if len(w.calls) != 13 {
		t.Fatalf("calls = %d", len(w.calls))
	}
	text := strings.Join(sink.text, "\n")
	if !strings.Contains(text, "cpi last 2027-01-13") || !strings.Contains(text, "employment_situation last 2026-12-04") ||
		!strings.Contains(text, "ppi last none") {
		t.Fatalf("text = %q", text)
	}
}

func TestRunEmptyAnswerStoresNothing(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{stored: map[string]map[contracts.Day]bool{"cpi": {"2026-12-10": true}}}
	j, sink := newJob(t, &fakeSource{}, w)
	res := runAll(j, "2026-11-14")
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if c := w.callOf("cpi"); c == nil || len(c.events) != 0 {
		t.Fatalf("cpi call %+v", c)
	}
	if !w.stored["cpi"]["2026-12-10"] {
		t.Fatal("stored row deleted by an empty answer")
	}
	if !strings.Contains(strings.Join(sink.text, ""), "kept stored rows of cpi: answer shorter or empty") {
		t.Fatalf("text = %v", sink.text)
	}
}

func TestRunShortAnswerHeld(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{stored: map[string]map[contracts.Day]bool{"cpi": {"2026-12-10": true, "2027-01-13": true}}}
	src := &fakeSource{dates: map[string][]contracts.Day{"10": {"2026-12-10"}}}
	j, sink := newJob(t, src, w)
	runAll(j, "2026-11-14")
	if !w.stored["cpi"]["2027-01-13"] {
		t.Fatal("stored later date deleted by a shorter answer")
	}
	if !strings.Contains(strings.Join(sink.text, ""), "kept stored rows of cpi: answer shorter or empty") {
		t.Fatalf("text = %v", sink.text)
	}
}

func TestRunForceSameDayKeepsToday(t *testing.T) {
	t.Parallel()
	// Stored: today's event and a later one. A forced rerun of the same day whose answer lacks today's event
	// still passes runDay = today to the writer, which never deletes a row of the run day.
	w := &fakeWriter{stored: map[string]map[contracts.Day]bool{"cpi": {"2026-11-14": true, "2026-12-10": true}}}
	src := &fakeSource{dates: map[string][]contracts.Day{"10": {"2026-12-10", "2027-01-13"}}}
	j, _ := newJob(t, src, w)
	res := runAll(j, "2026-11-14")
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if c := w.callOf("cpi"); c.runDay != "2026-11-14" {
		t.Fatalf("runDay %s", c.runDay)
	}
	if !w.stored["cpi"]["2026-11-14"] {
		t.Fatal("today's row deleted")
	}
}

func TestRunOneKindFails(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom " + "https://example.invalid")
	src := &fakeSource{
		dates: map[string][]contracts.Day{"10": {"2026-12-10"}, "46": {"2026-12-12"}},
		errs:  map[string]error{"50": &typedError{kind: contracts.Permanent, msg: "macroevents: status 400"}, "54": boom},
	}
	w := &fakeWriter{}
	j, sink := newJob(t, src, w)
	res := runAll(j, "2026-11-14")
	if res.Err == nil || res.Err.Kind() != contracts.Permanent || !strings.Contains(res.Err.Error(), "employment_situation") {
		t.Fatalf("err = %v", res.Err)
	}
	if w.callOf("employment_situation") != nil || w.callOf("pce") != nil {
		t.Fatal("a kind with a fetch error was written")
	}
	if w.callOf("cpi") == nil || w.callOf("ppi") == nil || w.callOf("fomc") == nil {
		t.Fatal("the other kinds were not written")
	}
	if !strings.Contains(strings.Join(sink.text, ""), "not fetched: employment_situation, pce") {
		t.Fatalf("text = %v", sink.text)
	}
	// A writer error is Retryable and does not stop the other kinds.
	w2 := &fakeWriter{fail: map[string]error{"cpi": errors.New("db down")}}
	j2, _ := newJob(t, &fakeSource{}, w2)
	res = runAll(j2, "2026-11-14")
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || len(w2.calls) != 13 {
		t.Fatalf("err %v calls %d", res.Err, len(w2.calls))
	}
}

func TestRunTwiceChangesNothing(t *testing.T) {
	t.Parallel()
	// Every FRED kind has an answer, so a repeated run has nothing to hold.
	src := &fakeSource{dates: map[string][]contracts.Day{}}
	for _, id := range []string{"10", "50", "54", "53", "46", "9", "192", "20"} {
		src.dates[id] = []contracts.Day{"2026-11-19", "2026-12-10", "2027-01-13"}
	}
	w := &fakeWriter{}
	j, sink := newJob(t, src, w)
	first := runAll(j, "2026-11-14")
	if first.Err != nil || first.Usable == 0 {
		t.Fatalf("first run %+v", first)
	}
	before := map[string]int{}
	for k, v := range w.stored {
		before[k] = len(v)
	}
	sink.text = nil
	second := runAll(j, "2026-11-14")
	if second.Err != nil || second.Usable != 0 || second.Returned != first.Returned {
		t.Fatalf("second %+v first %+v", second, first)
	}
	if w.deleted != 0 {
		t.Fatalf("deleted %d rows", w.deleted)
	}
	for k, n := range before {
		if len(w.stored[k]) != n {
			t.Errorf("%s: %d rows, was %d", k, len(w.stored[k]), n)
		}
	}
	if strings.Contains(strings.Join(sink.text, ""), "kept stored rows") {
		t.Fatalf("second run held: %v", sink.text)
	}
}
func checkFileKinds(t *testing.T, w *fakeWriter) {
	t.Helper()
	fomc := w.callOf("fomc")
	if fomc == nil || fomc.source != "file" || len(fomc.events) != 16 || fomc.events[0].RunDay != "2026-11-14" ||
		!strings.Contains(fomc.events[0].SourceRef, "federalreserve.gov") {
		t.Fatalf("fomc call %+v", fomc)
	}
	if m := w.callOf("fomc_minutes"); m == nil || m.events[0].Date != "2026-02-18" {
		t.Fatalf("minutes call %+v", m)
	}
	for _, k := range []string{"boj", "quarter_end", "us_tax"} {
		if c := w.callOf(k); c == nil || c.source != "file" {
			t.Fatalf("%s call %+v", k, c)
		}
	}
}
