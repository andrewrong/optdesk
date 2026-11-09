package cutover_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/andrewrong/optdesk/internal/jobs/cutover"
	"github.com/andrewrong/optdesk/internal/store"
)

// fakeAccount is an AccountStore with the insert-only rule of the contract; it counts the writes.
type fakeAccount struct {
	mu       sync.Mutex
	captures map[string]store.Capture
	writes   int
}

func newFake() *fakeAccount { return &fakeAccount{captures: map[string]store.Capture{}} }

func (f *fakeAccount) WriteCapture(_ context.Context, c store.Capture) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes++
	if _, ok := f.captures[c.ID]; !ok {
		f.captures[c.ID] = c
	}
	return nil
}

func (f *fakeAccount) WriteExecutions(context.Context, string, []store.Execution) error { return nil }

func (f *fakeAccount) Positions(_ context.Context, alias, day string) ([]store.Position, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []store.Position{}
	for _, c := range f.captures {
		if c.Alias == alias && c.SessionDay == day {
			out = append(out, c.Positions...)
		}
	}
	return out, nil
}

func (f *fakeAccount) Values(_ context.Context, alias, day string) ([]store.AccountValue, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []store.AccountValue{}
	for _, c := range f.captures {
		if c.Alias == alias && c.SessionDay == day {
			out = append(out, c.Values...)
		}
	}
	return out, nil
}

func (f *fakeAccount) Executions(context.Context, string, string, string) ([]store.Execution, error) {
	return []store.Execution{}, nil
}

// failingStore fails the test when anything is written.
type failingStore struct {
	*fakeAccount
	t *testing.T
}

func (f failingStore) WriteCapture(context.Context, store.Capture) error {
	f.t.Error("WriteCapture called")
	return nil
}

func line(alias, day, netliq, qty string) string {
	return `{"alias":"` + alias + `","session_day":"` + day + `","at":"` + day + `T21:00:00Z","status":"ok",` +
		`"values":[{"tag":"NetLiquidation","value":"` + netliq + `","currency":"USD"}],` +
		`"positions":[{"symbol":"SPY","sec_type":"STK","exchange":"","currency":"USD","expiry":"","strike":"",` +
		`"multiplier":"1","right":"","con_id":7,"quantity":"` + qty + `","avg_cost":"500.25"}]}` + "\n"
}

const through = "2026-11-06"

func TestImportAccountCaptureID(t *testing.T) {
	t.Parallel()
	f := newFake()
	in := line("main", "2026-11-05", "100.5", "10") + line("main", "2026-11-06", "101.5", "10")
	res, err := cutover.ImportAccount(context.Background(), strings.NewReader(in), f, nil, through)
	if err != nil {
		t.Fatal(err)
	}
	if res.Lines != 2 || res.Inserted != 2 || f.writes != 2 {
		t.Fatalf("res = %+v, writes %d", res, f.writes)
	}
	c, ok := f.captures["main:2026-11-05:import"]
	if !ok || c.Alias != "main" || c.SessionDay != "2026-11-05" || len(c.Positions) != 1 || len(c.Values) != 1 {
		t.Fatalf("capture = %+v, %v", c, ok)
	}
	if _, ok := f.captures["main:2026-11-06:import"]; !ok {
		t.Fatal("second capture missing")
	}
}

func TestImportAccountCountsSame(t *testing.T) {
	t.Parallel()
	f := newFake()
	in := line("main", "2026-11-05", "100.5", "10")
	if _, err := cutover.ImportAccount(context.Background(), strings.NewReader(in), f, nil, through); err != nil {
		t.Fatal(err)
	}
	f.writes = 0
	res, err := cutover.ImportAccount(context.Background(), strings.NewReader(in), f, nil, through)
	if err != nil {
		t.Fatal(err)
	}
	if res.Same != 1 || res.Inserted != 0 || res.Conflict != 0 || f.writes != 0 {
		t.Fatalf("res = %+v, writes %d", res, f.writes)
	}
}

func TestImportAccountCountsConflict(t *testing.T) {
	t.Parallel()
	f := newFake()
	if _, err := cutover.ImportAccount(context.Background(), strings.NewReader(line("main", "2026-11-05", "100.5", "10")), f, nil, through); err != nil {
		t.Fatal(err)
	}
	f.writes = 0
	res, err := cutover.ImportAccount(context.Background(), strings.NewReader(line("main", "2026-11-05", "100.5", "11")), f, nil, through)
	if err != nil {
		t.Fatal(err)
	}
	if res.Conflict != 1 || res.Inserted != 0 || res.Same != 0 || f.writes != 0 {
		t.Fatalf("res = %+v, writes %d", res, f.writes)
	}
}

func TestImportAccountRefusesAfterThrough(t *testing.T) {
	t.Parallel()
	st := failingStore{newFake(), t}
	in := line("main", "2026-11-05", "1", "1") + line("main", "2026-11-09", "1", "1")
	_, err := cutover.ImportAccount(context.Background(), strings.NewReader(in), st, nil, through)
	var le *cutover.LineError
	if !errors.As(err, &le) || le.Line != 2 {
		t.Fatalf("err = %v", err)
	}
}

func TestImportAccountErrorsHoldNoValue(t *testing.T) {
	t.Parallel()
	const marker = "MARKER-98765.4321"
	st := failingStore{newFake(), t}
	bad := []string{
		line("main", "2026-11-09", marker, marker),                                        // after through
		line("other", "2026-11-05", marker, marker),                                       // unmapped alias
		`{"alias":"main","session_day":"` + marker + `"}` + "\n",                          // bad day
		`{"alias":"main", ` + marker + "\n",                                               // not JSON
		line("main", "2026-11-05", marker, "1") + line("main", "2026-11-05", marker, "1"), // duplicate
	}
	for _, in := range bad {
		_, err := cutover.ImportAccount(context.Background(), strings.NewReader(in), st, map[string]string{"main": "main"}, through)
		if err == nil {
			t.Fatalf("no error for %q", in)
		}
		if strings.Contains(err.Error(), marker) || strings.Contains(err.Error(), "main") || strings.Contains(err.Error(), "other") {
			t.Fatalf("error holds content: %v", err)
		}
	}
}

func (f *fakeAccount) Close() error { return nil }
