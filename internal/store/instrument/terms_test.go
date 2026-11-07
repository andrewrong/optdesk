package instrument

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

func kindOf(err error) (contracts.ErrorKind, bool) {
	var te contracts.TypedError
	if errors.As(err, &te) {
		return te.Kind(), true
	}
	return contracts.Retryable, false
}

func sameDecimal(t *testing.T, name, got, want string) {
	t.Helper()
	g, ok1 := new(big.Rat).SetString(got)
	w, ok2 := new(big.Rat).SetString(want)
	if !ok1 || !ok2 || g.Cmp(w) != 0 {
		t.Fatalf("%s = %q, want the value %q", name, got, want)
	}
}

func TestOptionTerms(t *testing.T) {
	t.Parallel()
	from := time.Date(2026, 11, 1, 14, 30, 0, 0, time.UTC)
	f := newFake()
	// the view returns the current versions ordered by instrument id; id 3 has none
	f.rows = [][]any{
		{int64(11), int64(1), "C", "450.00", "USD", "100", "2026-12-18", "american", "physical", from},
		{int64(12), int64(2), "P", "4500", "USD", "100.0", "2027-01-15", "european", "cash", from},
	}
	got, err := (&optionTermsStore{db: f}).OptionTerms(context.Background(), []int64{2, 3, 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].InstrumentID != 1 || got[1].InstrumentID != 2 {
		t.Fatalf("got %+v", got)
	}
	a := got[0]
	if a.TermsVersionID != 11 || a.Right != "C" || a.StrikeCurrency != "USD" || a.ExpirationDate != "2026-12-18" ||
		a.ExerciseStyle != "american" || a.SettlementType != "physical" || !a.ValidFrom.Equal(from) {
		t.Fatalf("first = %+v", a)
	}
	sameDecimal(t, "Strike", a.Strike, "450")
	sameDecimal(t, "Multiplier", a.Multiplier, "100.00")
	sameDecimal(t, "Strike 2", got[1].Strike, "4500.0")
	sameDecimal(t, "Multiplier 2", got[1].Multiplier, "100")
	if len(f.selSQL) != 1 {
		t.Fatalf("selects = %d", len(f.selSQL))
	}
}

func TestOptionTermsOrdersByInstrumentID(t *testing.T) {
	t.Parallel()
	from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	f := newFake()
	// a database that ignores the ORDER BY: the answer is still ordered
	f.rows = [][]any{
		{int64(12), int64(2), "P", "1", "USD", "100", "2027-01-15", "european", "cash", from},
		{int64(11), int64(1), "C", "1", "USD", "100", "2026-12-18", "american", "physical", from},
	}
	got, err := (&optionTermsStore{db: f}).OptionTerms(context.Background(), []int64{1, 2})
	if err != nil || len(got) != 2 || got[0].InstrumentID != 1 || got[1].InstrumentID != 2 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestOptionTermsEmptyMakesNoCall(t *testing.T) {
	t.Parallel()
	f := newFake()
	got, err := (&optionTermsStore{db: f}).OptionTerms(context.Background(), nil)
	if err != nil || len(got) != 0 || len(f.selSQL) != 0 {
		t.Fatalf("got %v, %v, %d selects", got, err, len(f.selSQL))
	}
}

func TestOptionTermsTooManyIDsIsPermanent(t *testing.T) {
	t.Parallel()
	f := newFake()
	ids := make([]int64, 501)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	_, err := (&optionTermsStore{db: f}).OptionTerms(context.Background(), ids)
	if k, ok := kindOf(err); !ok || k != contracts.Permanent || len(f.selSQL) != 0 {
		t.Fatalf("err %v, selects %d", err, len(f.selSQL))
	}
	if _, err := (&optionTermsStore{db: f}).OptionTerms(context.Background(), ids[:500]); err != nil {
		t.Fatalf("500 ids: %v", err)
	}
}

func TestOptionTermsZeroIDIsPermanent(t *testing.T) {
	t.Parallel()
	f := newFake()
	_, err := (&optionTermsStore{db: f}).OptionTerms(context.Background(), []int64{1, 0})
	if k, ok := kindOf(err); !ok || k != contracts.Permanent || len(f.selSQL) != 0 {
		t.Fatalf("err %v, selects %d", err, len(f.selSQL))
	}
}

func TestOptionTermsDatabaseErrorIsRetryable(t *testing.T) {
	t.Parallel()
	_, err := (&optionTermsStore{db: failDB{}}).OptionTerms(context.Background(), []int64{1})
	if k, ok := kindOf(err); !ok || k != contracts.Retryable {
		t.Fatalf("want Retryable, got %v", err)
	}
}

var _ store.OptionTermsStore = (*optionTermsStore)(nil)
