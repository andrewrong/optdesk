package optioneod

import (
	"context"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

type indexRig struct {
	b   *fakeBroker
	res *fakeResolver
	w   *fakeWriter
	clk *clockwork.FakeClock
	job *job
}

// newIndexRig builds the real job on fakes at 11:00 New York time on 2026-11-13; a nil reader means none.
func newIndexRig(t *testing.T, closes IndexCloseReader) *indexRig {
	t.Helper()
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	r := &indexRig{b: newBroker(), res: &fakeResolver{}, w: &fakeWriter{}, clk: clockwork.NewFakeClockAt(time.Date(2026, 11, 13, 11, 0, 0, 0, loc))}
	r.b.price, r.b.noUndPrice = num(7590), true
	strikes := strikesRange(6000, 9000, 5)
	xsp := strikesRange(600, 900, 1)
	r.b.chains = []broker.Chain{
		spxChain([]string{"20261120", "20261217", "20270219"}, strikes),
		{Exchange: "CBOE", TradingClass: "XSP", Multiplier: "100", Expirations: []string{"20261119", "20261120", "20261217", "20261218"}, Strikes: xsp},
		{Exchange: "RUSSELL", TradingClass: "RUT", Multiplier: "100", Expirations: []string{"20261119", "20261217", "20270218"}, Strikes: strikes},
	}
	var reader IndexCloseReader
	if closes != nil {
		reader = closes
	}
	j, err := NewIndex(r.b, r.res, r.w, reader, r.clk)
	if err != nil {
		t.Fatal(err)
	}
	r.job = j.(*job)
	return r
}

func (r *indexRig) run(symbols ...string) contracts.JobResult {
	var keys []contracts.ExpectedKey
	for _, s := range symbols {
		keys = append(keys, contracts.ExpectedKey{DatasetID: "obs.option_eod_index", SubjectKey: s})
	}
	return r.job.Run(context.Background(), "2026-11-13", keys)
}

func TestNewIndexRefusesNil(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClock()
	b, res, w := &fakeBroker{}, &fakeResolver{}, &fakeWriter{}
	cases := []struct {
		name string
		call func() (contracts.Job, error)
	}{
		{"broker", func() (contracts.Job, error) { return NewIndex(nil, res, w, nil, clk) }},
		{"resolver", func() (contracts.Job, error) { return NewIndex(b, nil, w, nil, clk) }},
		{"writer", func() (contracts.Job, error) { return NewIndex(b, res, nil, nil, clk) }},
		{"clock", func() (contracts.Job, error) { return NewIndex(b, res, w, nil, nil) }},
	}
	for _, c := range cases {
		j, err := c.call()
		if err == nil || j != nil || !strings.Contains(err.Error(), c.name) {
			t.Fatalf("%s: job %v, err %v", c.name, j, err)
		}
	}
	if j, err := NewIndex(b, res, w, nil, clk); err != nil || j == nil {
		t.Fatalf("a nil reader is accepted: %v", err)
	}
}

func TestIndexRunStoresPairsWithNullIvAndDelta(t *testing.T) {
	t.Parallel()
	r := newIndexRig(t, nil)
	res := r.run("SPX")
	if res.Err != nil || res.Returned == 0 {
		t.Fatalf("result %+v", res)
	}
	rows := r.w.all()
	if len(rows) != res.Returned {
		t.Fatalf("%d rows for %d returned", len(rows), res.Returned)
	}
	for _, row := range rows {
		if row.Status != 1 || row.IvDecimal != nil || row.Delta != nil || row.UnderlyingPrice == nil || *row.UnderlyingPrice != 7590 {
			t.Fatalf("row %+v", row)
		}
	}
}

func TestIndexRunUsesTheStoredCloseWhenTheSnapshotHasNoPrice(t *testing.T) {
	t.Parallel()
	r := newIndexRig(t, &fakeCloses{closes: map[string][]string{"SPX": {"7400", "7500"}}})
	r.b.price = nil
	if res := r.run("SPX"); res.Err != nil {
		t.Fatal(res.Err)
	}
	for _, row := range r.w.all() {
		if row.UnderlyingPrice == nil || *row.UnderlyingPrice != 7500 {
			t.Fatalf("underlying price %v, want 7500", row.UnderlyingPrice)
		}
	}
}

func TestXspFallbackSpotIsTheSpxCloseOverTen(t *testing.T) {
	t.Parallel()
	fc := &fakeCloses{closes: map[string][]string{"SPX": {"7590"}}}
	r := newIndexRig(t, fc)
	r.b.price = nil
	if res := r.run("XSP"); res.Err != nil || res.Returned == 0 {
		t.Fatalf("result %+v", res)
	}
	rows := r.w.all()
	for _, row := range rows {
		if row.UnderlyingPrice == nil || math.Abs(*row.UnderlyingPrice-759) > 1e-9 {
			t.Fatalf("underlying price %v, want 759", row.UnderlyingPrice)
		}
	}
	for _, c := range r.b.resolved {
		if v, _ := strconv.ParseFloat(c.Strike, 64); v < 600 || v > 920 {
			t.Fatalf("strike %s is not around 759", c.Strike)
		}
	}
}

func TestIndexRunWithoutReaderUsesTheFixedBand(t *testing.T) {
	t.Parallel()
	r := newIndexRig(t, nil)
	r.b.chains[0].Expirations = []string{"20261217"}
	if res := r.run("SPX"); res.Err != nil {
		t.Fatal(res.Err)
	}
	w := 7590 * 1.8 * 0.20 * math.Sqrt(34.0/365)
	lo, hi := minMax(strikeSet(r.b.resolved, "20261217", "C"))
	if lo < 7590-w || lo > 7590-w+100 || hi > 7590+w || hi < 7590+w-100 {
		t.Fatalf("strikes %v..%v do not fill the band %v..%v of sigma 0.20", lo, hi, 7590-w, 7590+w)
	}
}

func TestIndexRunWithoutReaderAndWithoutPriceIsRetryable(t *testing.T) {
	t.Parallel()
	r := newIndexRig(t, nil)
	r.b.price = nil
	if res := r.run("SPX"); kindOf(res) != contracts.Retryable {
		t.Fatalf("result %+v", res)
	}
}

func TestIndexRunKeysAreSpxXspAndRut(t *testing.T) {
	t.Parallel()
	r := newIndexRig(t, nil)
	keys, _, err := r.job.Plan(context.Background(), "2026-11-13")
	if err != nil || len(keys) != 3 {
		t.Fatalf("keys %v, err %v", keys, err)
	}
	for i, s := range []string{"SPX", "XSP", "RUT"} {
		if keys[i].SubjectKey != s || keys[i].DatasetID != "obs.option_eod_index" {
			t.Fatalf("key %d is %+v", i, keys[i])
		}
	}
	res := r.job.Run(context.Background(), "2026-11-13", keys)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	var unders []string
	for _, c := range r.res.calls {
		if c.spec.Type == "index" {
			unders = append(unders, c.key.Value)
		}
	}
	if len(unders) != 3 || unders[0] != "416904" || unders[1] != "137851301" || unders[2] != "416888" {
		t.Fatalf("underlyings resolved in the order %v", unders)
	}
}

func TestIndexRunStaysInTheBudget(t *testing.T) {
	t.Parallel()
	r := newIndexRig(t, nil)
	r.b.afterSnap = func(n int) {
		if n == 1 {
			r.clk.Advance(9 * time.Minute)
		}
	}
	res := r.run("SPX", "XSP", "RUT")
	if kindOf(res) != contracts.Retryable {
		t.Fatalf("result %+v", res)
	}
	if len(r.w.all()) == 0 || len(r.w.all()) != res.Returned {
		t.Fatalf("rows kept %d, returned %d", len(r.w.all()), res.Returned)
	}
}

func TestIndexJobHasItsOwnLedgerDataset(t *testing.T) {
	t.Parallel()
	r := newIndexRig(t, nil)
	if r.job.ID() != "option_eod_index" {
		t.Fatalf("id %s", r.job.ID())
	}
	keys, _, _ := r.job.Plan(context.Background(), "2026-11-13")
	for _, k := range keys {
		if k.DatasetID != "obs.option_eod_index" {
			t.Fatalf("key %+v", k)
		}
	}
}

func TestIndexManifestRefusesBadRows(t *testing.T) {
	t.Parallel()
	good := string(manifestIndexBytes)
	replace := func(old, new string) string { return strings.Replace(good, old, new, 1) }
	cases := map[string]string{
		"strike_multiple": replace("strike_multiple = 50 ", "strike_multiple = 0 "),
		"max_contracts":   replace("max_contracts = 80 ", "max_contracts = 0 "),
		"spot_divisor":    replace("spot_divisor = 10 ", "spot_divisor = 0 "),
		"expiry_kind":     replace(`expiry_kind = "third_friday"`, `expiry_kind = "weekly"`),
		"sigma_index":     replace(`sigma_index = "RUT"`, `sigma_index = ""`),
		"spot_index":      replace(`spot_index = "RUT"`, `spot_index = ""`),
		"band_k":          replace("band_k = 1.8 ", "band_k = 0 "),
		"sigma_fallback":  replace("sigma_fallback = 0.20 ", "sigma_fallback = 0 "),
		"matching":        replace("symbol = \"RUT\"\ntrading_class", "symbol = \"NDX\"\ntrading_class"),
	}
	for key, text := range cases {
		_, err := parseManifest([]byte(text))
		if err == nil || (key != "matching" && !strings.Contains(err.Error(), key)) {
			t.Errorf("%s: err %v", key, err)
		}
	}
}
