package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

type memSeries struct{ rows []contracts.SeriesPoint }

func (m *memSeries) InsertSeriesPoints(_ context.Context, rows []contracts.SeriesPoint) error {
	m.rows = append(m.rows, rows...)
	return nil
}
func (m *memSeries) SeriesPoints(_ context.Context, id string, _, _ contracts.Day) ([]contracts.SeriesPoint, error) {
	var out []contracts.SeriesPoint
	for _, r := range m.rows {
		if r.SeriesID == id {
			out = append(out, r)
		}
	}
	return out, nil
}
func (m *memSeries) Close() error { return nil }

type memDays struct{ rows []contracts.SessionDay }

func (m *memDays) UpsertSessionDays(_ context.Context, rows []contracts.SessionDay) error {
	m.rows = append(m.rows, rows...)
	return nil
}
func (m *memDays) SessionDays(context.Context, string, contracts.Day, contracts.Day) ([]contracts.SessionDay, error) {
	return m.rows, nil
}
func (m *memDays) Close() error { return nil }

type memLedger struct {
	present map[contracts.ExpectedKey]store.PresentStatus
	cursor  store.Cursor
	lease   *store.Lease
	windows map[winKey]store.Window
	runs    []store.Run
}

type winKey struct {
	id  string
	day contracts.Day
}

type noValueErr struct{}

func (noValueErr) Error() string             { return "no window" }
func (noValueErr) Kind() contracts.ErrorKind { return contracts.NoValue }

func (l *memLedger) PutExpected(context.Context, contracts.Day, string, []contracts.ExpectedKey) error {
	return nil
}
func (l *memLedger) Expected(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return nil, "", nil
}
func (l *memLedger) MarkPresent(_ context.Context, _ contracts.Day, k contracts.ExpectedKey, s store.PresentStatus) error {
	l.present[k] = s
	return nil
}
func (l *memLedger) Present(context.Context, contracts.Day) (map[contracts.ExpectedKey]store.PresentStatus, error) {
	return l.present, nil
}
func (l *memLedger) PutWindow(_ context.Context, w store.Window) error {
	if l.windows == nil {
		l.windows = map[winKey]store.Window{}
	}
	l.windows[winKey{w.DatasetID, w.Day}] = w
	return nil
}
func (l *memLedger) Window(_ context.Context, id string, day contracts.Day) (store.Window, error) {
	w, ok := l.windows[winKey{id, day}]
	if !ok {
		return store.Window{}, noValueErr{}
	}
	return w, nil
}
func (l *memLedger) RecordRun(_ context.Context, r store.Run) error {
	for i := range l.runs {
		if l.runs[i].ID == r.ID {
			l.runs[i] = r
			return nil
		}
	}
	l.runs = append(l.runs, r)
	return nil
}
func (l *memLedger) Runs(_ context.Context, day contracts.Day) ([]store.Run, error) {
	var out []store.Run
	for _, r := range l.runs {
		if r.Day == day {
			out = append(out, r)
		}
	}
	return out, nil
}
func (l *memLedger) SetCursor(_ context.Context, c store.Cursor) error {
	l.cursor = c
	return nil
}
func (l *memLedger) Cursor(context.Context) (store.Cursor, error) { return l.cursor, nil }
func (l *memLedger) Acquire(_ context.Context, owner string, ttl time.Duration) (store.Lease, error) {
	v := store.Lease{Owner: owner, ExpiresAt: time.Unix(0, 0).Add(ttl)}
	l.lease = &v
	return v, nil
}
func (l *memLedger) Release(context.Context, store.Lease) error { l.lease = nil; return nil }
func (l *memLedger) Close() error                               { return nil }

var (
	_ store.SeriesStore        = (*memSeries)(nil)
	_ store.SessionDayStore    = (*memDays)(nil)
	_ store.OpsLedger          = (*memLedger)(nil)
	_ store.OpenSeriesFunc     = func(context.Context, string) (store.SeriesStore, error) { return &memSeries{}, nil }
	_ store.OpenLedgerFunc     = func(context.Context, string) (store.OpsLedger, error) { return nil, nil }
	_ store.SetupFunc          = func(context.Context, store.SetupConfig) error { return nil }
	_ store.OpenSessionDayFunc = func(context.Context, string) (store.SessionDayStore, error) { return &memDays{}, nil }
)

// roundTrip uses the writer and the reader through their interfaces only.
func roundTrip(ctx context.Context, w store.SeriesWriter, r store.SeriesReader) ([]contracts.SeriesPoint, error) {
	if err := w.InsertSeriesPoints(ctx, []contracts.SeriesPoint{{SeriesID: "DGS10"}, {SeriesID: "OTHER"}}); err != nil {
		return nil, err
	}
	return r.SeriesPoints(ctx, "DGS10", "2026-11-01", "2026-11-30")
}

func TestSeriesContract(t *testing.T) {
	t.Parallel()
	s := &memSeries{}
	got, err := roundTrip(context.Background(), s, s)
	if err != nil || len(got) != 1 || got[0].SeriesID != "DGS10" {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestLedgerContract(t *testing.T) {
	t.Parallel()
	var l store.OpsLedger = &memLedger{present: map[contracts.ExpectedKey]store.PresentStatus{}}
	ctx := context.Background()
	k := contracts.ExpectedKey{DatasetID: "d", SubjectKey: "s"}
	if err := l.MarkPresent(ctx, "2026-11-04", k, store.StatusNoValue); err != nil {
		t.Fatal(err)
	}
	p, _ := l.Present(ctx, "2026-11-04")
	if p[k] != store.StatusNoValue {
		t.Fatalf("present = %v", p)
	}
	_ = l.SetCursor(ctx, store.Cursor{Day: "2026-11-04"})
	if c, _ := l.Cursor(ctx); c.Day != "2026-11-04" {
		t.Fatalf("cursor = %v", c)
	}
}

func TestLedgerWindowKey(t *testing.T) {
	t.Parallel()
	var l store.OpsLedger = &memLedger{}
	ctx := context.Background()
	at := func(h int) time.Time { return time.Date(2026, 11, 4, h, 0, 0, 0, time.UTC) }
	ws := []store.Window{
		{DatasetID: "a", Day: "2026-11-04", OpenAt: at(9), CloseAt: at(16)},
		{DatasetID: "b", Day: "2026-11-04", OpenAt: at(10), CloseAt: at(17)},
		{DatasetID: "a", Day: "2026-11-05", OpenAt: at(11), CloseAt: at(18)},
	}
	for _, w := range ws {
		if err := l.PutWindow(ctx, w); err != nil {
			t.Fatal(err)
		}
	}
	for _, w := range ws {
		got, err := l.Window(ctx, w.DatasetID, w.Day)
		if err != nil || got != w {
			t.Fatalf("Window(%s, %s) = %+v, %v; want %+v", w.DatasetID, w.Day, got, err, w)
		}
	}
	_, err := l.Window(ctx, "c", "2026-11-04")
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.NoValue {
		t.Fatalf("missing window: err = %v, want TypedError of kind NoValue", err)
	}
}

func TestLedgerRuns(t *testing.T) {
	t.Parallel()
	var l store.OpsLedger = &memLedger{}
	ctx := context.Background()
	k := contracts.ExpectedKey{DatasetID: "d", SubjectKey: "s"}
	day := contracts.Day("2026-11-04")
	r := store.Run{ID: "r1", Day: day, Key: k, Status: store.RunRunning}
	if err := l.RecordRun(ctx, r); err != nil {
		t.Fatal(err)
	}
	r.Status, r.ReturnedN, r.UsableN = store.RunDone, 3, 2
	if err := l.RecordRun(ctx, r); err != nil {
		t.Fatal(err)
	}
	got, _ := l.Runs(ctx, day)
	if len(got) != 1 || got[0] != r {
		t.Fatalf("same ID: runs = %+v, want only %+v", got, r)
	}
	r2 := r
	r2.ID = "r2"
	if err := l.RecordRun(ctx, r2); err != nil {
		t.Fatal(err)
	}
	if got, _ := l.Runs(ctx, day); len(got) != 2 {
		t.Fatalf("two IDs: runs = %+v, want 2", got)
	}
}

func TestDSNEnvNames(t *testing.T) {
	t.Parallel()
	roles := []string{store.DSNEnvRunner, store.DSNEnvSeriesFred, store.DSNEnvPages}
	seen := map[string]bool{}
	for _, n := range roles {
		if !strings.HasPrefix(n, "OT_PG_DSN_") {
			t.Errorf("%q does not start with OT_PG_DSN_", n)
		}
		if seen[n] {
			t.Errorf("%q is declared twice", n)
		}
		seen[n] = true
	}
	if seen[store.AdminDSNEnv] {
		t.Errorf("AdminDSNEnv %q equals a role DSN name", store.AdminDSNEnv)
	}
}

// TestEnvContract checks the fake side of ReadEnvFunc and a caller that reads DSNEnvLoader from it.
func TestEnvContract(t *testing.T) {
	t.Parallel()
	var read store.ReadEnvFunc = func(path string) (map[string]string, error) {
		return map[string]string{store.DSNEnvLoader: "postgres://ot_loader@h/d", "PATH_SEEN": path}, nil
	}
	m, err := read("/x/.env")
	if err != nil {
		t.Fatal(err)
	}
	if store.DSNEnvLoader != "OT_PG_DSN_LOADER" {
		t.Fatalf("line name %q", store.DSNEnvLoader)
	}
	if got := m[store.DSNEnvLoader]; got != "postgres://ot_loader@h/d" {
		t.Fatalf("loader dsn %q", got)
	}
}

type memOptionEod struct{ rows []contracts.OptionEod }

func (m *memOptionEod) InsertOptionEod(_ context.Context, rows []contracts.OptionEod) error {
	for _, r := range rows {
		dup := false
		for _, e := range m.rows {
			if e.SessionDay.Equal(r.SessionDay.Time) && e.CaptureID == r.CaptureID && e.InstrumentID == r.InstrumentID {
				dup = true
			}
		}
		if !dup {
			m.rows = append(m.rows, r)
		}
	}
	return nil
}

func (m *memOptionEod) OptionEod(_ context.Context, id int64, from, to contracts.Day) ([]contracts.OptionEod, error) {
	var out []contracts.OptionEod
	for _, r := range m.rows {
		d := contracts.Day(r.SessionDay.Format(time.DateOnly))
		if int64(r.UnderlyingID) == id && d >= from && d <= to {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if !a.SessionDay.Equal(b.SessionDay.Time) {
			return a.SessionDay.Format(time.DateOnly) < b.SessionDay.Format(time.DateOnly)
		}
		if a.InstrumentID != b.InstrumentID {
			return a.InstrumentID < b.InstrumentID
		}
		return a.CaptureID < b.CaptureID
	})
	return out, nil
}
func (m *memOptionEod) Close() error { return nil }

type memQuoteSnapshot struct{ rows []contracts.QuoteSnapshot }

func (m *memQuoteSnapshot) InsertQuoteSnapshots(_ context.Context, rows []contracts.QuoteSnapshot) error {
	for _, r := range rows {
		dup := false
		for _, e := range m.rows {
			if e.SessionDay.Equal(r.SessionDay.Time) && e.CaptureID == r.CaptureID && e.Slot == r.Slot && e.InstrumentID == r.InstrumentID {
				dup = true
			}
		}
		if !dup {
			m.rows = append(m.rows, r)
		}
	}
	return nil
}

func (m *memQuoteSnapshot) QuoteSnapshots(_ context.Context, id int64, from, to contracts.Day) ([]contracts.QuoteSnapshot, error) {
	var out []contracts.QuoteSnapshot
	for _, r := range m.rows {
		d := contracts.Day(r.SessionDay.Format(time.DateOnly))
		if int64(r.InstrumentID) == id && d >= from && d <= to {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if !a.SessionDay.Equal(b.SessionDay.Time) {
			return a.SessionDay.Format(time.DateOnly) < b.SessionDay.Format(time.DateOnly)
		}
		if a.Slot != b.Slot {
			return a.Slot < b.Slot
		}
		return a.CaptureID < b.CaptureID
	})
	return out, nil
}
func (m *memQuoteSnapshot) Close() error { return nil }

var (
	_ store.OptionEodStore        = &memOptionEod{}
	_ store.QuoteSnapshotStore    = &memQuoteSnapshot{}
	_ store.OpenOptionEodFunc     = func(context.Context, string) (store.OptionEodStore, error) { return &memOptionEod{}, nil }
	_ store.OpenQuoteSnapshotFunc = func(context.Context, string) (store.QuoteSnapshotStore, error) { return &memQuoteSnapshot{}, nil }
)

func loadFixture[T any](t *testing.T, name string) []T {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "contracts", "fixtures", name))
	if err != nil {
		t.Fatal(err)
	}
	var recs []T
	if err := json.Unmarshal(b, &recs); err != nil {
		t.Fatal(err)
	}
	return recs
}

// storeOptionEod and readOptionEod use the stores through their interfaces only.
func storeAndRead(ctx context.Context, s store.OptionEodStore, rows []contracts.OptionEod, id int64, from, to contracts.Day) ([]contracts.OptionEod, error) {
	if err := s.InsertOptionEod(ctx, rows); err != nil {
		return nil, err
	}
	return s.OptionEod(ctx, id, from, to)
}

// requireEodOrder checks the documented order: session_day, instrument_id, capture_id.
func requireEodOrder(t *testing.T, all []contracts.OptionEod) {
	t.Helper()
	for i := 1; i < len(all); i++ {
		a, b := all[i-1], all[i]
		less := a.SessionDay.Before(b.SessionDay.Time) ||
			(a.SessionDay.Equal(b.SessionDay.Time) && (a.InstrumentID < b.InstrumentID ||
				(a.InstrumentID == b.InstrumentID && a.CaptureID <= b.CaptureID)))
		if !less {
			t.Fatalf("row %d is out of order: %+v after %+v", i, b, a)
		}
	}
}

func TestOptionEodStoreContract(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rows := loadFixture[contracts.OptionEod](t, "obs.option_eod.r1.json")
	if len(rows) < 4 {
		t.Fatalf("fixture has %d rows", len(rows))
	}
	s := &memOptionEod{}
	underlying := int64(rows[0].UnderlyingID)

	got, err := storeAndRead(ctx, s, rows, underlying, "2026-11-06", "2026-11-09")
	if err != nil || len(got) != len(rows) {
		t.Fatalf("first insert: got %d rows err %v, want %d", len(got), err, len(rows))
	}
	// a repeat of the same keys adds no row
	got, err = storeAndRead(ctx, s, rows, underlying, "2026-11-06", "2026-11-09")
	if err != nil || len(got) != len(rows) {
		t.Fatalf("repeat: got %d rows err %v, want %d", len(got), err, len(rows))
	}
	// a second capture with a new capture_id adds one
	again := rows[0]
	again.CaptureID = "00000000-0000-4000-8000-0000000000ff"
	got, err = storeAndRead(ctx, s, []contracts.OptionEod{again}, underlying, "2026-11-06", "2026-11-09")
	if err != nil || len(got) != len(rows)+1 {
		t.Fatalf("new capture: got %d rows err %v, want %d", len(got), err, len(rows)+1)
	}
	// the range is inclusive on both ends
	got, _ = s.OptionEod(ctx, underlying, "2026-11-09", "2026-11-09")
	if len(got) != 1 || got[0].SessionDay.Format(time.DateOnly) != "2026-11-09" {
		t.Fatalf("upper bound: %+v", got)
	}
	got, _ = s.OptionEod(ctx, underlying, "2026-11-06", "2026-11-06")
	if len(got) != len(rows) {
		t.Fatalf("lower bound: got %d rows, want %d", len(got), len(rows))
	}
	if got, _ = s.OptionEod(ctx, underlying, "2026-11-07", "2026-11-08"); len(got) != 0 {
		t.Fatalf("empty range: %+v", got)
	}
	all, _ := s.OptionEod(ctx, underlying, "2026-11-01", "2026-11-30")
	requireEodOrder(t, all)
	// a null column reads back as nil
	nullDelta := 0
	for _, r := range all {
		if r.Delta == nil {
			nullDelta++
		}
	}
	if nullDelta == 0 {
		t.Fatal("the fixture's null delta did not come back as nil")
	}
}

func TestQuoteSnapshotStoreContract(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rows := loadFixture[contracts.QuoteSnapshot](t, "obs.quote_snapshot.r1.json")
	if len(rows) < 4 {
		t.Fatalf("fixture has %d rows", len(rows))
	}
	var s store.QuoteSnapshotStore = &memQuoteSnapshot{}
	if err := s.InsertQuoteSnapshots(ctx, rows); err != nil {
		t.Fatal(err)
	}
	inst := int64(rows[0].InstrumentID)
	got, _ := s.QuoteSnapshots(ctx, inst, "2026-11-06", "2026-11-06")
	if len(got) != 2 {
		t.Fatalf("instrument %d: got %d rows, want 2 (slots 1000 and 1500)", inst, len(got))
	}
	// a repeat adds no row
	if err := s.InsertQuoteSnapshots(ctx, rows); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.QuoteSnapshots(ctx, inst, "2026-11-06", "2026-11-06"); len(got) != 2 {
		t.Fatalf("repeat: got %d rows, want 2", len(got))
	}
	// a second capture of the same slot adds one
	again := rows[0]
	again.CaptureID = "00000000-0000-4000-8000-0000000000ff"
	if err := s.InsertQuoteSnapshots(ctx, []contracts.QuoteSnapshot{again}); err != nil {
		t.Fatal(err)
	}
	got, _ = s.QuoteSnapshots(ctx, inst, "2026-11-06", "2026-11-06")
	if len(got) != 3 {
		t.Fatalf("new capture: got %d rows, want 3", len(got))
	}
	// order: slot, then capture_id
	if got[0].Slot != "1000" || got[1].Slot != "1000" || got[2].Slot != "1500" || got[0].CaptureID > got[1].CaptureID {
		t.Fatalf("order: %+v", got)
	}
	// inclusive range and an empty one
	if got, _ = s.QuoteSnapshots(ctx, inst, "2026-11-05", "2026-11-06"); len(got) != 3 {
		t.Fatalf("upper bound: got %d rows", len(got))
	}
	if got, _ = s.QuoteSnapshots(ctx, inst, "2026-11-07", "2026-11-08"); len(got) != 0 {
		t.Fatalf("empty range: %+v", got)
	}
	// a null price reads back as nil
	nulls, _ := s.QuoteSnapshots(ctx, int64(rows[2].InstrumentID), "2026-11-06", "2026-11-06")
	if len(nulls) == 0 || nulls[0].Bid != nil {
		t.Fatalf("null bid: %+v", nulls)
	}
}

func TestSnapshotDSNEnvNames(t *testing.T) {
	t.Parallel()
	if store.DSNEnvOptionEod != "OT_PG_DSN_OPTION_EOD" || store.DSNEnvQuoteSnapshot != "OT_PG_DSN_QUOTE_SNAPSHOT" {
		t.Fatalf("got %q and %q", store.DSNEnvOptionEod, store.DSNEnvQuoteSnapshot)
	}
}
