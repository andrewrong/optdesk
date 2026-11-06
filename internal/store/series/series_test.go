package series

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// fakeDB models the two statements of the Writer: a point or a definition is added only when its key is new,
// which is what ON CONFLICT DO NOTHING does on the real database.
type fakeDB struct {
	points      map[string][]any
	definitions map[string][]any
	events      []string
	rows        [][]any // canned rows for Select
	selSQL      string
	selArgs     []any
	closed      bool
}

func newFakeDB() *fakeDB {
	return &fakeDB{points: map[string][]any{}, definitions: map[string][]any{}}
}

type fakeTx struct{ f *fakeDB }

func (t fakeTx) Exec(_ context.Context, sql string, args ...any) error {
	switch sql {
	case insertPointSQL:
		key := fmt.Sprint(args[0], args[1], args[2])
		if _, ok := t.f.points[key]; !ok {
			t.f.points[key] = args
		}
	case insertDefinitionSQL:
		key := fmt.Sprint(args[0])
		if _, ok := t.f.definitions[key]; !ok {
			t.f.definitions[key] = args
		}
	default:
		return fmt.Errorf("unexpected SQL %q", sql)
	}
	return nil
}

func (f *fakeDB) InTx(_ context.Context, fn func(execer) error) error {
	f.events = append(f.events, "begin")
	if err := fn(fakeTx{f}); err != nil {
		f.events = append(f.events, "rollback")
		return err
	}
	f.events = append(f.events, "commit")
	return nil
}

type rowScanner struct{ vals []any }

func (s rowScanner) Scan(dest ...any) error {
	for i, d := range dest {
		reflect.ValueOf(d).Elem().Set(reflect.ValueOf(s.vals[i]))
	}
	return nil
}

func (f *fakeDB) Select(_ context.Context, sql string, args []any, scan func(scanner) error) error {
	f.selSQL, f.selArgs = sql, args
	for _, r := range f.rows {
		if err := scan(rowScanner{r}); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeDB) Close() error { f.closed = true; return nil }

func date(t *testing.T, s string) types.SerializableDate {
	t.Helper()
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		t.Fatal(err)
	}
	return types.SerializableDate{Time: d}
}

func point(t *testing.T, v *float64, status string) contracts.SeriesPoint {
	t.Helper()
	return contracts.SeriesPoint{
		SeriesID: "DGS10", ObservationDate: date(t, "2026-11-04"), RealtimeStart: date(t, "2026-11-05"),
		RealtimeEnd: date(t, "9999-12-31"), Value: v, Status: status, Unit: contracts.SeriesUnit("percent"),
		PublishedAt: time.Date(2026, 11, 5, 21, 0, 0, 0, time.UTC), Source: "fred", SourceRevision: "r1",
	}
}

// Argument positions of insertPointSQL, as documented on the constant.
const (
	argValue  = 4
	argStatus = 5
)

func TestSentinelRejected(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		v    float64
	}{
		{"minus one", -1},
		{"NaN", math.NaN()},
		{"plus infinity", math.Inf(1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFakeDB()
			w := &seriesStore{db: f}
			v := tc.v
			if err := w.InsertSeriesPoints(context.Background(), []contracts.SeriesPoint{point(t, &v, "present")}); err != nil {
				t.Fatal(err)
			}
			if len(f.points) != 1 {
				t.Fatalf("stored %d points, want 1", len(f.points))
			}
			for _, args := range f.points {
				if args[argValue] != nil {
					t.Errorf("value = %v, want NULL", args[argValue])
				}
				if args[argStatus] != StatusSentinelRejected {
					t.Errorf("status = %v, want %q", args[argStatus], StatusSentinelRejected)
				}
			}
		})
	}
}

func TestRealValueAndNoValueKept(t *testing.T) {
	t.Parallel()
	f := newFakeDB()
	w := &seriesStore{db: f}
	v := 4.25
	rows := []contracts.SeriesPoint{point(t, &v, "present")}
	nv := point(t, nil, "no_value")
	nv.ObservationDate = date(t, "2026-11-03")
	rows = append(rows, nv)
	if err := w.InsertSeriesPoints(context.Background(), rows); err != nil {
		t.Fatal(err)
	}
	var sawValue, sawNull bool
	for _, args := range f.points {
		switch {
		case args[argValue] == 4.25 && args[argStatus] == "present":
			sawValue = true
		case args[argValue] == nil && args[argStatus] == "no_value":
			sawNull = true
		}
	}
	if !sawValue || !sawNull {
		t.Errorf("points = %v", f.points)
	}
}

func TestReplayIdempotent(t *testing.T) {
	t.Parallel()
	f := newFakeDB()
	w := &seriesStore{db: f}
	v := 4.25
	capture := []contracts.SeriesPoint{point(t, &v, "present")}
	if err := w.InsertSeriesPoints(context.Background(), capture); err != nil {
		t.Fatal(err)
	}
	before := len(f.points)
	if err := w.InsertSeriesPoints(context.Background(), capture); err != nil {
		t.Fatal(err)
	}
	if before != 1 || len(f.points) != before {
		t.Errorf("points after replay = %d, before = %d, want 1 and 1", len(f.points), before)
	}
}

func TestInsertOneTransactionAndEmptyInput(t *testing.T) {
	t.Parallel()
	f := newFakeDB()
	w := &seriesStore{db: f}
	if err := w.InsertSeriesPoints(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if len(f.events) != 0 {
		t.Fatalf("empty input made calls: %v", f.events)
	}
	v := 1.0
	if err := w.InsertSeriesPoints(context.Background(), []contracts.SeriesPoint{point(t, &v, "present")}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.events, []string{"begin", "commit"}) {
		t.Errorf("events = %v", f.events)
	}
	if len(f.definitions) != 1 {
		t.Errorf("definitions = %d, want 1", len(f.definitions))
	}
}

func TestSeriesPointsRange(t *testing.T) {
	t.Parallel()
	f := newFakeDB()
	v := 4.25
	obs := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	f.rows = [][]any{{"DGS10", obs, obs, obs, &v, "present", "percent", "", time.Date(2026, 11, 5, 21, 0, 0, 0, time.UTC), "fred", "r1"}}
	r := &seriesStore{db: f}
	got, err := r.SeriesPoints(context.Background(), "DGS10", "2026-11-01", "2026-11-30")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].SeriesID != "DGS10" || got[0].Value == nil || *got[0].Value != 4.25 ||
		got[0].Unit != contracts.SeriesUnit("percent") || got[0].ObservationDate.Format(time.DateOnly) != "2026-11-04" {
		t.Errorf("got %+v", got)
	}
	if f.selArgs[0] != "DGS10" {
		t.Errorf("args = %v", f.selArgs)
	}
	if _, err := r.SeriesPoints(context.Background(), "DGS10", "bad", "2026-11-30"); err == nil {
		t.Error("bad day accepted")
	} else {
		var te contracts.TypedError
		if !errors.As(err, &te) || te.Kind() != contracts.Permanent {
			t.Errorf("err = %v, want Permanent", err)
		}
	}
}

func TestOpenFuncShape(t *testing.T) {
	t.Parallel()
	var _ store.SeriesStore = &seriesStore{}
}
