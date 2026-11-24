package data_health_page

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/coverage"
)

const (
	schemaDir  = "../../../contracts/schemas/"
	schemaHost = "https://optdesk.invalid/schemas/"
)

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

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(fileLoader{})
	s, err := c.Compile(schemaHost + "page.data_health_page.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func validate(t *testing.T, b []byte) {
	t.Helper()
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := compileSchema(t).Validate(inst); err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
}

type fakeReader struct {
	cells    []coverage.Cell
	sessions []coverage.Session
	newest   []coverage.Newest
	err      error
	from     contracts.Day
}

func (f *fakeReader) Cells(_ context.Context, from, _ contracts.Day) ([]coverage.Cell, error) {
	f.from = from
	return f.cells, f.err
}

func (f *fakeReader) Sessions(_ context.Context, cal string, _, _ contracts.Day) ([]coverage.Session, error) {
	out := []coverage.Session{}
	for _, s := range f.sessions {
		if s.Calendar == cal {
			out = append(out, s)
		}
	}
	return out, f.err
}

func (f *fakeReader) NewestObservations(context.Context) ([]coverage.Newest, error) {
	return f.newest, f.err
}
func (f *fakeReader) Close() error { return nil }

// weekdays gives sessions of the calendar for the 60 days ending 2026-03-10.
func weekdays(cal string) []coverage.Session {
	end, _ := time.Parse("2006-01-02", "2026-03-10")
	out := []coverage.Session{}
	for i := 59; i >= 0; i-- {
		d := end.AddDate(0, 0, -i)
		out = append(out, coverage.Session{Calendar: cal, Day: contracts.Day(d.Format("2006-01-02")),
			IsSession: d.Weekday() != time.Saturday && d.Weekday() != time.Sunday})
	}
	return out
}

var testNow = time.Date(2026, 3, 10, 18, 0, 0, 0, time.UTC)

func input(r *fakeReader, jobs []Job, form *Form) Input {
	return Input{Now: testNow, Today: "2026-03-10", Cov: r, Jobs: jobs, Form: form}
}

type doc struct {
	Gaps []struct {
		Reason string `json:"reason"`
	} `json:"gaps"`
	Datasets  []Dataset `json:"datasets"`
	Glossary  []struct{ Term, Text string }
	Overwrite string `json:"overwrite_note"`
	Form      *struct{}
}

func build(t *testing.T, in Input) doc {
	t.Helper()
	b, err := Build(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	validate(t, b)
	var d doc
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestReaderErrorGivesGap(t *testing.T) {
	t.Parallel()
	d := build(t, input(&fakeReader{err: errors.New("down")}, nil, nil))
	if len(d.Gaps) != 1 || d.Gaps[0].Reason != "coverage_unavailable" || len(d.Datasets) != 0 {
		t.Fatalf("%+v", d)
	}
}

func TestDatasetRows(t *testing.T) {
	t.Parallel()
	r := &fakeReader{sessions: weekdays("XNYS"), cells: []coverage.Cell{
		{Day: "2026-03-09", Dataset: "obs.a", Keys: 1, OK: 1, MissingKeys: []string{}},
		{Day: "2026-03-09", Dataset: "obs.orphan", Keys: 1, Missing: 1, MissingKeys: []string{"k"}},
	}}
	jobs := []Job{
		{ID: "first", Kind: KindAnyDay, Calendar: "XNYS", Datasets: []string{"obs.a"}},
		{ID: "second", Kind: KindAnyDay, Calendar: "XNYS", Datasets: []string{"obs.a"}},
	}
	d := build(t, input(r, jobs, testForm))
	if len(d.Datasets) != 2 {
		t.Fatalf("%d datasets", len(d.Datasets))
	}
	a, o := d.Datasets[0], d.Datasets[1]
	if a.DatasetID != "obs.a" || a.JobID == nil || *a.JobID != "first" {
		t.Fatalf("%+v", a)
	}
	if o.JobID != nil || len(o.Cells) != coverage.WindowSessions {
		t.Fatalf("%+v", o)
	}
	for _, c := range o.Cells {
		if c.State == "missing" && c.Repair != RepairNotRefetchable {
			t.Fatalf("orphan cell %+v", c)
		}
	}
	if o.GapDays != 0 || o.RepairAll != RepairNone || a.GapDays != coverage.WindowSessions-1 {
		t.Fatalf("%+v", o)
	}
	// from is the oldest day of the window: 30 sessions back from 2026-03-10.
	if r.from != o.Cells[0].Day {
		t.Fatalf("from %s, first cell %s", r.from, o.Cells[0].Day)
	}
}

func TestGapDaysOnRow(t *testing.T) {
	t.Parallel()
	r := &fakeReader{sessions: weekdays("XNYS"), cells: []coverage.Cell{
		{Day: "2026-03-05", Dataset: "obs.a", Keys: 2, OK: 1, Missing: 1, MissingKeys: []string{"k"}},
	}}
	jobs := []Job{{ID: "j", Kind: KindAnyDay, Calendar: "XNYS", Datasets: []string{"obs.a"}}}
	d := build(t, input(r, jobs, testForm))
	if d.Datasets[0].GapDays != coverage.WindowSessions || d.Datasets[0].RepairAll != RepairButton {
		t.Fatalf("%+v", d.Datasets[0])
	}
}

func TestFredNewest(t *testing.T) {
	t.Parallel()
	r := &fakeReader{sessions: weekdays("XNYS"),
		cells: []coverage.Cell{{Day: "2026-03-09", Dataset: fredDataset, Keys: 2, OK: 2, MissingKeys: []string{}}},
		newest: []coverage.Newest{
			{SeriesID: "DGS10", Day: "2026-03-09"},
			{SeriesID: "UNRATE", Day: "2026-02-01"},
		}}
	jobs := []Job{{ID: "fred_series", Kind: KindAnyDay, Calendar: "XNYS", Datasets: []string{fredDataset}}}
	d := build(t, input(r, jobs, nil))
	n := d.Datasets[0].Newest
	if len(n) != 2 || n[0].SeriesID != "DGS10" || n[0].AgeDays != 1 || n[1].AgeDays != 37 {
		t.Fatalf("%+v", n)
	}
}

func TestGlossaryCoversEnums(t *testing.T) {
	t.Parallel()
	d := build(t, input(&fakeReader{sessions: weekdays("XNYS")}, nil, nil))
	have := map[string]bool{}
	for _, g := range d.Glossary {
		have[g.Term] = true
	}
	want := []string{"ok", "no_value", "expired", "missing", "not_run", "none", "button", "not refetchable",
		"ledger", "session day"}
	for _, w := range want {
		if !have[w] {
			t.Errorf("no glossary row for %q", w)
		}
	}
	for _, v := range []string{string(coverage.StateOK), string(coverage.StateNoValue), string(coverage.StateExpired),
		string(coverage.StateMissing), StateNotRun, RepairNone, RepairButton,
		strings.ReplaceAll(RepairNotRefetchable, "_", " ")} {
		if !have[v] {
			t.Errorf("enum value %q has no glossary row", v)
		}
	}
}

func hasHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func TestGlossaryIsChinese(t *testing.T) {
	t.Parallel()
	d := build(t, input(&fakeReader{sessions: weekdays("XNYS")}, nil, nil))
	terms := []string{"ok", "no_value", "expired", "missing", "not_run", "none", "button", "not refetchable",
		"ledger", "session day"}
	if len(d.Glossary) != len(terms) {
		t.Fatalf("%d rows", len(d.Glossary))
	}
	for i, g := range d.Glossary {
		if g.Term != terms[i] {
			t.Errorf("row %d term %q, want %q", i, g.Term, terms[i])
		}
		if !hasHan(g.Text) {
			t.Errorf("row %q has no Chinese: %q", g.Term, g.Text)
		}
	}
	if !hasHan(d.Overwrite) {
		t.Errorf("overwrite note has no Chinese: %q", d.Overwrite)
	}
}

func TestOverwriteNote(t *testing.T) {
	t.Parallel()
	d := build(t, input(&fakeReader{sessions: weekdays("XNYS")}, nil, nil))
	if d.Overwrite != zhOverwriteNote {
		t.Fatalf("%q", d.Overwrite)
	}
}

func TestBuildLateTextValidatesSchema(t *testing.T) {
	t.Parallel()
	r := &fakeReader{sessions: weekdays("XNYS"),
		cells:  []coverage.Cell{{Day: "2026-03-10", Dataset: fredDataset, Keys: 1, Missing: 1, MissingKeys: []string{"VIXCLS"}}},
		newest: []coverage.Newest{{SeriesID: "VIXCLS", Day: "2026-03-09"}}}
	jobs := []Job{{ID: "fred_series", Kind: KindAnyDay, Calendar: "XNYS", Datasets: []string{fredDataset}}}
	d := build(t, input(r, jobs, testForm))
	cells := d.Datasets[0].Cells
	last := cells[len(cells)-1]
	if last.Day != "2026-03-10" || last.DueText != lateText || last.State != "missing" {
		t.Fatalf("%+v", last)
	}
}
