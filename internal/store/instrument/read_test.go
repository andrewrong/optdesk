package instrument

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

func TestInstruments(t *testing.T) {
	t.Parallel()
	f := newFake()
	// the join returns one row per valid identifier, ordered by id; id 3 is not in the database
	f.rows = [][]any{
		{int64(1), "stock", "SPY", "USD", "XARC", "XNYS", "active", "0.01", "1", "ibkr_conid", "756733"},
		{int64(1), "stock", "SPY", "USD", "XARC", "XNYS", "active", "0.01", "1", "ticker_exchange", "SPY:ARCA"},
		{int64(2), "option", "SPY C450", "USD", "XNYS", "XNYS", "active", "0.0500", "100.0", "osi", "SPY   261120C00450000"},
		{int64(4), "index", "SPX", "USD", "XCBO", "XNYS", "active", "0.1", "1", nil, nil},
	}
	got, err := (&instrumentStore{db: f}).Instruments(context.Background(), []int64{4, 2, 1, 3})
	if err != nil {
		t.Fatal(err)
	}
	spec := func(typ, name, mic, inc, lot string) store.InstrumentSpec {
		return store.InstrumentSpec{Type: typ, Name: name, Currency: "USD", MIC: mic, CalendarID: "XNYS", Status: "active",
			PriceIncrement: inc, LotSize: lot}
	}
	want := []store.Instrument{
		{ID: 1, Spec: spec("stock", "SPY", "XARC", "0.01", "1"),
			Identifiers: []store.Identifier{{Scheme: "ibkr_conid", Value: "756733"}, {Scheme: "ticker_exchange", Value: "SPY:ARCA"}}},
		{ID: 2, Spec: spec("option", "SPY C450", "XNYS", "0.0500", "100.0"),
			Identifiers: []store.Identifier{{Scheme: "osi", Value: "SPY   261120C00450000"}}},
		{ID: 4, Spec: spec("index", "SPX", "XCBO", "0.1", "1")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestInstrumentsEmptyMakesNoCall(t *testing.T) {
	t.Parallel()
	f := newFake()
	got, err := (&instrumentStore{db: f}).Instruments(context.Background(), nil)
	if err != nil || len(got) != 0 || len(f.selSQL) != 0 {
		t.Fatalf("got %v, %v, %d selects", got, err, len(f.selSQL))
	}
}

func TestInstrumentsDatabaseErrorIsRetryable(t *testing.T) {
	t.Parallel()
	_, err := (&instrumentStore{db: failDB{}}).Instruments(context.Background(), []int64{1})
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Retryable {
		t.Fatalf("want Retryable, got %v", err)
	}
}
