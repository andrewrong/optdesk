package option_data_page

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/runview"
)

type fakeReasons struct {
	byDay map[contracts.Day][]runview.Reason
	err   error
	days  []contracts.Day
}

func (f *fakeReasons) RunsOn(context.Context, contracts.Day) ([]runview.Row, error) { return nil, nil }
func (f *fakeReasons) NotWired(context.Context) ([]runview.Reason, error)           { return nil, nil }
func (f *fakeReasons) ReasonsOn(_ context.Context, d contracts.Day) ([]runview.Reason, error) {
	f.days = append(f.days, d)
	return f.byDay[d], f.err
}

func reason(code, subject, detail string) runview.Reason {
	return runview.Reason{Dataset: "option_eod", Subject: subject, Code: code, Detail: detail}
}

func TestReasonNotesFilterBySubjectAndCode(t *testing.T) {
	t.Parallel()
	f := &fakeReasons{byDay: map[contracts.Day][]runview.Reason{"2026-11-06": {
		reason("field_missing", "AAPL", "iv missing"),
		reason("field_missing", "MSFT", "other symbol"),
		reason("no_chain", "AAPL", "other code"),
		reason("fields_complete", "AAPL", "all fields present"),
	}}}
	got, err := reasonNotes(t.Context(), Config{Symbol: "AAPL", Reasons: f}, "2026-11-06")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"iv missing", "all fields present"}; !slices.Equal(got, want) {
		t.Fatalf("notes = %q", got)
	}
}

func TestReasonNotesNoRecordNote(t *testing.T) {
	t.Parallel()
	f := &fakeReasons{byDay: map[contracts.Day][]runview.Reason{"2026-11-06": {reason("no_chain", "AAPL", "x")}}}
	got, err := reasonNotes(t.Context(), Config{Symbol: "AAPL", Reasons: f}, "2026-11-06")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"no field reasons recorded for 2026-11-06"}; !slices.Equal(got, want) {
		t.Fatalf("notes = %q", got)
	}
}

func TestReasonNotesNilReaderAndEmptySymbol(t *testing.T) {
	t.Parallel()
	got, err := reasonNotes(t.Context(), Config{Symbol: "AAPL"}, "2026-11-06")
	if err != nil || got != nil {
		t.Fatalf("nil reader: %q, %v", got, err)
	}
	f := &fakeReasons{}
	got, err = reasonNotes(t.Context(), Config{Reasons: f}, "2026-11-06")
	if err != nil || got != nil || len(f.days) != 0 {
		t.Fatalf("empty symbol: %q, %v, calls %v", got, err, f.days)
	}
}

func chainCfg(t *testing.T, f *fakeReasons) Config {
	c := cfg(&fakeEod{rows: []contracts.OptionEod{eodRow(t, 1, f64(1), "2026-11-06T21:00:00Z")}}, &fakeQuotes{})
	c.Symbol, c.Reasons = "AAPL", f
	return c
}

func notesOf(m map[string]any) []string {
	var out []string
	for _, n := range m["notes"].([]any) {
		out = append(out, n.(string))
	}
	return out
}

func TestBuildNotesHoldReasonsOfChainDay(t *testing.T) {
	t.Parallel()
	f := &fakeReasons{byDay: map[contracts.Day][]runview.Reason{
		"2026-11-06": {reason("field_missing", "AAPL", "of chain day")},
		"2026-11-09": {reason("field_missing", "AAPL", "of page day")},
	}}
	b, err := Build(t.Context(), chainCfg(t, f), "2026-11-09", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(f.days, []contracts.Day{"2026-11-06"}) {
		t.Fatalf("read days = %v", f.days)
	}
	if !contains(string(b), "of chain day") || contains(string(b), "of page day") {
		t.Fatalf("page = %s", b)
	}
	f2 := &fakeReasons{}
	c := cfg(&fakeEod{}, &fakeQuotes{})
	c.Symbol, c.Reasons = "AAPL", f2
	if _, err := Build(t.Context(), c, "2026-11-06", fixedAt); err != nil {
		t.Fatal(err)
	}
	if len(f2.days) != 0 {
		t.Fatalf("no chain read %v", f2.days)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func TestBuildNotesOrderAndCap(t *testing.T) {
	t.Parallel()
	f := &fakeReasons{byDay: map[contracts.Day][]runview.Reason{"2026-11-06": {
		reason("field_missing", "AAPL", "r1"), reason("fields_complete", "AAPL", "r2"),
	}}}
	c := chainCfg(t, f)
	c.Notes = []string{"n1", "n2"}
	if got := notesOf(buildMap(t, c)); !slices.Equal(got, []string{"r1", "r2", "n1", "n2"}) {
		t.Fatalf("notes = %q", got)
	}
	var many []runview.Reason
	for i := range maxNotes + 5 {
		many = append(many, reason("field_missing", "AAPL", fmt.Sprintf("r%d", i)))
	}
	f.byDay["2026-11-06"] = many
	got := notesOf(buildMap(t, c))
	if len(got) != maxNotes || got[0] != "r0" || got[maxNotes-1] != fmt.Sprintf("r%d", maxNotes-1) {
		t.Fatalf("notes = %q", got)
	}
}

func TestBuildReasonReaderErrorIsBuildError(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	b, err := Build(t.Context(), chainCfg(t, &fakeReasons{err: boom}), "2026-11-06", fixedAt)
	if !errors.Is(err, boom) || b != nil {
		t.Fatalf("got %s, %v", b, err)
	}
}
