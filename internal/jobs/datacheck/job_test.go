package datacheck

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/coverage"
	"github.com/andrewrong/optdesk/internal/store/finding"
)

const checkDay = contracts.Day("2026-11-26")

type fakeCov struct {
	mu    sync.Mutex
	cals  []string
	days  map[string][]coverage.Session
	fails bool
}

func (c *fakeCov) Cells(context.Context, contracts.Day, contracts.Day) ([]coverage.Cell, error) {
	return []coverage.Cell{}, nil
}
func (c *fakeCov) Sessions(_ context.Context, cal string, _, _ contracts.Day) ([]coverage.Session, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cals = append(c.cals, cal)
	if c.fails {
		return nil, errors.New("down")
	}
	return c.days[cal], nil
}
func (c *fakeCov) NewestObservations(context.Context) ([]coverage.Newest, error) {
	return []coverage.Newest{}, nil
}
func (c *fakeCov) Close() error { return nil }

type fakeProbe struct{ finding.Probe }

type fakeOut struct {
	mu       sync.Mutex
	inserted []finding.Run
	noticed  []int64
	markNew  bool
}

func (o *fakeOut) InsertRun(_ context.Context, r finding.Run) (finding.Run, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.inserted = append(o.inserted, r)
	r.RunID = int64(len(o.inserted))
	fs := make([]finding.Finding, len(r.Findings))
	for i, f := range r.Findings {
		f.IsNew = o.markNew
		fs[i] = f
	}
	r.Findings = fs
	return r, nil
}
func (o *fakeOut) MarkNoticed(_ context.Context, id int64) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.noticed = append(o.noticed, id)
	return nil
}
func (o *fakeOut) Close() error { return nil }

type fakeAnn struct {
	texts []string
	err   error
}

func (a *fakeAnn) Announce(_ context.Context, text string) error {
	a.texts = append(a.texts, text)
	return a.err
}

func sessions(days ...string) []coverage.Session {
	var out []coverage.Session
	for _, d := range days {
		out = append(out, coverage.Session{Day: contracts.Day(d), IsSession: true})
	}
	return out
}

func okRule(id string, fs ...finding.Finding) Rule {
	return Rule{ID: id, Run: func(context.Context, Input) ([]finding.Finding, error) { return fs, nil }}
}

func badRule(id string) Rule {
	return Rule{ID: id, Run: func(context.Context, Input) ([]finding.Finding, error) { return nil, errors.New("boom") }}
}

func build(t *testing.T, cov *fakeCov, out *fakeOut, ann Announcer, rules ...Rule) contracts.Job {
	t.Helper()
	if cov == nil {
		cov = &fakeCov{}
	}
	j, err := newWithRules(Stores{Cov: cov, Probe: fakeProbe{}, Out: out, Announce: ann},
		clockwork.NewFakeClockAt(time.Date(2026, 11, 26, 23, 0, 0, 0, time.UTC)), rules)
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func TestNewRefusesNilKeys(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClock()
	full := Stores{Cov: &fakeCov{}, Probe: fakeProbe{}, Out: &fakeOut{}}
	cases := map[string]func(Stores) (Stores, bool){
		"Cov":   func(s Stores) (Stores, bool) { s.Cov = nil; return s, true },
		"Probe": func(s Stores) (Stores, bool) { s.Probe = nil; return s, true },
		"Out":   func(s Stores) (Stores, bool) { s.Out = nil; return s, true },
	}
	for key, mut := range cases {
		s, _ := mut(full)
		if _, err := New(s, clk); err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("%s: err %v", key, err)
		}
	}
	if _, err := New(full, nil); err == nil || !strings.Contains(err.Error(), "clock") {
		t.Errorf("clock: err %v", err)
	}
	if _, err := New(full, clk); err != nil {
		t.Errorf("nil Announce must be accepted: %v", err)
	}
}

func TestPlanOneKey(t *testing.T) {
	t.Parallel()
	j := build(t, nil, &fakeOut{}, nil)
	keys, h1, err := j.Plan(context.Background(), checkDay)
	if err != nil || len(keys) != 1 || keys[0].DatasetID != "datacheck.run" {
		t.Fatalf("keys %v err %v", keys, err)
	}
	_, h2, _ := j.Plan(context.Background(), "2026-11-27")
	_, h3, _ := j.Plan(context.Background(), checkDay)
	if h1 == h2 || h1 != h3 || h1 == "" {
		t.Fatalf("hashes %q %q %q", h1, h2, h3)
	}
	if j.ID() != "data_check" {
		t.Fatalf("id %q", j.ID())
	}
}

func TestNoRulesIsAnError(t *testing.T) {
	t.Parallel()
	out := &fakeOut{}
	res := build(t, nil, out, nil).Run(context.Background(), checkDay, nil)
	if res.Err == nil || !strings.Contains(res.Err.Error(), "no_rules") || res.Returned != 0 || res.Usable != 0 {
		t.Fatalf("res %+v", res)
	}
	if len(out.inserted) != 0 {
		t.Fatal("a run was written")
	}
}

func TestDefaultRulesIsEmptyAtThisEntry(t *testing.T) {
	t.Parallel()
	if r := defaultRules(); r == nil || len(r) != 0 {
		t.Fatalf("rules %v", r)
	}
}

func TestRuleErrorBecomesUnknown(t *testing.T) {
	t.Parallel()
	out := &fakeOut{}
	res := build(t, nil, out, nil, okRule("a"), badRule("b")).Run(context.Background(), checkDay, nil)
	if res.Err != nil || res.Returned != 2 || res.Usable != 1 {
		t.Fatalf("res %+v", res)
	}
	if len(out.inserted) != 1 {
		t.Fatalf("runs %d", len(out.inserted))
	}
	r := out.inserted[0]
	if r.RulesTotal != 2 || r.RulesUnknown != 1 || r.CheckDay != checkDay || len(r.Findings) != 1 {
		t.Fatalf("run %+v", r)
	}
	f := r.Findings[0]
	if f.Rule != "b" || f.Dataset != "b" || f.Day != checkDay || f.Severity != finding.SeverityUnknown ||
		len(f.Evidence) != 1 || f.Evidence[0] != (finding.Count{Name: "read_failed", N: 1}) {
		t.Fatalf("finding %+v", f)
	}
}

func TestAllUnknownIsUsableZero(t *testing.T) {
	t.Parallel()
	out := &fakeOut{}
	res := build(t, nil, out, nil, badRule("a"), badRule("b")).Run(context.Background(), checkDay, nil)
	if res.Err != nil || res.Returned != 2 || res.Usable != 0 || len(out.inserted) != 1 {
		t.Fatalf("res %+v runs %d", res, len(out.inserted))
	}
}

func TestWindowsAreLastFiveSessionsBeforeCheckDay(t *testing.T) {
	t.Parallel()
	cov := &fakeCov{days: map[string][]coverage.Session{
		"XNYS": append(sessions("2026-11-16", "2026-11-17", "2026-11-18", "2026-11-19", "2026-11-20", "2026-11-23",
			"2026-11-24", "2026-11-25"), coverage.Session{Day: "2026-11-21", IsSession: false}, coverage.Session{Day: checkDay, IsSession: true}),
		"XTKS": sessions("2026-11-25"),
	}}
	var got Input
	spy := Rule{ID: "spy", Run: func(_ context.Context, in Input) ([]finding.Finding, error) { got = in; return nil, nil }}
	j, err := newWithRules(Stores{Cov: cov, Probe: fakeProbe{}, Out: &fakeOut{}, Expected: []ExpectedDataset{{"d", "XLON"}}},
		clockwork.NewFakeClock(), []Rule{spy})
	if err != nil {
		t.Fatal(err)
	}
	if res := j.Run(context.Background(), checkDay, nil); res.Err != nil {
		t.Fatal(res.Err)
	}
	want := []contracts.Day{"2026-11-19", "2026-11-20", "2026-11-23", "2026-11-24", "2026-11-25"}
	if w := got.Windows["XNYS"]; len(w) != 5 || w[0] != want[0] || w[4] != want[4] {
		t.Fatalf("XNYS %v", w)
	}
	if len(got.Windows["XTKS"]) != 1 {
		t.Fatalf("XTKS %v", got.Windows["XTKS"])
	}
	if w, ok := got.Windows["XLON"]; !ok || len(w) != 0 {
		t.Fatalf("XLON %v %v", w, ok)
	}
}

func TestSessionsFailureIsRetryable(t *testing.T) {
	t.Parallel()
	out := &fakeOut{}
	res := build(t, &fakeCov{fails: true}, out, nil, okRule("a")).Run(context.Background(), checkDay, nil)
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || len(out.inserted) != 0 {
		t.Fatalf("res %+v", res)
	}
}

func newFinding() finding.Finding {
	return finding.Finding{Rule: finding.RuleCaptureGap, Dataset: "obs.price_bar", Day: "2026-11-25",
		Severity: finding.SeverityWarn, Evidence: []finding.Count{{Name: "keys", N: 3}}}
}

func TestSendThenMarkNoticedOnce(t *testing.T) {
	t.Parallel()
	out := &fakeOut{markNew: true}
	ann := &fakeAnn{}
	res := build(t, nil, out, ann, okRule("a", newFinding())).Run(context.Background(), checkDay, nil)
	if res.Err != nil || len(ann.texts) != 1 || len(out.noticed) != 1 || out.noticed[0] != 1 {
		t.Fatalf("res %+v texts %d noticed %v", res, len(ann.texts), out.noticed)
	}
}

func TestSendFailureNoNotice(t *testing.T) {
	t.Parallel()
	out := &fakeOut{markNew: true}
	ann := &fakeAnn{err: errors.New("telegram down")}
	res := build(t, nil, out, ann, okRule("a", newFinding())).Run(context.Background(), checkDay, nil)
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || len(out.noticed) != 0 || len(out.inserted) != 1 {
		t.Fatalf("res %+v noticed %v", res, out.noticed)
	}
}

func TestNilAnnouncerStoresOnly(t *testing.T) {
	t.Parallel()
	out := &fakeOut{markNew: true}
	res := build(t, nil, out, nil, okRule("a", newFinding())).Run(context.Background(), checkDay, nil)
	if res.Err != nil || len(out.noticed) != 0 || len(out.inserted) != 1 || res.Usable != 1 {
		t.Fatalf("res %+v noticed %v", res, out.noticed)
	}
}

func TestNothingNewStillMarksNoticedWithoutMessage(t *testing.T) {
	t.Parallel()
	out := &fakeOut{markNew: false}
	ann := &fakeAnn{}
	res := build(t, nil, out, ann, okRule("a", newFinding())).Run(context.Background(), checkDay, nil)
	if res.Err != nil || len(ann.texts) != 0 || len(out.noticed) != 1 {
		t.Fatalf("res %+v texts %d noticed %v", res, len(ann.texts), out.noticed)
	}
}

func TestRunDedupesBeforeStoring(t *testing.T) {
	t.Parallel()
	out := &fakeOut{}
	capture := newFinding()
	gap := capture
	gap.Rule = finding.RuleGapUnclosed
	build(t, nil, out, nil, okRule("a", capture, gap)).Run(context.Background(), checkDay, nil)
	if n := len(out.inserted[0].Findings); n != 1 {
		t.Fatalf("findings %d", n)
	}
}
