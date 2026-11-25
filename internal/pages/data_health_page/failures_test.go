package data_health_page

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/coverage"
)

type failReader struct {
	*fakeReader
	rows  []coverage.Failure
	err   error
	mu    sync.Mutex
	calls [][2]contracts.Day
}

func (f *failReader) RepeatedFailures(_ context.Context, from, to contracts.Day) ([]coverage.Failure, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, [2]contracts.Day{from, to})
	return f.rows, f.err
}

func failIn(rows []coverage.Failure) (*failReader, Input) {
	base := &fakeReader{sessions: weekdays("XNYS"), cells: []coverage.Cell{
		{Day: "2026-03-09", Dataset: "obs.a", Keys: 1, OK: 1, MissingKeys: []string{}}}}
	fr := &failReader{fakeReader: base, rows: rows}
	jobs := []Job{{ID: "job_a", Kind: KindAnyDay, Calendar: "XNYS", Datasets: []string{"obs.a"}}}
	return fr, Input{Now: testNow, Today: "2026-03-10", Cov: fr, Jobs: jobs}
}

func fail(key string, days, runs int, last contracts.Day, class string) coverage.Failure {
	kind := coverage.FailureFailed
	if class == "empty" {
		kind = coverage.FailureEmpty
	}
	return coverage.Failure{Dataset: "obs.a", Key: key, Kind: kind, Days: days, Runs: runs,
		FirstDay: "2026-03-02", LastDay: last, LastClass: class}
}

func TestRowsOrderedWithClassTexts(t *testing.T) {
	t.Parallel()
	_, in := failIn([]coverage.Failure{
		fail("b", 3, 3, "2026-03-09", "empty"),
		fail("a", 5, 7, "2026-03-09", "retryable"),
		fail("c", 3, 3, "2026-03-09", "permanent"),
		fail("d", 3, 3, "2026-03-09", "unknown_calendar"),
	})
	d := build(t, in).Datasets[0]
	var keys []string
	for _, r := range d.Repeated {
		keys = append(keys, r.Key)
		if r.ClassText == "" || r.ClassText == zhClassOther {
			t.Fatalf("class text of %s: %q", r.Key, r.ClassText)
		}
	}
	if strings.Join(keys, ",") != "a,b,c,d" || d.FailuresTotal != 4 || d.FailuresHidden != 0 || d.FailuresHiddenText != "" {
		t.Fatalf("%v %+v", keys, d)
	}
	if d.Repeated[0].Days != 5 || d.Repeated[0].Runs != 7 || d.Repeated[0].LastClass != "retryable" {
		t.Fatalf("%+v", d.Repeated[0])
	}
}

func TestOrderTiebreaks(t *testing.T) {
	t.Parallel()
	rows, _, _, _ := failureView([]coverage.Failure{
		fail("k1", 3, 3, "2026-03-05", "retryable"),
		fail("k2", 3, 4, "2026-03-01", "retryable"),
		fail("k3", 3, 3, "2026-03-09", "retryable"),
		fail("k0", 3, 3, "2026-03-09", "retryable"),
	})
	var keys []string
	for _, r := range rows {
		keys = append(keys, r.Key)
	}
	if strings.Join(keys, ",") != "k2,k0,k3,k1" {
		t.Fatalf("%v", keys)
	}
}

func TestCapAtFifty(t *testing.T) {
	t.Parallel()
	var rows []coverage.Failure
	for i := range 60 {
		rows = append(rows, fail(fmt.Sprintf("k%02d", i), 3, 3, "2026-03-09", "retryable"))
	}
	_, in := failIn(rows)
	d := build(t, in).Datasets[0]
	if len(d.Repeated) != 50 || d.FailuresTotal != 60 || d.FailuresHidden != 10 || !strings.Contains(d.FailuresHiddenText, "10") {
		t.Fatalf("%d %+v", len(d.Repeated), d)
	}
}

func TestFieldsAlwaysPresent(t *testing.T) {
	t.Parallel()
	_, in := failIn(nil)
	b, err := Build(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	validate(t, b)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"failures_note", "failures_none_text"} {
		if string(m[k]) == "" || string(m[k]) == `""` {
			t.Fatalf("%s missing", k)
		}
	}
	var ds []map[string]json.RawMessage
	if err := json.Unmarshal(m["datasets"], &ds); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"failures_total", "failures_hidden", "failures_hidden_text", "repeated_failures"} {
		if _, ok := ds[0][k]; !ok {
			t.Fatalf("%s missing", k)
		}
	}
	if string(ds[0]["repeated_failures"]) != "[]" {
		t.Fatalf("%s", ds[0]["repeated_failures"])
	}
}

func TestReaderCalledOncePerCalendarWithPinnedWindow(t *testing.T) {
	t.Parallel()
	base := &fakeReader{sessions: append(weekdays("XNYS"), weekdays("XLON")...), cells: []coverage.Cell{
		{Day: "2026-03-09", Dataset: "obs.a", Keys: 1, OK: 1, MissingKeys: []string{}},
		{Day: "2026-03-09", Dataset: "obs.b", Keys: 1, OK: 1, MissingKeys: []string{}},
		{Day: "2026-03-09", Dataset: "obs.c", Keys: 1, OK: 1, MissingKeys: []string{}},
	}}
	fr := &failReader{fakeReader: base, rows: []coverage.Failure{
		{Dataset: "obs.a", Key: "ka", Kind: coverage.FailureFailed, Days: 3, Runs: 3, FirstDay: "2026-03-02", LastDay: "2026-03-09", LastClass: "retryable"},
		{Dataset: "obs.b", Key: "kb", Kind: coverage.FailureFailed, Days: 3, Runs: 3, FirstDay: "2026-03-02", LastDay: "2026-03-09", LastClass: "retryable"},
		{Dataset: "obs.c", Key: "kc", Kind: coverage.FailureFailed, Days: 3, Runs: 3, FirstDay: "2026-03-02", LastDay: "2026-03-09", LastClass: "retryable"},
		{Dataset: "obs.nocell", Key: "kz", Kind: coverage.FailureFailed, Days: 3, Runs: 3, FirstDay: "2026-03-02", LastDay: "2026-03-09", LastClass: "retryable"},
	}}
	jobs := []Job{
		{ID: "ja", Kind: KindAnyDay, Calendar: "XNYS", Datasets: []string{"obs.a", "obs.c"}},
		{ID: "jb", Kind: KindAnyDay, Calendar: "XLON", Datasets: []string{"obs.b"}},
	}
	d := build(t, Input{Now: testNow, Today: "2026-03-10", Cov: fr, Jobs: jobs})
	if len(fr.calls) != 2 {
		t.Fatalf("calls %v", fr.calls)
	}
	// the 14th weekday counting back from Tuesday 2026-03-10 is Thursday 2026-02-19.
	for _, c := range fr.calls {
		if c[0] != "2026-02-19" || c[1] != "2026-03-10" {
			t.Fatalf("call %v", c)
		}
	}
	got := map[string][]string{}
	for _, ds := range d.Datasets {
		for _, r := range ds.Repeated {
			got[ds.DatasetID] = append(got[ds.DatasetID], r.Key)
		}
	}
	want := map[string][]string{"obs.a": {"ka"}, "obs.b": {"kb"}, "obs.c": {"kc"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%v", got)
	}
}

func TestRowsOfAnotherCalendarDropped(t *testing.T) {
	t.Parallel()
	// Both calendars are read; a row of obs.a is kept from the XNYS call only, so it is listed once.
	base := &fakeReader{sessions: append(weekdays("XNYS"), weekdays("XLON")...), cells: []coverage.Cell{
		{Day: "2026-03-09", Dataset: "obs.a", Keys: 1, OK: 1, MissingKeys: []string{}},
		{Day: "2026-03-09", Dataset: "obs.b", Keys: 1, OK: 1, MissingKeys: []string{}},
	}}
	fr := &failReader{fakeReader: base, rows: []coverage.Failure{
		{Dataset: "obs.a", Key: "ka", Kind: coverage.FailureFailed, Days: 3, Runs: 3, FirstDay: "2026-03-02", LastDay: "2026-03-09", LastClass: "permanent"},
	}}
	jobs := []Job{
		{ID: "ja", Kind: KindAnyDay, Calendar: "XNYS", Datasets: []string{"obs.a"}},
		{ID: "jb", Kind: KindAnyDay, Calendar: "XLON", Datasets: []string{"obs.b"}},
	}
	d := build(t, Input{Now: testNow, Today: "2026-03-10", Cov: fr, Jobs: jobs})
	if len(d.Datasets[0].Repeated) != 1 || d.Datasets[0].FailuresTotal != 1 || len(d.Datasets[1].Repeated) != 0 {
		t.Fatalf("%+v", d.Datasets)
	}
}

func TestGapWhenReaderLacksFailureReader(t *testing.T) {
	t.Parallel()
	in := input(&fakeReader{sessions: weekdays("XNYS"), cells: []coverage.Cell{
		{Day: "2026-03-09", Dataset: "obs.a", Keys: 1, OK: 1, MissingKeys: []string{}}}}, nil, nil)
	d := build(t, in)
	if len(d.Gaps) != 1 || d.Gaps[0].Reason != "failures_unavailable" || len(d.Datasets) != 1 || len(d.Datasets[0].Cells) == 0 {
		t.Fatalf("%+v", d)
	}
	b, _ := Build(context.Background(), in)
	if !strings.Contains(string(b), `"text":"`+zhGapText) {
		t.Fatalf("gap text missing: %s", b)
	}
}

func TestGapWhenFailureReadFails(t *testing.T) {
	t.Parallel()
	fr, in := failIn(nil)
	fr.err = errors.New("down")
	d := build(t, in)
	if len(d.Gaps) != 1 || d.Gaps[0].Reason != "failures_unavailable" || len(d.Datasets) != 1 || len(d.Datasets[0].Repeated) != 0 {
		t.Fatalf("%+v", d)
	}
}

func TestEmptyClassTextAndNoneText(t *testing.T) {
	t.Parallel()
	for _, want := range []string{"正常", "网格"} {
		if !strings.Contains(zhClassEmpty, want) {
			t.Errorf("empty class text lacks %q", want)
		}
	}
	for _, want := range []string{"expired", "没有键符合规则"} {
		if !strings.Contains(zhFailuresNone, want) {
			t.Errorf("none text lacks %q", want)
		}
	}
}

// TestJSONTagsMatchSchemaFixture: the ok fixture decodes into the view-model types without loss.
func TestJSONTagsMatchSchemaFixture(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("../../../contracts/fixtures/page.data_health_page.v1.ok.json")
	if err != nil {
		t.Fatal(err)
	}
	var vm viewModel
	if err := json.Unmarshal(b, &vm); err != nil {
		t.Fatal(err)
	}
	if vm.FailuresNote == "" || vm.FailuresNone == "" {
		t.Fatalf("top-level fields lost: %+v", vm)
	}
	var found *Dataset
	for i := range vm.Datasets {
		if len(vm.Datasets[i].Repeated) > 0 {
			found = &vm.Datasets[i]
		}
	}
	if found == nil || found.FailuresTotal != 2 || len(found.Repeated) != 2 {
		t.Fatalf("%+v", found)
	}
	r := found.Repeated[0]
	if r.Key == "" || r.Kind != "failed" || r.Days != 4 || r.Runs != 6 || r.FirstDay == "" || r.LastDay == "" ||
		r.LastClass != "retryable" || r.ClassText == "" {
		t.Fatalf("%+v", r)
	}
}
