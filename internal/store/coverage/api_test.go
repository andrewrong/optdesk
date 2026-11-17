package coverage_test

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/coverage"
)

type fakeReader struct{ cells []coverage.Cell }

func (f fakeReader) Cells(context.Context, contracts.Day, contracts.Day) ([]coverage.Cell, error) {
	return f.cells, nil
}

func (fakeReader) Sessions(context.Context, string, contracts.Day, contracts.Day) ([]coverage.Session, error) {
	return []coverage.Session{}, nil
}

func (fakeReader) NewestObservations(context.Context) ([]coverage.Newest, error) {
	return []coverage.Newest{}, nil
}

func (fakeReader) Close() error { return nil }

var _ coverage.Reader = fakeReader{}

func TestReaderContractFake(t *testing.T) {
	t.Parallel()
	open := coverage.OpenFunc(func(context.Context, string) (coverage.Reader, error) {
		return fakeReader{cells: []coverage.Cell{
			{Day: "2026-11-16", Dataset: "a", Keys: 4, OK: 1, NoValue: 1, Expired: 1, Missing: 1, MissingKeys: []string{"k"}},
			{Day: "2026-11-17", Dataset: "a", Keys: 2, OK: 2},
		}}, nil
	})
	r, err := open(context.Background(), "dsn")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	cells, err := r.Cells(context.Background(), "2026-11-16", "2026-11-17")
	if err != nil {
		t.Fatal(err)
	}
	if len(cells) != 2 {
		t.Fatalf("cells = %d, want 2", len(cells))
	}
	for _, c := range cells {
		if c.Keys != c.OK+c.NoValue+c.Expired+c.Missing {
			t.Errorf("%s %s: Keys %d != sum of states", c.Day, c.Dataset, c.Keys)
		}
	}
	if coverage.StateOK != "ok" || coverage.StateNoValue != "no_value" ||
		coverage.StateExpired != "expired" || coverage.StateMissing != "missing" {
		t.Error("state constants differ from the design")
	}
}

func TestEveryMethodReturnsNonNil(t *testing.T) {
	t.Parallel()
	var r coverage.Reader = fakeReader{cells: []coverage.Cell{}}
	ctx := context.Background()
	if c, _ := r.Cells(ctx, "2026-01-01", "2026-01-02"); c == nil {
		t.Error("Cells returned nil")
	}
	if s, _ := r.Sessions(ctx, "XNYS", "2026-01-01", "2026-01-02"); s == nil {
		t.Error("Sessions returned nil")
	}
	if n, _ := r.NewestObservations(ctx); n == nil {
		t.Error("NewestObservations returned nil")
	}
}

func TestWindowSessionsIs30(t *testing.T) {
	t.Parallel()
	if coverage.WindowSessions != 30 {
		t.Fatalf("WindowSessions = %d, want 30", coverage.WindowSessions)
	}
}

func TestLookbackIs60(t *testing.T) {
	t.Parallel()
	if coverage.LookbackDays != 60 {
		t.Fatalf("LookbackDays = %d, want 60", coverage.LookbackDays)
	}
}
