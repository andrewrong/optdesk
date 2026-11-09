package ownersetpg

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
)

var _ ownerset.Store = (*Store)(nil)

type paramRow struct {
	id         int64
	key, value string
	at         time.Time
}

type exclRow struct {
	id       int64
	instr    int64
	excluded bool
	at       time.Time
}

// fakeDB is the query layer in memory. It knows the statements of the package by their text and answers each
// as the database would, over two tables whose set_at is the fake's now.
type fakeDB struct {
	now    time.Time
	nextID int64
	params []paramRow
	excl   []exclRow
	// failAfter makes the n-th Exec fail (0: never); failWith is the error.
	failAfter int
	failWith  error
	execs     int
}

func newFake() *fakeDB { return &fakeDB{now: time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)} }

func (f *fakeDB) tick() time.Time { f.now = f.now.Add(time.Minute); return f.now }

func (f *fakeDB) id() int64 { f.nextID++; return f.nextID }

type argScanner []any

func (a argScanner) Scan(dest ...any) error {
	for i, d := range dest {
		reflect.ValueOf(d).Elem().Set(reflect.ValueOf(a[i]))
	}
	return nil
}

func (f *fakeDB) Select(_ context.Context, sql string, args []any, scan func(scanner) error) error {
	var rows []argScanner
	switch sql {
	case paramsSQL:
		rows = f.paramRows(args[0].(time.Time), true)
	case currentParamsSQL:
		rows = f.paramRows(time.Unix(1<<40, 0), false)
	case exclusionsSQL:
		rows = f.exclRows(args[0].(time.Time), true)
	case currentExclusionsSQL:
		rows = f.exclRows(time.Unix(1<<40, 0), false)
	case historySQL:
		rows = f.historyRows(args[0].(int))
	case versionSQL:
		rows = []argScanner{{f.version(args[0].(time.Time))}}
	default:
		return errors.New("unknown query " + sql)
	}
	for _, r := range rows {
		if err := scan(r); err != nil {
			return err
		}
	}
	return nil
}

// paramRows is the newest row per key not after at: with its set_at, or the key and value only.
func (f *fakeDB) paramRows(at time.Time, withAt bool) []argScanner {
	best := map[string]paramRow{}
	for _, r := range f.params {
		b, ok := best[r.key]
		if !r.at.After(at) && (!ok || r.at.After(b.at) || (r.at.Equal(b.at) && r.id > b.id)) {
			best[r.key] = r
		}
	}
	var rows []argScanner
	for _, r := range best {
		if withAt {
			rows = append(rows, argScanner{r.key, r.value, r.at})
		} else {
			rows = append(rows, argScanner{r.key, r.value})
		}
	}
	return rows
}

func (f *fakeDB) exclRows(at time.Time, withAt bool) []argScanner {
	best := map[int64]exclRow{}
	for _, r := range f.excl {
		b, ok := best[r.instr]
		if !r.at.After(at) && (!ok || r.at.After(b.at) || (r.at.Equal(b.at) && r.id > b.id)) {
			best[r.instr] = r
		}
	}
	var rows []argScanner
	for _, r := range best {
		if withAt {
			rows = append(rows, argScanner{r.instr, r.excluded, r.at})
		} else {
			rows = append(rows, argScanner{r.instr, r.excluded})
		}
	}
	return rows
}

func (f *fakeDB) historyRows(n int) []argScanner {
	type h struct {
		row argScanner
		at  time.Time
		id  int64
	}
	var all []h
	for _, r := range f.params {
		all = append(all, h{argScanner{"param", r.key, int64(0), r.value, false, r.at}, r.at, r.id})
	}
	for _, r := range f.excl {
		all = append(all, h{argScanner{"exclusion", "", r.instr, "", r.excluded, r.at}, r.at, r.id})
	}
	sort.SliceStable(all, func(i, j int) bool {
		if !all[i].at.Equal(all[j].at) {
			return all[i].at.After(all[j].at)
		}
		return all[i].id > all[j].id
	})
	var rows []argScanner
	for i, x := range all {
		if i < n {
			rows = append(rows, x.row)
		}
	}
	return rows
}

func (f *fakeDB) version(at time.Time) int64 {
	var v int64
	for _, r := range f.params {
		if !r.at.After(at) && r.id > v {
			v = r.id
		}
	}
	for _, r := range f.excl {
		if !r.at.After(at) && r.id > v {
			v = r.id
		}
	}
	return v
}

func (f *fakeDB) Exec(_ context.Context, sql string, args ...any) error {
	f.execs++
	if f.failAfter > 0 && f.execs == f.failAfter {
		return f.failWith
	}
	switch sql {
	case insertParamSQL:
		f.params = append(f.params, paramRow{f.id(), args[0].(string), args[1].(string), f.now})
	case insertExclusionSQL:
		f.excl = append(f.excl, exclRow{f.id(), args[0].(int64), args[1].(bool), f.now})
	default:
		return errors.New("unknown statement " + sql)
	}
	return nil
}

// InTx runs fn and, when it fails, puts the tables back as they were: a rolled-back transaction.
func (f *fakeDB) InTx(_ context.Context, fn func(querier) error) error {
	params, excl, next := append([]paramRow(nil), f.params...), append([]exclRow(nil), f.excl...), f.nextID
	if err := fn(f); err != nil {
		f.params, f.excl, f.nextID = params, excl, next
		return err
	}
	return nil
}

func (f *fakeDB) Close() error { return nil }

func apply(t *testing.T, s *Store, c ownerset.Change) int {
	t.Helper()
	n, err := s.Apply(context.Background(), c)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	return n
}

func TestParamsAsOf(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := &Store{db: f}
	t0 := f.tick()
	apply(t, s, ownerset.Change{Params: []ownerset.ParamValue{{Key: "min_dte", Value: "21"}, {Key: "max_dte", Value: "45"}}})
	t1 := f.tick()
	apply(t, s, ownerset.Change{Params: []ownerset.ParamValue{{Key: "min_dte", Value: "30"}}})
	ctx := context.Background()

	got, err := s.Params(ctx, t1)
	want := []ownerset.Param{{Key: "max_dte", Value: "45", SetAt: t0}, {Key: "min_dte", Value: "30", SetAt: t1}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("as of t1 = %v, %v; want %v", got, err, want)
	}
	got, _ = s.Params(ctx, t1.Add(-time.Second))
	if len(got) != 2 || got[1].Value != "21" {
		t.Errorf("as of before t1 = %v; min_dte must be 21", got)
	}
	got, err = s.Params(ctx, t0.Add(-time.Hour))
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("before every row = %#v, %v; want an empty non-nil slice", got, err)
	}
	// A later row at the very same instant wins by id.
	f.now = t1
	apply(t, s, ownerset.Change{Params: []ownerset.ParamValue{{Key: "min_dte", Value: "28"}}})
	got, _ = s.Params(ctx, t1)
	if got[1].Value != "28" {
		t.Errorf("tie at t1 = %v; the larger id wins", got[1])
	}
}

func TestExclusionsAsOf(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := &Store{db: f}
	t0 := f.tick()
	apply(t, s, ownerset.Change{Exclusions: []ownerset.ExclusionValue{{InstrumentID: 9, Excluded: true}, {InstrumentID: 3, Excluded: true}}})
	t1 := f.tick()
	apply(t, s, ownerset.Change{Exclusions: []ownerset.ExclusionValue{{InstrumentID: 9, Excluded: false}}})
	ctx := context.Background()

	got, err := s.Exclusions(ctx, t1)
	want := []ownerset.Exclusion{{InstrumentID: 3, Excluded: true, SetAt: t0}, {InstrumentID: 9, Excluded: false, SetAt: t1}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("as of t1 = %v, %v; want %v", got, err, want)
	}
	got, _ = s.Exclusions(ctx, t0)
	if len(got) != 2 || !got[1].Excluded {
		t.Errorf("as of t0 = %v; instrument 9 is still excluded", got)
	}
	got, err = s.Exclusions(ctx, t0.Add(-time.Hour))
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("before every row = %#v, %v", got, err)
	}
}

func TestApplyWritesOnlyDifferences(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := &Store{db: f}
	f.tick()
	apply(t, s, ownerset.Change{
		Params:     []ownerset.ParamValue{{Key: "min_dte", Value: "21"}, {Key: "max_dte", Value: "45"}},
		Exclusions: []ownerset.ExclusionValue{{InstrumentID: 5, Excluded: true}},
	})
	f.tick()
	n := apply(t, s, ownerset.Change{
		Params:     []ownerset.ParamValue{{Key: "min_dte", Value: "21"}, {Key: "max_dte", Value: "50"}},
		Exclusions: []ownerset.ExclusionValue{{InstrumentID: 5, Excluded: true}, {InstrumentID: 6, Excluded: true}},
	})
	if n != 2 || len(f.params) != 3 || len(f.excl) != 2 {
		t.Errorf("inserted %d; params %d, exclusions %d; want 2, 3, 2", n, len(f.params), len(f.excl))
	}
	if last := f.params[2]; last.key != "max_dte" || last.value != "50" || !last.at.Equal(f.now) {
		t.Errorf("new param row = %+v; want max_dte 50 at the database's now", last)
	}
}

func TestApplyUnchangedWritesNothing(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := &Store{db: f}
	c := ownerset.Change{
		Params:     []ownerset.ParamValue{{Key: "min_dte", Value: "21"}},
		Exclusions: []ownerset.ExclusionValue{{InstrumentID: 5, Excluded: true}},
	}
	if n := apply(t, s, c); n != 2 {
		t.Fatalf("first apply = %d, want 2", n)
	}
	execs := f.execs
	if n := apply(t, s, c); n != 0 || f.execs != execs {
		t.Errorf("second apply = %d with %d more statements; want 0 and none", n, f.execs-execs)
	}
	if n := apply(t, s, ownerset.Change{}); n != 0 {
		t.Errorf("empty change = %d, want 0", n)
	}
	// Not excluded and no row is the state of an instrument never touched: nothing to write.
	if n := apply(t, s, ownerset.Change{Exclusions: []ownerset.ExclusionValue{{InstrumentID: 77, Excluded: false}}}); n != 0 {
		t.Errorf("not-excluded with no row = %d, want 0", n)
	}
}

func TestApplyIsOneTransaction(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := &Store{db: f}
	apply(t, s, ownerset.Change{Params: []ownerset.ParamValue{{Key: "min_dte", Value: "21"}}})
	before := len(f.params)

	f.failAfter, f.failWith = f.execs+2, errors.New("connection lost")
	_, err := s.Apply(context.Background(), ownerset.Change{
		Params:     []ownerset.ParamValue{{Key: "min_dte", Value: "30"}, {Key: "max_dte", Value: "60"}},
		Exclusions: []ownerset.ExclusionValue{{InstrumentID: 1, Excluded: true}},
	})
	if err == nil {
		t.Fatal("apply succeeded although an insert failed")
	}
	if len(f.params) != before || len(f.excl) != 0 {
		t.Errorf("rows after a failed apply: params %d (was %d), exclusions %d; want none kept", len(f.params), before, len(f.excl))
	}
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Retryable {
		t.Errorf("error = %v; want Retryable", err)
	}

	// No grant is a Permanent error: the database says it again every time.
	f.failAfter, f.failWith = f.execs+1, &pgconn.PgError{Code: "42501", Message: "permission denied"}
	_, err = s.Apply(context.Background(), ownerset.Change{Params: []ownerset.ParamValue{{Key: "max_dte", Value: "60"}}})
	if !errors.As(err, &te) || te.Kind() != contracts.Permanent {
		t.Errorf("error = %v; want Permanent", err)
	}
}

func TestEmptyValueIsDefaultRow(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := &Store{db: f}
	// No row and an empty value are the same state: nothing is written.
	if n := apply(t, s, ownerset.Change{Params: []ownerset.ParamValue{{Key: "min_dte", Value: ""}}}); n != 0 {
		t.Errorf("empty value with no row = %d, want 0", n)
	}
	f.tick()
	apply(t, s, ownerset.Change{Params: []ownerset.ParamValue{{Key: "min_dte", Value: "30"}}})
	at := f.tick()
	// A reset over a stored value is one more row, empty; nothing is updated or deleted.
	if n := apply(t, s, ownerset.Change{Params: []ownerset.ParamValue{{Key: "min_dte", Value: ""}}}); n != 1 {
		t.Fatalf("reset = %d, want 1", n)
	}
	got, err := s.Params(context.Background(), at)
	if err != nil || len(got) != 1 || got[0].Value != "" || got[0].Key != "min_dte" {
		t.Errorf("params = %v, %v; want min_dte with an empty value", got, err)
	}
	if len(f.params) != 2 {
		t.Errorf("%d rows, want 2: the old row stays", len(f.params))
	}
}

func TestHistoryNewestFirst(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := &Store{db: f}
	f.tick()
	apply(t, s, ownerset.Change{Params: []ownerset.ParamValue{{Key: "min_dte", Value: "21"}}})
	t1 := f.tick()
	apply(t, s, ownerset.Change{Exclusions: []ownerset.ExclusionValue{{InstrumentID: 4, Excluded: true}}})
	t2 := f.tick()
	apply(t, s, ownerset.Change{Params: []ownerset.ParamValue{{Key: "min_dte", Value: "30"}}})

	got, err := s.History(context.Background(), 2)
	want := []ownerset.HistoryRow{
		{Kind: "param", Key: "min_dte", Value: "30", SetAt: t2},
		{Kind: "exclusion", InstrumentID: 4, Excluded: true, SetAt: t1},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("history = %+v, %v\nwant      %+v", got, err, want)
	}
	if got, err := s.History(context.Background(), 10); err != nil || len(got) != 3 {
		t.Errorf("history of 10 = %d rows, %v; want 3", len(got), err)
	}
	if got, err := s.History(context.Background(), 0); err != nil || got == nil || len(got) != 0 {
		t.Errorf("history of 0 = %#v, %v; want an empty non-nil slice", got, err)
	}
}

func TestVersionIsNewestId(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := &Store{db: f}
	ctx := context.Background()
	if v, err := s.Version(ctx, f.now); err != nil || v != 0 {
		t.Errorf("empty version = %d, %v; want 0", v, err)
	}
	t0 := f.tick()
	apply(t, s, ownerset.Change{Params: []ownerset.ParamValue{{Key: "min_dte", Value: "21"}}})
	t1 := f.tick()
	apply(t, s, ownerset.Change{Exclusions: []ownerset.ExclusionValue{{InstrumentID: 4, Excluded: true}}})
	if v, err := s.Version(ctx, t1); err != nil || v != 2 {
		t.Errorf("version at t1 = %d, %v; want 2", v, err)
	}
	if v, err := s.Version(ctx, t0); err != nil || v != 1 {
		t.Errorf("version at t0 = %d, %v; want 1", v, err)
	}
	if v, _ := s.Version(ctx, t0.Add(-time.Second)); v != 0 {
		t.Errorf("version before every row = %d, want 0", v)
	}
}
