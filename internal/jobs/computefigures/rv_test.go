package computefigures

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/own"
	"github.com/andrewrong/optdesk/internal/compute/rules"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
	"github.com/andrewrong/optdesk/internal/store"
)

const rvName = "market.rv20"

// fakeIndexes answers the index instruments whose name is asked for, as the real reader does.
type fakeIndexes struct {
	rows []store.InstrumentName
	err  error
}

func newIndexes() *fakeIndexes {
	return &fakeIndexes{rows: []store.InstrumentName{{InstrumentID: 11, Type: "index", Name: "SPX"},
		{InstrumentID: 12, Type: "index", Name: "NDX"}, {InstrumentID: 13, Type: "index", Name: "RUT"}}}
}

func (f *fakeIndexes) IndexIDs(_ context.Context, names []string) ([]store.InstrumentName, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := []store.InstrumentName{}
	for _, r := range f.rows {
		for _, n := range names {
			if r.Name == n {
				out = append(out, r)
			}
		}
	}
	return out, nil
}

// rvStores is the stores with both readers of the rv20 step: no closes, the three index instruments.
func (f *fx) rvStores() ComputeStores {
	s := f.stores()
	s.PriceBars = &fakeBars{}
	s.IndexIDs = newIndexes()
	return s
}

func rvJobOn(t *testing.T, st ComputeStores, figs compute.Figures) contracts.Job {
	t.Helper()
	j, err := New(st, figs, rules.New(), own.NewUnits(), own.NewSeriesOps(), clockwork.NewFakeClockAt(closeD.Add(20*time.Minute)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return j
}

// rvJob is a plain job with both readers of the rv20 step.
func rvJob(t *testing.T, f *fx, figs compute.Figures) contracts.Job {
	t.Helper()
	return rvJobOn(t, f.rvStores(), figs)
}

// addFor stores closes for the instrument id on the days ending at last.
func (b *fakeBars) addFor(id int64, basis string, last contracts.Day, closes []float64) {
	if b.rows == nil {
		b.rows = map[string][]contracts.PriceBar{}
	}
	key := strconv.FormatInt(id, 10) + "|" + basis
	for i, c := range closes {
		d := shiftDay(last, i-len(closes)+1)
		b.rows[key] = append(b.rows[key], contracts.PriceBar{InstrumentID: int(id), Basis: gen.PriceBarBasis(basis),
			Close: strconv.FormatFloat(c, 'f', -1, 64), SessionDay: dateOf(d), AdjustEpoch: 1})
	}
}

func rvReading(t *testing.T, f *fx) contracts.ReadingRecord {
	t.Helper()
	return reading(t, f, rvName, "underlying", "SPX")
}

func runRv(t *testing.T, b *fakeBars) (*fx, contracts.JobResult) {
	t.Helper()
	f := world()
	st := f.rvStores()
	st.PriceBars = b
	return f, runAll(t, rvJobOn(t, st, realFigs()))
}

func TestRv20StoresHandWorkedValue(t *testing.T) {
	t.Parallel()
	b := &fakeBars{}
	b.addFor(11, "trades", dayD, alternating(21))
	b.addFor(11, "adjusted_last", dayD, []float64{5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25})
	f, res := runRv(t, b)
	if res.Err != nil {
		t.Fatalf("result %+v", res)
	}
	// ln(1.01) x sqrt(20/19) x sqrt(252), at 6 places, from the trades rows and not the adjusted_last ones.
	rec := rvReading(t, f)
	if rec.State != "ok" || rec.Value == nil || *rec.Value != "0.162060" {
		t.Fatalf("reading %+v, want ok 0.162060", rec)
	}
	for _, p := range rec.Parameters {
		if p.Param == "window" && (p.ValueNum == nil || *p.ValueNum != 20) {
			t.Fatalf("window %+v", p)
		}
	}
	for _, n := range []string{"NDX", "RUT"} {
		if r := reading(t, f, rvName, "underlying", n); r.State != "missing_input" {
			t.Fatalf("%s reading %+v, want missing for want of closes", n, r)
		}
	}
	for _, name := range []string{"market.ivp_own", "market.ivr_own"} {
		if rows, _ := f.LatestReadings(context.Background(), name, "underlying", "SPX", dayD, dayD); len(rows) != 0 {
			t.Fatalf("runIvStats stored %s for SPX", name)
		}
	}
}

func rvMissing(t *testing.T, b *fakeBars, wantCode int, why string) {
	t.Helper()
	f, _ := runRv(t, b)
	rec := rvReading(t, f)
	if rec.State != "missing_input" || rec.Value != nil || rec.ReasonCode == nil || *rec.ReasonCode != wantCode {
		t.Fatalf("reading %+v, want missing %s (code %d) and no value", rec, why, wantCode)
	}
}

func TestRv20ShortHistoryIsMissing(t *testing.T) {
	t.Parallel()
	b := &fakeBars{}
	b.addFor(11, "trades", dayD, alternating(20))
	rvMissing(t, b, 2, "short_history")
}

func TestRv20NoBarsIsNoInput(t *testing.T) {
	t.Parallel()
	rvMissing(t, &fakeBars{}, 1, "no_input")
}

func TestRv20StaleIsMissing(t *testing.T) {
	t.Parallel()
	b := &fakeBars{}
	b.addFor(11, "trades", shiftDay(dayD, -5), alternating(21))
	rvMissing(t, b, 3, "stale_input")
}

func TestRv20CloseNotAboveZeroIsBadInput(t *testing.T) {
	t.Parallel()
	for _, bad := range []float64{0, -1} {
		cs := alternating(21)
		cs[10] = bad
		b := &fakeBars{}
		b.addFor(11, "trades", dayD, cs)
		f, _ := runRv(t, b)
		rec := rvReading(t, f)
		// bad_input is reason code 5; the value is never stored, never 0.
		if rec.State != "missing_input" || rec.Value != nil || rec.ReasonCode == nil || *rec.ReasonCode != 5 {
			t.Fatalf("close %v: reading %+v, want missing bad_input without a value", bad, rec)
		}
	}
}

func TestRv20NoBarReaderFails(t *testing.T) {
	t.Parallel()
	f := world()
	st := f.rvStores()
	st.PriceBars = nil
	res := runAll(t, rvJobOn(t, st, realFigs()))
	wantRvFailure(t, f, res, contracts.Permanent, "no price bar reader", 3)
}

func TestRv20NoIndexReaderFails(t *testing.T) {
	t.Parallel()
	f := world()
	st := f.rvStores()
	st.IndexIDs = nil
	res := runAll(t, rvJobOn(t, st, realFigs()))
	wantRvFailure(t, f, res, contracts.Permanent, "no index reader", 3)
}

func TestRv20UnknownIndexFailsRetryable(t *testing.T) {
	t.Parallel()
	f := world()
	st := f.rvStores()
	st.IndexIDs = &fakeIndexes{rows: newIndexes().rows[:2]} // no RUT
	res := runAll(t, rvJobOn(t, st, realFigs()))
	wantRvFailure(t, f, res, contracts.Retryable, "no index instrument named RUT", 1)
	if r := reading(t, f, rvName, "underlying", "SPX"); r.State == "" {
		t.Fatal("the known indices are still stored")
	}
}

func TestRv20AmbiguousIndexFailsPermanent(t *testing.T) {
	t.Parallel()
	f := world()
	st := f.rvStores()
	st.IndexIDs = &fakeIndexes{rows: append(newIndexes().rows, store.InstrumentName{InstrumentID: 99, Type: "index", Name: "NDX"})}
	res := runAll(t, rvJobOn(t, st, realFigs()))
	wantRvFailure(t, f, res, contracts.Permanent, "ambiguous", 1)
}

func TestRv20IndexReaderErrorIsRetryable(t *testing.T) {
	t.Parallel()
	f := world()
	st := f.rvStores()
	st.IndexIDs = &fakeIndexes{err: errors.New("boom")}
	res := runAll(t, rvJobOn(t, st, realFigs()))
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || !strings.Contains(res.Err.Error(), "boom") {
		t.Fatalf("Err %v, want Retryable boom", res.Err)
	}
}

// wantRvFailure checks the first error, that exactly failed rv20 keys are not stored, and that the other keys are.
func wantRvFailure(t *testing.T, f *fx, res contracts.JobResult, kind contracts.ErrorKind, text string, failed int) {
	t.Helper()
	if res.Err == nil || res.Err.Kind() != kind || !strings.Contains(res.Err.Error(), text) {
		t.Fatalf("Err %v, want %v containing %q", res.Err, kind, text)
	}
	if res.Returned-res.Usable != failed {
		t.Fatalf("result %+v, want %d keys failed", res, failed)
	}
	stored := 0
	for _, n := range []string{"SPX", "NDX", "RUT"} {
		rows, _ := f.LatestReadings(context.Background(), rvName, "underlying", n, dayD, dayD)
		stored += len(rows)
	}
	if stored != 3-failed {
		t.Fatalf("%d rv20 readings stored, want %d", stored, 3-failed)
	}
	if r := reading(t, f, "market.iv30_own", "underlying", "SPY"); r.State == "" {
		t.Fatal("the other keys are stored")
	}
}
