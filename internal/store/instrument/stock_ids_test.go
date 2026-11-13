package instrument

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

func TestStockIDsOrdersById(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.rows = [][]any{{int64(7), "stock", "MSFT"}, {int64(3), "stock", "AAPL"}}
	got, err := (&StockIDs{db: f}).StockIDs(context.Background(), []string{"MSFT", "AAPL", "ZZZ"})
	if err != nil {
		t.Fatal(err)
	}
	want := []store.InstrumentName{{InstrumentID: 3, Type: "stock", Name: "AAPL"}, {InstrumentID: 7, Type: "stock", Name: "MSFT"}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if len(f.selSQL) != 1 || f.selSQL[0] != stockIDsSQL {
		t.Fatalf("sql %v", f.selSQL)
	}
}

func TestStockIDsEmptyMakesNoCall(t *testing.T) {
	t.Parallel()
	got, err := (&StockIDs{db: noCallDB{t}}).StockIDs(context.Background(), nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestStockIDsRefusesTooManyAndEmptyName(t *testing.T) {
	t.Parallel()
	names := make([]string, 501)
	for i := range names {
		names[i] = "N" + string(rune('A'+i%26)) + string(rune('A'+i/26))
	}
	r := &StockIDs{db: noCallDB{t}}
	if _, err := r.StockIDs(context.Background(), names); !isPermanent(err) {
		t.Fatalf("too many: %v", err)
	}
	if _, err := r.StockIDs(context.Background(), []string{"AAPL", ""}); !isPermanent(err) {
		t.Fatalf("empty name: %v", err)
	}
	f := newFake()
	if _, err := (&StockIDs{db: f}).StockIDs(context.Background(), names[:500]); err != nil || len(f.selSQL) != 1 {
		t.Fatalf("500 names: %v, %d selects", err, len(f.selSQL))
	}
}

func TestStockIDsTwoRowsOfOneName(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.rows = [][]any{{int64(9), "stock", "ABC"}, {int64(4), "stock", "ABC"}}
	got, err := (&StockIDs{db: f}).StockIDs(context.Background(), []string{"ABC"})
	if err != nil || len(got) != 2 || got[0].InstrumentID != 4 || got[1].InstrumentID != 9 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func isPermanent(err error) bool {
	k, ok := kindOf(err)
	return ok && k == contracts.Permanent
}
