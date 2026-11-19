package basecheck

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// load reads the invented base input of testdata/facts.json: every rule has a fact and every
// verdict is pass.
func load(t *testing.T) basecheckapi.Input {
	t.Helper()
	b, err := os.ReadFile("testdata/facts.json")
	if err != nil {
		t.Fatal(err)
	}
	var in basecheckapi.Input
	if err := json.Unmarshal(b, &in); err != nil {
		t.Fatal(err)
	}
	return in
}

// set changes the facts of one rule of one row.
func set(in *basecheckapi.Input, dataset, rule string, mut func(*basecheckapi.Fact)) {
	for i := range in.Facts {
		if f := &in.Facts[i]; f.Dataset == dataset && f.Rule == rule {
			mut(f)
		}
	}
}

// drop removes the facts of one rule of one row.
func drop(in *basecheckapi.Input, dataset, rule string) {
	var kept []basecheckapi.Fact
	for _, f := range in.Facts {
		if f.Dataset != dataset || f.Rule != rule {
			kept = append(kept, f)
		}
	}
	in.Facts = kept
}

func rowOf(t *testing.T, out basecheckapi.Output, id string) basecheckapi.Row {
	t.Helper()
	for _, r := range out.Rows {
		if r.Dataset == id {
			return r
		}
	}
	t.Fatalf("no row %s", id)
	return basecheckapi.Row{}
}

func checkOf(t *testing.T, r basecheckapi.Row, rule string) basecheckapi.Check {
	t.Helper()
	for _, c := range r.Checks {
		if c.Rule == rule {
			return c
		}
	}
	t.Fatalf("row %s has no check %s", r.Dataset, rule)
	return basecheckapi.Check{}
}

func TestBaselineAllPass(t *testing.T) {
	t.Parallel()
	out := Evaluate(load(t))
	if out.Level != basecheckapi.Pass {
		for _, r := range out.Rows {
			for _, c := range r.Checks {
				if c.Level != basecheckapi.Pass && c.Level != "" {
					t.Logf("%s.%s = %s", r.Dataset, c.Rule, c.Level)
				}
			}
		}
		t.Fatalf("level = %s, want pass", out.Level)
	}
	if len(out.Rows) != len(basecheckapi.Rows) || out.Counts[basecheckapi.Pass] != len(basecheckapi.Rows) {
		t.Errorf("rows = %d counts = %v", len(out.Rows), out.Counts)
	}
}

func TestWorstLevelWins(t *testing.T) {
	t.Parallel()
	in := load(t)
	set(&in, "bars", "bar_jump", func(f *basecheckapi.Fact) { f.Bad = 1 })   // warn
	drop(&in, "bars", "ohlc_order")                                          // unknown
	set(&in, "quotes", "crossed", func(f *basecheckapi.Fact) { f.Bad = 50 }) // fail
	set(&in, "quotes", "empty", func(f *basecheckapi.Fact) { f.Bad = 150 })  // warn
	out := Evaluate(in)
	if got := rowOf(t, out, "bars").Level; got != basecheckapi.Unknown {
		t.Errorf("bars (warn, unknown) = %s, want unknown", got)
	}
	if got := rowOf(t, out, "quotes").Level; got != basecheckapi.Fail {
		t.Errorf("quotes (fail, warn) = %s, want fail", got)
	}
	if out.Level != basecheckapi.Fail {
		t.Errorf("top = %s, want fail", out.Level)
	}
	if out.Counts[basecheckapi.Fail] != 1 || out.Counts[basecheckapi.Unknown] != 1 {
		t.Errorf("counts = %v", out.Counts)
	}
}

func TestNoRowsIsFail(t *testing.T) {
	t.Parallel()
	in := load(t)
	set(&in, "fx", "rows", func(f *basecheckapi.Fact) { f.N = 0 })
	out := Evaluate(in)
	r := rowOf(t, out, "fx")
	if r.Level != basecheckapi.Fail || checkOf(t, r, "rows").Level != basecheckapi.Fail {
		t.Errorf("fx with 0 rows = %s, want fail", r.Level)
	}
	if out.Level != basecheckapi.Fail {
		t.Errorf("top = %s", out.Level)
	}
}

func TestMissingFactIsUnknown(t *testing.T) {
	t.Parallel()
	in := load(t)
	drop(&in, "quotes", "crossed")
	out := Evaluate(in)
	r := rowOf(t, out, "quotes")
	if checkOf(t, r, "crossed").Level != basecheckapi.Unknown || r.Level != basecheckapi.Unknown {
		t.Errorf("quotes = %s, want unknown", r.Level)
	}
	in = load(t)
	in.Cells = nil
	if got := checkOf(t, rowOf(t, Evaluate(in), "bars"), "coverage_gap").Level; got != basecheckapi.Unknown {
		t.Errorf("coverage_gap with no cells = %s, want unknown", got)
	}
	in = load(t)
	drop(&in, "bars", "newest")
	r = rowOf(t, Evaluate(in), "bars")
	if checkOf(t, r, "age").Level != basecheckapi.Unknown {
		t.Errorf("age with no newest = %s, want unknown", checkOf(t, r, "age").Level)
	}
}

func TestTopLinePassOnlyWhenAllPass(t *testing.T) {
	t.Parallel()
	for _, want := range []basecheckapi.Level{basecheckapi.Warn, basecheckapi.Unknown, basecheckapi.Fail} {
		in := load(t)
		switch want {
		case basecheckapi.Warn:
			set(&in, "bars", "bar_jump", func(f *basecheckapi.Fact) { f.Bad = 1 })
		case basecheckapi.Unknown:
			drop(&in, "bars", "bar_jump")
		case basecheckapi.Fail, basecheckapi.Pass:
			set(&in, "bars", "ohlc_order", func(f *basecheckapi.Fact) { f.Bad = 1 })
		}
		if got := Evaluate(in).Level; got != want {
			t.Errorf("one row %s: top = %s", want, got)
		}
	}
	if got := Evaluate(load(t)).Level; got != basecheckapi.Pass {
		t.Errorf("all pass: top = %s", got)
	}
}

func TestShareThresholds(t *testing.T) {
	t.Parallel()
	// quotes.crossed is (share 0.001, share 0.02) over N = 1000.
	for _, tc := range []struct {
		bad   int64
		level basecheckapi.Level
		share string
	}{
		{0, basecheckapi.Pass, "0.0000"},
		{1, basecheckapi.Warn, "0.0010"},
		{19, basecheckapi.Warn, "0.0190"},
		{20, basecheckapi.Fail, "0.0200"},
	} {
		in := load(t)
		set(&in, "quotes", "crossed", func(f *basecheckapi.Fact) { f.Bad = tc.bad })
		c := checkOf(t, rowOf(t, Evaluate(in), "quotes"), "crossed")
		if c.Level != tc.level || c.Share != tc.share {
			t.Errorf("bad %d: level %s share %q, want %s %q", tc.bad, c.Level, c.Share, tc.level, tc.share)
		}
		if c.WarnAt != "0.001" || c.FailAt != "0.02" || c.ThresholdSource != "trial" {
			t.Errorf("thresholds = %q %q %q", c.WarnAt, c.FailAt, c.ThresholdSource)
		}
	}
}

func TestBarJumpNeverFails(t *testing.T) {
	t.Parallel()
	in := load(t)
	set(&in, "bars", "bar_jump", func(f *basecheckapi.Fact) { f.Bad = f.N })
	c := checkOf(t, rowOf(t, Evaluate(in), "bars"), "bar_jump")
	if c.Level != basecheckapi.Warn {
		t.Errorf("every bar jumped: %s, want warn", c.Level)
	}
}

func TestNoteDoesNotDriveVerdict(t *testing.T) {
	t.Parallel()
	in := load(t)
	set(&in, "flex", "flex_section_absent", func(f *basecheckapi.Fact) { f.Bad = 3; f.N = 3 })
	r := rowOf(t, Evaluate(in), "flex")
	if r.Level != basecheckapi.Pass {
		t.Errorf("flex with a note = %s, want pass", r.Level)
	}
	if c := checkOf(t, r, "flex_section_absent"); c.Bad != 3 || c.Level != "" {
		t.Errorf("note check = %+v, want shown with no level", c)
	}
	drop(&in, "flex", "flex_section_absent")
	if got := rowOf(t, Evaluate(in), "flex").Level; got != basecheckapi.Pass {
		t.Errorf("flex with no note fact = %s, want pass", got)
	}
}

func TestNotReadRowIsUnknown(t *testing.T) {
	t.Parallel()
	in := load(t)
	in.NotRead = []string{"chains"}
	out := Evaluate(in)
	r := rowOf(t, out, "chains")
	if r.Level != basecheckapi.Unknown || len(r.Checks) != 1 || r.Checks[0].Rule != "not_read" || r.Checks[0].Level != basecheckapi.Unknown {
		t.Errorf("chains = %+v", r)
	}
	if out.Level != basecheckapi.Unknown {
		t.Errorf("top = %s, want unknown", out.Level)
	}
	if got := rowOf(t, out, "bars").Level; got != basecheckapi.Pass {
		t.Errorf("bars = %s, want pass", got)
	}
}

func TestCoverageGapSkipsDaysBeforeHistoryStart(t *testing.T) {
	t.Parallel()
	gap := func(cells ...basecheckapi.Cell) basecheckapi.Check {
		in := load(t)
		in.Cells = cells
		return checkOf(t, rowOf(t, Evaluate(in), "bars"), "coverage_gap")
	}
	cell := func(day string, missing int) basecheckapi.Cell {
		return basecheckapi.Cell{Day: contracts.Day(day), Dataset: "bars", Keys: 10, Missing: missing}
	}
	c := gap(cell("2026-11-02", 3), cell("2026-11-06", 3), cell("2026-11-09", 0))
	if c.N != 1 || c.Bad != 0 || c.Level != basecheckapi.Pass {
		t.Errorf("before start skipped: N=%d Bad=%d level=%s, want 1 0 pass", c.N, c.Bad, c.Level)
	}
	c = gap(cell("2026-11-02", 3), cell("2026-11-06", 3), cell("2026-11-09", 0), cell("2026-11-10", 1))
	if c.N != 2 || c.Bad != 1 || c.Example != "2026-11-10" || c.Level != basecheckapi.Warn {
		t.Errorf("one bad day: N=%d Bad=%d example=%q level=%s", c.N, c.Bad, c.Example, c.Level)
	}
	c = gap(cell("2026-11-02", 3), cell("2026-11-06", 3))
	if c.Level != basecheckapi.Unknown {
		t.Errorf("only before start: level=%s, want unknown", c.Level)
	}
}

func TestRowsIgnoresSubjectOutsideRow(t *testing.T) {
	t.Parallel()
	in := load(t)
	drop(&in, "quotes", "rows")
	drop(&in, "chains", "rows")
	in.Facts = append(in.Facts,
		basecheckapi.Fact{Dataset: "quotes", Rule: "rows", Subject: "USD", N: 1},
		basecheckapi.Fact{Dataset: "quotes", Rule: "rows", Subject: "JPY", N: 0},
		basecheckapi.Fact{Dataset: "chains", Rule: "rows", Subject: "USD", N: 5},
		basecheckapi.Fact{Dataset: "chains", Rule: "rows", Subject: "JPY", N: 0},
	)
	out := Evaluate(in)
	q := checkOf(t, rowOf(t, out, "quotes"), "rows")
	if q.Level != basecheckapi.Pass || q.N != 1 {
		t.Errorf("quotes rows = %s N %d, want pass N 1", q.Level, q.N)
	}
	if c := checkOf(t, rowOf(t, out, "chains"), "rows"); c.Level != basecheckapi.Fail {
		t.Errorf("chains rows with a bare JPY = %s, want fail", c.Level)
	}
}

func TestCoverageGapSkipsDaysNotClosed(t *testing.T) {
	t.Parallel()
	gap := func(cells ...basecheckapi.Cell) basecheckapi.Check {
		in := load(t)
		in.Cells = cells
		return checkOf(t, rowOf(t, Evaluate(in), "bars"), "coverage_gap")
	}
	cell := func(day string) basecheckapi.Cell {
		return basecheckapi.Cell{Day: contracts.Day(day), Dataset: "bars", Keys: 10, Missing: 1}
	}
	c := gap(cell("2026-11-10"), cell("2026-11-18"))
	if c.N != 1 || c.Bad != 1 || c.Example != "2026-11-10" || c.Level != basecheckapi.Warn {
		t.Errorf("open day skipped: N=%d Bad=%d example=%q level=%s, want 1 1 2026-11-10 warn", c.N, c.Bad, c.Example, c.Level)
	}
	c = gap(cell("2026-11-18"))
	if c.Level != basecheckapi.Unknown {
		t.Errorf("only open day: level=%s, want unknown", c.Level)
	}
}

func TestPairLevelFoldsIntoRow(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ pair, want basecheckapi.Level }{
		{basecheckapi.Fail, basecheckapi.Fail},
		{basecheckapi.Warn, basecheckapi.Warn},
		{basecheckapi.Pass, basecheckapi.Pass},
	} {
		in := load(t)
		in.Pairs = []basecheckapi.PairLevel{{Dataset: "bars", Pair: "R6", Level: tc.pair}}
		out := Evaluate(in)
		r := rowOf(t, out, "bars")
		if r.Level != tc.want {
			t.Errorf("pair %s: bars = %s, want %s", tc.pair, r.Level, tc.want)
		}
		if len(r.Checks) != len(rowOf(t, Evaluate(load(t)), "bars").Checks) {
			t.Errorf("pair %s added a check to the row", tc.pair)
		}
		if tc.want != basecheckapi.Pass && out.Level != tc.want {
			t.Errorf("pair %s: top = %s, want %s", tc.pair, out.Level, tc.want)
		}
	}
	// a pair does not lower a worse check level
	in := load(t)
	set(&in, "bars", "ohlc_order", func(f *basecheckapi.Fact) { f.N, f.Bad = 100, 100 })
	in.Pairs = []basecheckapi.PairLevel{{Dataset: "bars", Pair: "R6", Level: basecheckapi.Warn}}
	if got := rowOf(t, Evaluate(in), "bars").Level; got != basecheckapi.Fail {
		t.Errorf("bars = %s, want fail", got)
	}
}

func TestRowWithoutPairUnchanged(t *testing.T) {
	t.Parallel()
	base := Evaluate(load(t))
	in := load(t)
	in.Pairs = []basecheckapi.PairLevel{{Dataset: "bars", Pair: "R6", Level: basecheckapi.Fail}}
	out := Evaluate(in)
	for i, r := range base.Rows {
		if r.Dataset == "bars" {
			continue
		}
		if !reflect.DeepEqual(r, out.Rows[i]) {
			t.Errorf("row %s changed: %+v vs %+v", r.Dataset, r, out.Rows[i])
		}
	}
}

func TestUnknownPairMakesRowUnknown(t *testing.T) {
	t.Parallel()
	in := load(t)
	in.Pairs = []basecheckapi.PairLevel{{Dataset: "bars", Pair: "R6", Level: basecheckapi.Unknown}}
	out := Evaluate(in)
	if got := rowOf(t, out, "bars").Level; got != basecheckapi.Unknown {
		t.Errorf("bars = %s, want unknown", got)
	}
	if out.Level != basecheckapi.Unknown {
		t.Errorf("top = %s, want unknown", out.Level)
	}
}
