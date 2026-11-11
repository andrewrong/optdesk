package computefigures

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/figures"
	"github.com/andrewrong/optdesk/internal/compute/gonumstats"
	"github.com/andrewrong/optdesk/internal/compute/own"
	"github.com/andrewrong/optdesk/internal/compute/rules"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
)

const hvName = "market.hv30_own"

// fakeBars is a bar.PriceBarReader over rows kept by instrument and basis; it records every call.
type fakeBars struct {
	rows  map[string][]contracts.PriceBar // "id|basis"
	calls []barCall
}

type barCall struct {
	id       int64
	basis    string
	from, to contracts.Day
}

func (b *fakeBars) PriceBars(_ context.Context, id int64, basis string, from, to contracts.Day) ([]contracts.PriceBar, error) {
	b.calls = append(b.calls, barCall{id, basis, from, to})
	var out []contracts.PriceBar
	for _, r := range b.rows[fmt.Sprintf("%d|%s", id, basis)] {
		if d := dayOf(r.SessionDay); d >= from && d <= to {
			out = append(out, r)
		}
	}
	return out, nil
}

// addCloses stores trades closes for instrument 1 on the n days ending at last, ascending.
func (b *fakeBars) addCloses(basis string, last contracts.Day, closes []float64) {
	if b.rows == nil {
		b.rows = map[string][]contracts.PriceBar{}
	}
	key := "1|" + basis
	for i, c := range closes {
		d := shiftDay(last, i-len(closes)+1)
		b.rows[key] = append(b.rows[key], contracts.PriceBar{InstrumentID: 1, Basis: gen.PriceBarBasis(basis),
			Close: strconv.FormatFloat(c, 'f', -1, 64), SessionDay: dateOf(d), AdjustEpoch: 1})
	}
}

func alternating(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = 100 + float64(i%2)
	}
	return out
}

func realFigs() compute.Figures {
	stats := gonumstats.New()
	return figures.New(stats, own.NewRolling(stats), own.NewSeriesOps(), rules.New())
}

func hvJob(t *testing.T, f *fx, b *fakeBars, figs compute.Figures) contracts.Job {
	t.Helper()
	st := f.stores()
	st.IndexIDs = newIndexes()
	if b != nil {
		st.PriceBars = b
	}
	j, err := New(st, figs, rules.New(), own.NewUnits(), own.NewSeriesOps(), clockwork.NewFakeClockAt(closeD.Add(20*time.Minute)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return j
}

func TestHv30StoredForSubject(t *testing.T) {
	t.Parallel()
	f, b := world(), &fakeBars{}
	b.addCloses("trades", dayD, alternating(31))
	j := hvJob(t, f, b, realFigs())
	res := runAll(t, j)
	if res.Err != nil || res.Returned != res.Usable {
		t.Fatalf("result %+v", res)
	}
	rec := reading(t, f, hvName, "underlying", "SPY")
	// stdev of 15 returns +ln(1.01) and 15 of -ln(1.01), n-1 = 29, times sqrt(252), at 6 places.
	if rec.State != "ok" || rec.Value == nil || *rec.Value != "0.160657" {
		t.Fatalf("reading %+v, want ok 0.160657", rec)
	}
	got := map[string]bool{}
	for _, p := range rec.Parameters {
		got[p.Param] = true
		if p.Param == "window" && (p.ValueNum == nil || *p.ValueNum != 30) {
			t.Fatalf("window %+v", p)
		}
		if p.Param == "observations" && (p.ValueNum == nil || *p.ValueNum != 30) {
			t.Fatalf("observations %+v", p)
		}
		if p.Param == "data_day" && (p.ValueTag == nil || *p.ValueTag != string(dayD)) {
			t.Fatalf("data_day %+v", p)
		}
	}
	if !got["window"] || !got["observations"] || !got["data_day"] || rec.RowsSha256 == "" {
		t.Fatalf("parameters %v, hash %q", got, rec.RowsSha256)
	}
}

func missingCode(t *testing.T, f *fx) int {
	t.Helper()
	rec := reading(t, f, hvName, "underlying", "SPY")
	if rec.State != "missing_input" || rec.Value != nil || rec.ReasonCode == nil {
		t.Fatalf("reading %+v, want missing", rec)
	}
	return *rec.ReasonCode
}

func TestHv30ShortHistoryIsMissing(t *testing.T) {
	t.Parallel()
	f, b := world(), &fakeBars{}
	b.addCloses("trades", dayD, alternating(30))
	runAll(t, hvJob(t, f, b, realFigs()))
	if c := missingCode(t, f); c != 2 {
		t.Fatalf("reason code %d, want 2 (short_history)", c)
	}
}

func TestHv30NoBarsIsNoInput(t *testing.T) {
	t.Parallel()
	f := world()
	runAll(t, hvJob(t, f, &fakeBars{}, realFigs()))
	if c := missingCode(t, f); c != 1 {
		t.Fatalf("reason code %d, want 1 (no_input)", c)
	}
}

func TestHv30StaleIsMissing(t *testing.T) {
	t.Parallel()
	f, b := world(), &fakeBars{}
	b.addCloses("trades", shiftDay(dayD, -10), alternating(31))
	runAll(t, hvJob(t, f, b, realFigs()))
	if c := missingCode(t, f); c != 3 {
		t.Fatalf("reason code %d, want 3 (stale_input)", c)
	}
}

func TestNilPriceBarsSkipsStep(t *testing.T) {
	t.Parallel()
	f := world()
	j := hvJob(t, f, nil, realFigs())
	for _, k := range plan(t, j) {
		if name, _, _, _ := parseKey(k.SubjectKey); name == hvName {
			t.Fatalf("planned %v without PriceBars", k)
		}
	}
	res := runAll(t, j)
	if rows, _ := f.LatestReadings(context.Background(), hvName, "underlying", "SPY", dayD, dayD); len(rows) != 0 {
		t.Fatalf("stored %d hv30 readings without PriceBars", len(rows))
	}
	// every rv20 key fails for want of the bar reader: Permanent, nothing stored, the other keys still stored.
	if res.Err == nil || res.Err.Kind() != contracts.Permanent || !strings.Contains(res.Err.Error(), "no price bar reader") {
		t.Fatalf("Err %v, want Permanent: no price bar reader", res.Err)
	}
	if res.Returned-res.Usable != 3 {
		t.Fatalf("result %+v, want the three rv20 keys failed", res)
	}
	for _, n := range []string{"SPX", "NDX", "RUT"} {
		if rows, _ := f.LatestReadings(context.Background(), "market.rv20", "underlying", n, dayD, dayD); len(rows) != 0 {
			t.Fatalf("stored an rv20 reading of %s without PriceBars", n)
		}
	}
}

func TestHv30PlansOneKeyPerSubject(t *testing.T) {
	t.Parallel()
	j := hvJob(t, world(), &fakeBars{}, realFigs())
	n := 0
	for _, k := range plan(t, j) {
		if k.SubjectKey == hvName+"|underlying|SPY" && k.DatasetID == "readings.reading" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("hv30 keys for SPY: %d, want 1", n)
	}
}

func TestHv30UsesTradesBasisOnly(t *testing.T) {
	t.Parallel()
	f, b := world(), &fakeBars{}
	b.addCloses("adjusted_last", dayD, alternating(31)) // only the other basis holds rows
	runAll(t, hvJob(t, f, b, realFigs()))
	if len(b.calls) == 0 {
		t.Fatal("no read of the bars")
	}
	for _, c := range b.calls {
		if c.basis != "trades" {
			t.Fatalf("read basis %q", c.basis)
		}
	}
	if c := missingCode(t, f); c != 1 {
		t.Fatalf("reason code %d, want 1: adjusted_last rows must not be used", c)
	}
}

// seriesFigs records the series HistoricalVol is asked for.
type seriesFigs struct {
	*fakeFigs
	got compute.Series
}

func (s *seriesFigs) HistoricalVol(id string, c compute.Series, d contracts.Day) compute.Figure {
	if id == hvName {
		s.got = c
	}
	return s.fakeFigs.HistoricalVol(id, c, d)
}

func TestHv30ClosesAscendingLast31(t *testing.T) {
	t.Parallel()
	f, b := world(), &fakeBars{}
	closes := make([]float64, 40)
	for i := range closes {
		closes[i] = float64(100 + i)
	}
	b.addCloses("trades", dayD, closes)
	sf := &seriesFigs{fakeFigs: newFigs()}
	runAll(t, hvJob(t, f, b, sf))
	if len(sf.got) < 31 {
		t.Fatalf("series of %d points", len(sf.got))
	}
	for i := 1; i < len(sf.got); i++ {
		if sf.got[i].Day <= sf.got[i-1].Day {
			t.Fatalf("not ascending at %d: %v", i, sf.got)
		}
	}
	last := sf.got[len(sf.got)-31:]
	for i, p := range last {
		if want := float64(109 + i); p.Value != want {
			t.Fatalf("point %d = %v, want %v", i, p.Value, want)
		}
	}
	if last[30].Day != dayD {
		t.Fatalf("newest day %s", last[30].Day)
	}
	c := b.calls[0]
	if c.id != 1 || c.to != dayD || c.from != shiftDay(dayD, -70) {
		t.Fatalf("read %+v, want instrument 1, %s to %s", c, shiftDay(dayD, -70), dayD)
	}
}
