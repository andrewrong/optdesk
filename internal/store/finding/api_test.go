package finding

import (
	"context"
	"testing"
)

// fakeStore is one in-memory value on both sides of the seam.
type fakeStore struct{ last Run }

func (f *fakeStore) InsertRun(_ context.Context, r Run) (Run, error) {
	r.RunID = 1
	out := make([]Finding, len(r.Findings))
	for i, fd := range r.Findings {
		fd.IsNew = true
		out[i] = fd
	}
	r.Findings = out
	f.last = r
	return r, nil
}

func (f *fakeStore) MarkNoticed(context.Context, int64) error { return nil }

func (f *fakeStore) Latest(context.Context) (Run, error) {
	r := f.last
	if r.Findings == nil {
		r.Findings = []Finding{}
	}
	fs := make([]Finding, len(r.Findings))
	for i, fd := range r.Findings {
		if fd.Evidence == nil {
			fd.Evidence = []Count{}
		}
		fs[i] = fd
	}
	r.Findings = fs
	return r, nil
}

func (f *fakeStore) Close() error { return nil }

var (
	_ Writer = (*fakeStore)(nil)
	_ Reader = (*fakeStore)(nil)
)

func TestFindingContractFake(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := &fakeStore{}
	in := Run{CheckDay: "2026-11-25", RulesTotal: 6, Findings: []Finding{{
		Rule: RuleCaptureGap, Dataset: "bars.eod", Day: "2026-11-24", Severity: SeverityWarn,
		Evidence: []Count{{Name: "missing", N: 3}},
	}}}
	stored, err := Writer(st).InsertRun(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if stored.RunID == 0 {
		t.Fatal("run id not set")
	}
	got, err := Reader(st).Latest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.RunID != stored.RunID || len(got.Findings) != 1 {
		t.Fatalf("got %+v", got)
	}
	if !got.Findings[0].IsNew {
		t.Fatal("IsNew not read")
	}
	if got.Findings[0].Evidence[0] != (Count{Name: "missing", N: 3}) {
		t.Fatalf("evidence %+v", got.Findings[0].Evidence)
	}
}

func TestReaderNonNilSlices(t *testing.T) {
	t.Parallel()
	got, err := Reader(&fakeStore{}).Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Findings == nil {
		t.Fatal("Findings is nil")
	}
	st := &fakeStore{last: Run{Findings: []Finding{{Rule: RulePlanAbsent}}}}
	got, _ = st.Latest(context.Background())
	if got.Findings[0].Evidence == nil {
		t.Fatal("Evidence is nil")
	}
}

func TestRuleIDsAreDistinct(t *testing.T) {
	t.Parallel()
	ids := []string{RuleReadingRepeat, RuleOpenLateQuote, RuleCandidatesStale, RuleCaptureGap, RulePlanAbsent, RuleGapUnclosed}
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" || seen[id] {
			t.Fatalf("empty or duplicate rule id %q", id)
		}
		seen[id] = true
	}
}

func TestDataCheckDSNEnvName(t *testing.T) {
	t.Parallel()
	if DSNEnvDataCheck != "OT_PG_DSN_DATACHECK" {
		t.Fatalf("got %q", DSNEnvDataCheck)
	}
}
