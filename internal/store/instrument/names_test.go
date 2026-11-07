package instrument

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

func TestInstrumentNames(t *testing.T) {
	t.Parallel()
	f := newFake()
	// a database that ignores the ORDER BY; id 3 has no row
	f.rows = [][]any{
		{int64(2), "index", "SPX"},
		{int64(1), "stock", "SPY"},
	}
	got, err := (&instrumentNameStore{db: f}).InstrumentNames(context.Background(), []int64{2, 3, 1})
	if err != nil {
		t.Fatal(err)
	}
	want := []store.InstrumentName{{InstrumentID: 1, Type: "stock", Name: "SPY"}, {InstrumentID: 2, Type: "index", Name: "SPX"}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if len(f.selSQL) != 1 {
		t.Fatalf("selects = %d", len(f.selSQL))
	}
}

func TestInstrumentNamesUnknownIDIsAbsent(t *testing.T) {
	t.Parallel()
	got, err := (&instrumentNameStore{db: newFake()}).InstrumentNames(context.Background(), []int64{9})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestInstrumentNamesEmptyMakesNoCall(t *testing.T) {
	t.Parallel()
	f := newFake()
	got, err := (&instrumentNameStore{db: f}).InstrumentNames(context.Background(), nil)
	if err != nil || len(got) != 0 || len(f.selSQL) != 0 {
		t.Fatalf("got %v, %v, %d selects", got, err, len(f.selSQL))
	}
}

func TestInstrumentNamesTooManyIDsIsPermanent(t *testing.T) {
	t.Parallel()
	f := newFake()
	ids := make([]int64, 501)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	_, err := (&instrumentNameStore{db: f}).InstrumentNames(context.Background(), ids)
	if k, ok := kindOf(err); !ok || k != contracts.Permanent || len(f.selSQL) != 0 {
		t.Fatalf("err %v, selects %d", err, len(f.selSQL))
	}
	if _, err := (&instrumentNameStore{db: f}).InstrumentNames(context.Background(), ids[:500]); err != nil {
		t.Fatalf("500 ids: %v", err)
	}
}

func TestInstrumentNamesZeroIDIsPermanent(t *testing.T) {
	t.Parallel()
	f := newFake()
	_, err := (&instrumentNameStore{db: f}).InstrumentNames(context.Background(), []int64{1, 0})
	if k, ok := kindOf(err); !ok || k != contracts.Permanent || len(f.selSQL) != 0 {
		t.Fatalf("err %v, selects %d", err, len(f.selSQL))
	}
}

func TestInstrumentNamesDatabaseErrorIsRetryable(t *testing.T) {
	t.Parallel()
	_, err := (&instrumentNameStore{db: failDB{}}).InstrumentNames(context.Background(), []int64{1})
	if k, ok := kindOf(err); !ok || k != contracts.Retryable {
		t.Fatalf("want Retryable, got %v", err)
	}
}

func TestInstrumentNamesClose(t *testing.T) {
	t.Parallel()
	f := newFake()
	if err := (&instrumentNameStore{db: f}).Close(); err != nil || !f.closed {
		t.Fatalf("close: %v, closed %v", err, f.closed)
	}
}

var _ store.InstrumentNameStore = (*instrumentNameStore)(nil)
