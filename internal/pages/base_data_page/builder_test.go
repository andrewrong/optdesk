package base_data_page

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute/basecheck"
	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
	"github.com/andrewrong/optdesk/internal/compute/reconapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/basefact"
	"github.com/andrewrong/optdesk/internal/store/coverage"
	reconstore "github.com/andrewrong/optdesk/internal/store/recon"
)

const (
	schemaDir  = "../../../contracts/schemas/"
	schemaHost = "https://optdesk.invalid/schemas/"
)

var testNow = time.Date(2026, 11, 18, 9, 30, 0, 0, time.UTC)

type fileLoader struct{}

func (fileLoader) Load(url string) (any, error) {
	file, ok := strings.CutPrefix(url, schemaHost)
	if !ok || strings.ContainsAny(file, "/\\") {
		return nil, fmt.Errorf("unexpected schema url %q", url)
	}
	b, err := os.ReadFile(schemaDir + file)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}

func validate(t *testing.T, b []byte) {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(fileLoader{})
	s, err := c.Compile(schemaHost + "page.base_data_page.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(inst); err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
}

type fakeFacts struct {
	res basefact.Result
	err error
}

func (f fakeFacts) Facts(context.Context) (basefact.Result, error) { return f.res, f.err }
func (fakeFacts) Close() error                                     { return nil }

// fakeCov serves every weekday as a session of every calendar.
type fakeCov struct {
	cells      []coverage.Cell
	sessionErr error
}

func (f fakeCov) Cells(context.Context, contracts.Day, contracts.Day) ([]coverage.Cell, error) {
	return f.cells, nil
}

func (f fakeCov) Sessions(_ context.Context, cal string, from, to contracts.Day) ([]coverage.Session, error) {
	if f.sessionErr != nil {
		return nil, f.sessionErr
	}
	a, _ := time.Parse(dayFormat, string(from))
	b, _ := time.Parse(dayFormat, string(to))
	out := []coverage.Session{}
	for d := a; !d.After(b); d = d.AddDate(0, 0, 1) {
		out = append(out, coverage.Session{Calendar: cal, Day: contracts.Day(d.Format(dayFormat)),
			IsSession: d.Weekday() != time.Saturday && d.Weekday() != time.Sunday})
	}
	return out, nil
}
func (fakeCov) NewestObservations(context.Context) ([]coverage.Newest, error) { return nil, nil }
func (fakeCov) Close() error                                                  { return nil }

// fakeEval gives every row pass, except bars (warn, bar_jump) and chains (fail, crossed); NotRead rows are unknown.
// It records the Input it got.
type fakeEval struct{ got *basecheckapi.Input }

func (f fakeEval) Func() basecheckapi.Func {
	return func(in basecheckapi.Input) basecheckapi.Output {
		if f.got != nil {
			*f.got = in
		}
		unread := map[string]bool{}
		for _, id := range in.NotRead {
			unread[id] = true
		}
		out := basecheckapi.Output{Level: basecheckapi.Pass, Counts: map[basecheckapi.Level]int{}}
		for _, info := range basecheckapi.Rows {
			r := basecheckapi.Row{Dataset: info.ID, Group: info.Group, Level: basecheckapi.Pass,
				Newest: "2026-11-17", NewestValue: "value of " + info.ID, NewestSubject: "SUB",
				Checks: []basecheckapi.Check{{Rule: "rows", Level: basecheckapi.Pass, N: 40, ThresholdSource: "def"}}}
			switch {
			case unread[info.ID]:
				r = basecheckapi.Row{Dataset: info.ID, Group: info.Group, Level: basecheckapi.Unknown,
					Checks: []basecheckapi.Check{{Rule: "not_read", Level: basecheckapi.Unknown}}}
			case info.ID == "bars":
				r.Level = basecheckapi.Warn
				r.Checks = append(r.Checks, basecheckapi.Check{Rule: "bar_jump", Level: basecheckapi.Warn, N: 900, Bad: 3,
					Share: "0.0033", WarnAt: "1", ThresholdSource: "trial", Example: "NVDA 2026-11-10"})
			case info.ID == "chains":
				r.Level = basecheckapi.Fail
				r.AgeSessions = 1
				r.Checks = append(r.Checks, basecheckapi.Check{Rule: "crossed", Level: basecheckapi.Fail, N: 112004, Bad: 2352,
					Share: "0.021", WarnAt: "0.01", FailAt: "0.02", ThresholdSource: "trial", Example: "NVDA 2026-12-18 C 200"})
			}
			out.Counts[r.Level]++
			if rank(r.Level) > rank(out.Level) {
				out.Level = r.Level
			}
			out.Rows = append(out.Rows, r)
		}
		return out
	}
}

func rank(l basecheckapi.Level) int {
	return map[basecheckapi.Level]int{basecheckapi.Pass: 0, basecheckapi.Warn: 1, basecheckapi.Unknown: 2, basecheckapi.Fail: 3}[l]
}

func okFacts() fakeFacts {
	return fakeFacts{res: basefact.Result{Facts: []basefact.Fact{{Dataset: "bars", Rule: "rows", N: 40}}, Failed: []string{}}}
}

func build(t *testing.T, in Input) (map[string]any, []byte) {
	t.Helper()
	b, err := Build(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	validate(t, b)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m, b
}

func defaultInput(ev fakeEval) Input {
	return Input{Now: testNow, Facts: okFacts(), Cov: fakeCov{}, Eval: ev.Func()}
}

func gapReasons(m map[string]any) []string {
	var out []string
	for _, g := range m["gaps"].([]any) {
		out = append(out, g.(map[string]any)["reason"].(string))
	}
	return out
}

func rowIDs(m map[string]any) []string {
	var ids []string
	for _, g := range m["groups"].([]any) {
		for _, r := range g.(map[string]any)["rows"].([]any) {
			ids = append(ids, r.(map[string]any)["dataset"].(string))
		}
	}
	return ids
}

func jsonEqual(t *testing.T, got []byte, file string) {
	t.Helper()
	want, err := os.ReadFile(filepath.Join("testdata", file))
	if err != nil {
		t.Fatal(err)
	}
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want, &w); err != nil {
		t.Fatal(err)
	}
	gb, _ := json.MarshalIndent(g, "", " ")
	wb, _ := json.MarshalIndent(w, "", " ")
	if !bytes.Equal(gb, wb) {
		t.Fatalf("output differs from testdata/%s:\n%s", file, gb)
	}
}

// TestBuildGroupsAreFour: the rows lie in the four groups of the table, in table order.
func TestBuildGroupsAreFour(t *testing.T) {
	t.Parallel()
	m, _ := build(t, defaultInput(fakeEval{}))
	var names []string
	for _, g := range m["groups"].([]any) {
		names = append(names, g.(map[string]any)["name"].(string))
	}
	if got := strings.Join(names, "|"); got != "Market data|Rates and events|Company facts|Account facts" {
		t.Fatalf("groups = %s", got)
	}
	if len(rowIDs(m)) != len(basecheckapi.Rows) {
		t.Fatalf("rows = %v", rowIDs(m))
	}
}

// TestBuildPayloadMatchesFixture: the output for a fake Eval equals testdata/ok.json and validates.
func TestBuildPayloadMatchesFixture(t *testing.T) {
	t.Parallel()
	_, b := build(t, defaultInput(fakeEval{}))
	jsonEqual(t, b, "ok.json")
}

// TestBuildOneViewFailsOthersStay: a failed view group puts its rows in NotRead and gives one gap; the other rows stay.
func TestBuildOneViewFailsOthersStay(t *testing.T) {
	t.Parallel()
	var got basecheckapi.Input
	in := defaultInput(fakeEval{got: &got})
	in.Facts = fakeFacts{res: basefact.Result{Facts: []basefact.Fact{}, Failed: []string{basefact.GroupOption}}}
	m, _ := build(t, in)
	if want := "quotes,chains,open"; strings.Join(got.NotRead, ",") != want {
		t.Fatalf("NotRead = %v, want %s", got.NotRead, want)
	}
	if r := gapReasons(m); len(r) != 1 || r[0] != "view_unavailable" {
		t.Fatalf("gaps = %v", r)
	}
	text := m["gaps"].([]any)[0].(map[string]any)["text"].(string)
	if want := "这些基础数据读不到，没有判定：在用的期权报价、期权链和合约。"; text != want {
		t.Fatalf("gap text = %s", text)
	}
	if len(rowIDs(m)) != len(basecheckapi.Rows) {
		t.Fatalf("rows = %v", rowIDs(m))
	}
	if m["top"].(map[string]any)["level"] != "unknown" {
		t.Fatalf("top = %v", m["top"])
	}
}

// TestBuildAllViewsFailGap: all four views failing gives facts_unavailable, no row and no verdict call.
func TestBuildAllViewsFailGap(t *testing.T) {
	t.Parallel()
	called := false
	in := defaultInput(fakeEval{})
	in.Eval = func(basecheckapi.Input) basecheckapi.Output { called = true; return basecheckapi.Output{} }
	in.Facts = fakeFacts{err: errors.New("all four views failed")}
	m, b := build(t, in)
	if called {
		t.Fatal("Eval called")
	}
	if r := gapReasons(m); len(r) != 1 || r[0] != "facts_unavailable" {
		t.Fatalf("gaps = %v", r)
	}
	if len(rowIDs(m)) != 0 {
		t.Fatalf("rows = %v", rowIDs(m))
	}
	jsonEqual(t, b, "gap.json")
}

// TestBuildGivesBothCalendars: Days and Sessions carry both calendars, the newest session before the zone's today;
// a failed coverage read gives the gap and still evaluates the rules.
func TestBuildGivesBothCalendars(t *testing.T) {
	t.Parallel()
	var got basecheckapi.Input
	m, _ := build(t, defaultInput(fakeEval{got: &got}))
	// 2026-11-18 09:30 UTC is 04:30 on the 18th in New York and 18:30 on the 18th in Tokyo: both last closed on the 17th.
	for _, cal := range []string{"XNYS", "XTKS"} {
		if got.Days[cal] != "2026-11-17" {
			t.Errorf("Days[%s] = %s", cal, got.Days[cal])
		}
		if s := got.Sessions[cal]; len(s) == 0 || s[len(s)-1] != "2026-11-17" {
			t.Errorf("Sessions[%s] ends at %v", cal, s)
		}
	}
	if ls := m["last_sessions"].(map[string]any); ls["XNYS"] != "2026-11-17" || ls["XTKS"] != "2026-11-17" {
		t.Errorf("last_sessions = %v", ls)
	}

	// 2026-11-17 16:00 UTC is 01:00 on the 18th in Tokyo (Tokyo closed on the 17th) and 11:00 on the 17th in New York.
	early := defaultInput(fakeEval{got: &got})
	early.Now = time.Date(2026, 11, 17, 16, 0, 0, 0, time.UTC)
	build(t, early)
	if got.Days["XNYS"] != "2026-11-16" || got.Days["XTKS"] != "2026-11-17" {
		t.Errorf("Days = %v", got.Days)
	}

	bad := defaultInput(fakeEval{got: &got})
	bad.Cov = fakeCov{sessionErr: errors.New("down")}
	m, _ = build(t, bad)
	if r := gapReasons(m); len(r) != 1 || r[0] != "coverage_unavailable" {
		t.Fatalf("gaps = %v", r)
	}
	if len(got.Days) != 2 || len(got.Sessions) != 2 || len(rowIDs(m)) != len(basecheckapi.Rows) {
		t.Fatalf("Days %v Sessions %v rows %v", got.Days, got.Sessions, rowIDs(m))
	}
}

// TestBuildStepTwoFromStepOnePayload: the check list of a row is built from the row payload of the table: the
// checks of the row come with their label and the hint of "<row>.<rule>", and the summary names the failed ones.
func TestBuildStepTwoFromStepOnePayload(t *testing.T) {
	t.Parallel()
	m, _ := build(t, defaultInput(fakeEval{}))
	var chains map[string]any
	for _, g := range m["groups"].([]any) {
		for _, r := range g.(map[string]any)["rows"].([]any) {
			if row := r.(map[string]any); row["dataset"] == "chains" {
				chains = row
			}
		}
	}
	checks := chains["checks"].([]any)
	if len(checks) != 2 {
		t.Fatalf("checks = %v", checks)
	}
	crossed := checks[1].(map[string]any)
	hint, _ := Hint("chains.crossed")
	if crossed["rule"] != "crossed" || crossed["hint_zh"] != hint || crossed["label"] != "Crossed quotes" || crossed["bad"] != float64(2352) {
		t.Fatalf("crossed = %v", crossed)
	}
	if chains["failed_summary"] != "crossed 2.1%" || chains["age_text"] != "1 s" {
		t.Fatalf("row = %v", chains)
	}
	label, _ := Hint("chains")
	if chains["label_zh"] != label {
		t.Fatalf("label_zh = %v", chains["label_zh"])
	}
}

// TestBarsRowLevelIncludesR6: a fail pair over passing checks gives a fail bars row, through the real Evaluate.
func TestBarsRowLevelIncludesR6(t *testing.T) {
	t.Parallel()
	in := reconInput([]reconstore.Run{r6Run()}, basecheckapi.Fail, nil)
	in.Eval = basecheck.Evaluate
	in.Facts = fakeFacts{res: basefact.Result{Facts: []basefact.Fact{{Dataset: "bars", Rule: "rows", N: 40}}, Failed: []string{}}}
	// Not validated: the real Evaluate puts a note check of an empty level on the flex row, which is not this entry's.
	b, err := Build(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, g := range m["groups"].([]any) {
		for _, r := range g.(map[string]any)["rows"].([]any) {
			if row := r.(map[string]any); row["dataset"] == "bars" {
				if row["level"] != "fail" {
					t.Fatalf("bars level = %v", row["level"])
				}
				for _, c := range row["checks"].([]any) {
					if c.(map[string]any)["level"] == "fail" {
						t.Fatalf("a check failed: %v", c)
					}
				}
				return
			}
		}
	}
	t.Fatal("no bars row")
}

// TestReaderFailureAddsGap: a failed reader adds the gap view_unavailable and gives Eval no pairs.
func TestReaderFailureAddsGap(t *testing.T) {
	t.Parallel()
	var got basecheckapi.Input
	in := reconInput(nil, basecheckapi.Pass, nil)
	in.Eval = fakeEval{got: &got}.Func()
	in.Recon = fakeRecon{err: errors.New("down")}
	m, _ := build(t, in)
	if r := gapReasons(m); len(r) != 1 || r[0] != "view_unavailable" {
		t.Fatalf("gaps = %v", r)
	}
	if len(got.Pairs) != 0 {
		t.Fatalf("Pairs = %v", got.Pairs)
	}
}

// TestVerdictIsTheInjectedFunction: the level of the pair is what the injected function says, and Eval gets it for
// the row bars and pair R6.
func TestVerdictIsTheInjectedFunction(t *testing.T) {
	t.Parallel()
	for _, level := range []basecheckapi.Level{basecheckapi.Pass, basecheckapi.Warn, basecheckapi.Fail} {
		var got basecheckapi.Input
		var calls []reconapi.Output
		in := reconInput([]reconstore.Run{r6Run()}, level, &calls)
		in.Eval = fakeEval{got: &got}.Func()
		cc := crossOf(t, in)
		if cc["level"] != string(level) {
			t.Fatalf("level = %v, want %s", cc["level"], level)
		}
		want := basecheckapi.PairLevel{Dataset: "bars", Pair: "R6", Level: level}
		if len(got.Pairs) != 1 || got.Pairs[0] != want {
			t.Fatalf("Pairs = %v", got.Pairs)
		}
		if len(calls) != 1 || calls[0].N != 90 || calls[0].Diff != 1 || calls[0].MissingOurs != 1 {
			t.Fatalf("calls = %+v", calls)
		}
	}
}

// TestNoReaderNoCrossCheck: without a reader the page has no cross_check and no pairs, and no gap.
func TestNoReaderNoCrossCheck(t *testing.T) {
	t.Parallel()
	var got basecheckapi.Input
	m, _ := build(t, defaultInput(fakeEval{got: &got}))
	if len(got.Pairs) != 0 || len(gapReasons(m)) != 0 {
		t.Fatalf("pairs %v gaps %v", got.Pairs, gapReasons(m))
	}
}

// TestHistoryLostCheckNotJudged: a history_lost check of an empty level is built into the page with its label and
// hint, and the level of its row is the level without the check.
func TestHistoryLostCheckNotJudged(t *testing.T) {
	t.Parallel()
	base := fakeEval{}.Func()
	with := func(in basecheckapi.Input) basecheckapi.Output {
		out := base(in)
		for i := range out.Rows {
			if out.Rows[i].Dataset == "open" {
				out.Rows[i].Checks = append(out.Rows[i].Checks, basecheckapi.Check{Rule: "history_lost", N: 5, Bad: 2})
			}
		}
		return out
	}
	find := func(ev basecheckapi.Func) map[string]any {
		in := defaultInput(fakeEval{})
		in.Eval = ev
		b, err := Build(context.Background(), in) // not validated: the schema may not know an empty level
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		for _, g := range m["groups"].([]any) {
			for _, r := range g.(map[string]any)["rows"].([]any) {
				if row := r.(map[string]any); row["dataset"] == "open" {
					return row
				}
			}
		}
		t.Fatal("no open row")
		return nil
	}
	plain, got := find(base), find(with)
	if plain["level"] != got["level"] {
		t.Fatalf("level %v != %v", got["level"], plain["level"])
	}
	wantHint, _ := Hint("open.history_lost")
	for _, c := range got["checks"].([]any) {
		if c := c.(map[string]any); c["rule"] == "history_lost" {
			if c["label"] != "History lost" || c["hint_zh"] != wantHint || wantHint == "" {
				t.Fatalf("check = %v", c)
			}
			return
		}
	}
	t.Fatalf("no history_lost check: %v", got["checks"])
}

// TestBaseDataGapTextsAreChinese: every gap text holds a Chinese character and none is an old English sentence.
func TestBaseDataGapTextsAreChinese(t *testing.T) {
	t.Parallel()
	old := []string{
		"All four fact views failed to read", "The coverage of the ledger could not be read",
		"The cross-check runs could not be read", "These base data could not be read", "stock bars",
	}
	all := defaultInput(fakeEval{})
	all.Facts = fakeFacts{err: errors.New("all four views failed")}
	one := defaultInput(fakeEval{})
	one.Facts = fakeFacts{res: basefact.Result{Facts: []basefact.Fact{}, Failed: []string{basefact.GroupBars, basefact.GroupAccount}}}
	cov := defaultInput(fakeEval{})
	cov.Cov = fakeCov{sessionErr: errors.New("down")}
	recon := reconInput(nil, basecheckapi.Pass, nil)
	recon.Recon = fakeRecon{err: errors.New("down")}
	for name, in := range map[string]Input{"all": all, "one": one, "coverage": cov, "recon": recon} {
		m, _ := build(t, in)
		gaps := m["gaps"].([]any)
		if len(gaps) == 0 {
			t.Fatalf("%s: no gap", name)
		}
		for _, g := range gaps {
			text := g.(map[string]any)["text"].(string)
			chinese := false
			for _, r := range text {
				if r >= 0x4e00 && r <= 0x9fff {
					chinese = true
				}
			}
			if !chinese {
				t.Errorf("%s: no Chinese character in %q", name, text)
			}
			for _, o := range old {
				if strings.Contains(text, o) {
					t.Errorf("%s: old sentence %q in %q", name, o, text)
				}
			}
		}
		if name == "one" {
			if want := "这些基础数据读不到，没有判定：股票日线；券商账户数据。"; gaps[0].(map[string]any)["text"] != want {
				t.Errorf("one: %v", gaps[0])
			}
		}
	}
}
