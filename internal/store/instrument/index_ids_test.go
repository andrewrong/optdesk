package instrument

import (
	"context"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// noCallDB fails the test when any database method is called.
type noCallDB struct{ t *testing.T }

func (d noCallDB) InTx(context.Context, func(execer) error) error {
	d.t.Error("no database call expected: InTx")
	return nil
}

func (d noCallDB) Select(context.Context, string, []any, func(scanner) error) error {
	d.t.Error("no database call expected: Select")
	return nil
}

func (d noCallDB) Close() error { return nil }

func TestIndexIDsEmptyMakesNoDatabaseCall(t *testing.T) {
	t.Parallel()
	got, err := (&IndexIDs{db: noCallDB{t}}).IndexIDs(context.Background(), nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestIndexIDsTooManyNamesIsPermanent(t *testing.T) {
	t.Parallel()
	names := make([]string, 501)
	for i := range names {
		names[i] = "N" + strings.Repeat("x", i%7)
	}
	_, err := (&IndexIDs{db: noCallDB{t}}).IndexIDs(context.Background(), names)
	if k, ok := kindOf(err); !ok || k != contracts.Permanent {
		t.Fatalf("err %v", err)
	}
	f := newFake()
	if _, err := (&IndexIDs{db: f}).IndexIDs(context.Background(), names[:500]); err != nil || len(f.selSQL) != 1 {
		t.Fatalf("500 names: %v, %d selects", err, len(f.selSQL))
	}
}

func TestIndexIDsEmptyNameIsPermanent(t *testing.T) {
	t.Parallel()
	_, err := (&IndexIDs{db: noCallDB{t}}).IndexIDs(context.Background(), []string{"SPX", ""})
	if k, ok := kindOf(err); !ok || k != contracts.Permanent {
		t.Fatalf("err %v", err)
	}
}

func TestIndexIDsOrdersByIDAndPassesNames(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.rows = [][]any{{int64(7), "index", "RUT"}, {int64(3), "index", "SPX"}}
	got, err := (&IndexIDs{db: f}).IndexIDs(context.Background(), []string{"RUT", "SPX", "NDX"})
	if err != nil {
		t.Fatal(err)
	}
	want := []store.InstrumentName{{InstrumentID: 3, Type: "index", Name: "SPX"}, {InstrumentID: 7, Type: "index", Name: "RUT"}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if len(f.selSQL) != 1 || f.selSQL[0] != indexIDsSQL {
		t.Fatalf("sql %v", f.selSQL)
	}
}
