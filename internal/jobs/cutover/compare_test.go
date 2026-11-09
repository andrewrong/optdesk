package cutover_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/cutover"
)

// fakeReader serves invented readings and counts the calls.
type fakeReader struct {
	mu    sync.Mutex
	rows  map[string][]contracts.ReadingRecord // key name|kind|subject
	calls []string
	from  contracts.Day
	to    contracts.Day
}

func (f *fakeReader) LatestReadings(_ context.Context, name, kind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, name+"|"+kind+"|"+subject)
	f.from, f.to = from, to
	return f.rows[name+"|"+kind+"|"+subject], nil
}

func reading(name, day, value string) contracts.ReadingRecord {
	d, err := time.Parse("2006-01-02", day)
	if err != nil {
		panic(err)
	}
	return contracts.ReadingRecord{
		Name: name, Version: 1, SubjectKind: "market", Subject: "X",
		Day: types.SerializableDate{Time: d}, Value: &value,
	}
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func run(t *testing.T, old, reg, classes, expl string, rd *fakeReader) (jobs.FigureReport, error) {
	t.Helper()
	return cutover.CompareFigures(context.Background(), strings.NewReader(old), strings.NewReader(reg),
		strings.NewReader(classes), strings.NewReader(expl), rd)
}

func oldLine(name, day, value string, places string) string {
	return `{"name":"` + name + `","version":1,"subject_kind":"market","subject":"X","day":"` + day +
		`","value":` + value + `,"places":` + places + `,"from":"page"}` + "\n"
}

func statusOf(t *testing.T, rep jobs.FigureReport, id string) jobs.FigureResult {
	t.Helper()
	for _, r := range rep.Results {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("no result for %s", id)
	return jobs.FigureResult{}
}

func TestCompareReadsEachSubjectOnce(t *testing.T) {
	t.Parallel()
	old := oldLine("demo.alpha", "2026-11-02", `"1.0"`, "1") + oldLine("demo.alpha", "2026-11-05", `"1.0"`, "1") +
		oldLine("demo.alpha", "2026-11-03", `"1.0"`, "1") + oldLine("demo.beta", "2026-11-03", `"1.0"`, "1")
	rd := &fakeReader{}
	if _, err := run(t, old, fixture(t, "register.toml"), fixture(t, "classes.toml"), fixture(t, "explained.toml"), rd); err != nil {
		t.Fatal(err)
	}
	if len(rd.calls) != 2 {
		t.Fatalf("calls = %v, want one per distinct subject", rd.calls)
	}
	if rd.from != "2026-11-02" || rd.to != "2026-11-05" {
		t.Fatalf("range = %s..%s", rd.from, rd.to)
	}
}

func TestCompareRoundsToPlaces(t *testing.T) {
	t.Parallel()
	cases := []struct {
		old, places, new string
		want             jobs.FigureStatus
	}{
		{`"1.25"`, "1", "1.2", jobs.FigureChecked},       // tie to even, down
		{`"1.35"`, "1", "1.4", jobs.FigureChecked},       // tie to even, up
		{`"-0.04"`, "1", "0.0", jobs.FigureChecked},      // negative zero is zero
		{`"2.00"`, "1", "2.04999", jobs.FigureChecked},   // the new side is rounded too
		{`"2.0"`, "1", "2.05", jobs.FigureChecked},       // 2.05 ties to 2.0
		{`"2.0"`, "1", "2.0501", jobs.FigureUnexplained}, // 2.0501 rounds to 2.1
		{`"100"`, "0", "100.5", jobs.FigureChecked},      // 100.5 ties to 100
		{`"101"`, "0", "100.5", jobs.FigureUnexplained},
	}
	for _, c := range cases {
		rd := &fakeReader{rows: map[string][]contracts.ReadingRecord{
			"demo.alpha|market|X": {reading("demo.alpha", "2026-11-02", c.new)},
		}}
		rep, err := run(t, oldLine("demo.alpha", "2026-11-02", c.old, c.places), fixture(t, "register.toml"),
			fixture(t, "classes.toml"), "", rd)
		if err != nil {
			t.Fatal(err)
		}
		if got := statusOf(t, rep, "demo.alpha").Status; got != c.want {
			t.Errorf("old %s new %s places %s: %s, want %s", c.old, c.new, c.places, got, c.want)
		}
	}
}

func TestOKOnlyWithoutUnexplained(t *testing.T) {
	t.Parallel()
	good := &fakeReader{rows: map[string][]contracts.ReadingRecord{"demo.alpha|market|X": {reading("demo.alpha", "2026-11-02", "1.0")}}}
	rep, err := run(t, oldLine("demo.alpha", "2026-11-02", `"1.0"`, "1"), fixture(t, "register.toml"), fixture(t, "classes.toml"), "", good)
	if err != nil || !rep.OK {
		t.Fatalf("OK = %v, err %v", rep.OK, err)
	}
	bad := &fakeReader{rows: map[string][]contracts.ReadingRecord{"demo.alpha|market|X": {reading("demo.alpha", "2026-11-02", "9.0")}}}
	rep, err = run(t, oldLine("demo.alpha", "2026-11-02", `"1.0"`, "1"), fixture(t, "register.toml"), fixture(t, "classes.toml"), "", bad)
	if err != nil || rep.OK {
		t.Fatalf("OK = %v with an unexplained figure, err %v", rep.OK, err)
	}
}

func TestUncheckedReasons(t *testing.T) {
	t.Parallel()
	reg, cls := fixture(t, "register.toml"), fixture(t, "classes.toml")
	cases := []struct {
		name, old, reason string
		rd                *fakeReader
	}{
		{"no line", "", "no_old_value", &fakeReader{}},
		{"null value", oldLine("demo.alpha", "2026-11-02", "null", "1"), "no_old_value", &fakeReader{}},
		{"path not found", `{"name":"demo.alpha","version":1,"subject_kind":"market","subject":"X",` +
			`"day":"2026-11-02","value":null,"places":1,"reason":"path_not_found"}` + "\n",
			"path_not_found", &fakeReader{}},
		{"no reading", oldLine("demo.alpha", "2026-11-02", `"1.0"`, "1"), "not_computed_yet", &fakeReader{}},
	}
	for _, c := range cases {
		rep, err := run(t, c.old, reg, cls, "", c.rd)
		if err != nil {
			t.Fatal(err)
		}
		got := statusOf(t, rep, "demo.alpha")
		if got.Status != jobs.FigureUnchecked || got.Reason != c.reason {
			t.Errorf("%s: %+v, want unchecked %s", c.name, got, c.reason)
		}
	}
}

func TestFiveStatuses(t *testing.T) {
	t.Parallel()
	rd := &fakeReader{rows: map[string][]contracts.ReadingRecord{
		"demo.alpha|market|X": {reading("demo.alpha", "2026-11-02", "1.2"), reading("demo.alpha", "2026-11-03", "2.0")},
		"demo.beta|market|X":  {reading("demo.beta", "2026-11-02", "6.0")},
		"demo.gamma|market|X": {reading("demo.gamma", "2026-11-02", "8")},
	}}
	rep, err := run(t, fixture(t, "old.jsonl"), fixture(t, "register_five.toml"), fixture(t, "classes.toml"), fixture(t, "explained.toml"), rd)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]jobs.FigureStatus{
		"demo.alpha": jobs.FigureChecked, "demo.beta": jobs.FigureExplained, "demo.gamma": jobs.FigureUnexplained,
		"demo.fresh": jobs.FigureNew, "demo.delta": jobs.FigureUnchecked,
	}
	if len(rep.Results) != 5 {
		t.Fatalf("results = %+v", rep.Results)
	}
	for id, st := range want {
		if got := statusOf(t, rep, id).Status; got != st {
			t.Errorf("%s = %s, want %s", id, got, st)
		}
	}
	if rep.OK {
		t.Error("OK with an unexplained figure")
	}
	if r := statusOf(t, rep, "demo.delta"); r.Reason != "path_not_found" {
		t.Errorf("delta reason = %q", r.Reason)
	}
}
