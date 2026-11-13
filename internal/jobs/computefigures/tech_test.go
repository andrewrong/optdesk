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
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/compute/own"
	"github.com/andrewrong/optdesk/internal/compute/rules"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	techName = "market.tech_state"
	techKey  = "market.tech_state|market|NDX_SPX"
	ndxID    = 12
	spxID    = 11
)

var testTechParams = blockapi.TechParams{Band: "1.00", Hard: "5.00", Short: 21, Long: 63, Fold: 10, MaxAgeDays: 5}

// techFake is a TechFunc that records its input and answers res.
type techFake struct {
	res   blockapi.TechResult
	in    []blockapi.TechIn
	calls int
}

func (f *techFake) classify(in blockapi.TechIn) blockapi.TechResult {
	f.calls++
	f.in = append(f.in, in)
	return f.res
}

func okTech() blockapi.TechResult {
	return blockapi.TechResult{State: "LAGGING HARD", Raw: "LAGGING", Rank: -2, DataDay: dayD, RawDays: 7,
		NDX1m: "-1.00", SPX1m: "2.00", Gap1m: "-3.00", NDX3m: "-4.00", SPX3m: "3.00", Gap3m: "-7.00"}
}

func addTechBars(b *fakeBars, id int, days int, closeOf func(i int) string) {
	if b.rows == nil {
		b.rows = map[string][]contracts.PriceBar{}
	}
	key := strconv.Itoa(id) + "|trades"
	for i := 0; i < days; i++ {
		d := shiftDay(dayD, i-days+1)
		b.rows[key] = append(b.rows[key], contracts.PriceBar{InstrumentID: id, Basis: gen.PriceBarBasis("trades"),
			Close: closeOf(i), SessionDay: dateOf(d), AdjustEpoch: 1})
	}
}

func techWorld(t *testing.T, fake *techFake, idx *fakeIndexes) (contracts.Job, *fx, *fakeBars) {
	t.Helper()
	f, b := world(), &fakeBars{}
	addTechBars(b, ndxID, 3, func(i int) string { return "100.5" + strconv.Itoa(i) })
	addTechBars(b, spxID, 3, func(i int) string { return "50.2" + strconv.Itoa(i) })
	st := f.stores()
	st.IndexIDs, st.PriceBars = idx, b
	j, err := New(st, realFigs(), rules.New(), own.NewUnits(), own.NewSeriesOps(), clockwork.NewFakeClockAt(closeD.Add(20*time.Minute)))
	if err != nil {
		t.Fatal(err)
	}
	return WithTech(j, TechDeps{Classify: fake.classify, Params: testTechParams}), f, b
}

func runTech(j contracts.Job) contracts.JobResult {
	return j.Run(context.Background(), dayD, []contracts.ExpectedKey{{DatasetID: datasetReadings, SubjectKey: techKey}})
}

func techReading(t *testing.T, f *fx) contracts.ReadingRecord {
	t.Helper()
	rows, _ := f.LatestReadings(context.Background(), techName, "market", "NDX_SPX", dayD, dayD)
	if len(rows) != 1 {
		t.Fatalf("%d tech readings, want 1", len(rows))
	}
	return rows[0]
}

func techTag(r contracts.ReadingRecord, name string) (string, bool) {
	for _, p := range r.Parameters {
		if p.Param == name && p.ValueTag != nil {
			return *p.ValueTag, true
		}
	}
	return "", false
}

func TestWithTechAddsOneKey(t *testing.T) {
	t.Parallel()
	j, _, _ := techWorld(t, &techFake{}, newIndexes())
	f := world()
	st := f.stores()
	st.IndexIDs, st.PriceBars = newIndexes(), &fakeBars{}
	plain, err0 := New(st, realFigs(), rules.New(), own.NewUnits(), own.NewSeriesOps(), clockwork.NewFakeClock())
	if err0 != nil {
		t.Fatal(err0)
	}
	keys, hash, err := j.Plan(context.Background(), dayD)
	pk, ph, _ := plain.Plan(context.Background(), dayD)
	if err != nil || len(keys) != len(pk)+1 || hash == ph {
		t.Fatalf("keys %d vs %d, err %v, hashes equal %v", len(keys), len(pk), err, hash == ph)
	}
	n := 0
	for _, k := range keys {
		if k.SubjectKey == techKey && k.DatasetID == datasetReadings {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("tech key listed %d times", n)
	}
}

func TestWithTechNilClassifyIsSameJob(t *testing.T) {
	t.Parallel()
	f, b := world(), &fakeBars{}
	st := f.stores()
	st.IndexIDs, st.PriceBars = newIndexes(), b
	j, _ := New(st, realFigs(), rules.New(), own.NewUnits(), own.NewSeriesOps(), clockwork.NewFakeClock())
	if got := WithTech(j, TechDeps{Params: testTechParams}); got != j {
		t.Fatal("job changed")
	}
}

func TestWithTechNoIndexReaderIsSameJob(t *testing.T) {
	t.Parallel()
	fake := &techFake{}
	for _, c := range []struct{ bars, idx bool }{{true, false}, {false, true}} {
		st := world().stores()
		if c.bars {
			st.PriceBars = &fakeBars{}
		}
		if c.idx {
			st.IndexIDs = newIndexes()
		}
		j, _ := New(st, realFigs(), rules.New(), own.NewUnits(), own.NewSeriesOps(), clockwork.NewFakeClock())
		if got := WithTech(j, TechDeps{Classify: fake.classify}); got != j {
			t.Fatalf("job changed with %+v", c)
		}
	}
}

type foreignJob struct{ contracts.Job }

func TestWithTechForeignJobIsSameJob(t *testing.T) {
	t.Parallel()
	j := foreignJob{}
	if got := WithTech(j, TechDeps{Classify: (&techFake{}).classify}); got != contracts.Job(j) {
		t.Fatal("foreign job changed")
	}
}

func TestTechStoresRankStateAndEvidence(t *testing.T) {
	t.Parallel()
	fake := &techFake{res: okTech()}
	j, f, _ := techWorld(t, fake, newIndexes())
	if res := runTech(j); res.Err != nil || res.Usable != 1 {
		t.Fatalf("result %+v", res)
	}
	r := techReading(t, f)
	if r.State != "ok" || r.Value == nil || *r.Value != "-2" || r.Version != 1 || r.Origin != "compute_figures" ||
		r.Unit != "text_tag" {
		t.Fatalf("reading %+v", r)
	}
	want := map[string]string{"state": "LAGGING HARD", "raw": "LAGGING", "data_day": string(dayD), "ndx_1m": "-1.00",
		"spx_1m": "2.00", "gap_1m": "-3.00", "ndx_3m": "-4.00", "spx_3m": "3.00", "gap_3m": "-7.00", "band": "1.00",
		"hard": "5.00", "sessions_1m": "21", "sessions_3m": "63", "trial": "true"}
	for k, v := range want {
		if got, ok := techTag(r, k); !ok || got != v {
			t.Errorf("tag %s = %q (%v), want %q", k, got, ok, v)
		}
	}
	if len(r.Inputs) != 1 || r.Inputs[0].InputTable != "obs.price_bar" || len(r.Inputs[0].Keys) != 2 ||
		r.Inputs[0].Keys[0] != "12|trades" || r.Inputs[0].Keys[1] != "11|trades" ||
		r.Inputs[0].ConventionVersion != "tech_state_v1" {
		t.Fatalf("inputs %+v", r.Inputs)
	}
}

func TestTechStoresRawDays(t *testing.T) {
	t.Parallel()
	fake := &techFake{res: okTech()}
	j, f, _ := techWorld(t, fake, newIndexes())
	runTech(j)
	if got, ok := techTag(techReading(t, f), "raw_days"); !ok || got != "7" {
		t.Fatalf("raw_days %q %v", got, ok)
	}
}

func TestTechStoresGapAsMissingInput(t *testing.T) {
	t.Parallel()
	fake := &techFake{res: blockapi.TechResult{Reason: compute.ShortHistory, DataDay: dayD, State: "LEADING", Rank: 1}}
	j, f, _ := techWorld(t, fake, newIndexes())
	if res := runTech(j); res.Err != nil || res.Usable != 1 {
		t.Fatalf("result %+v", res)
	}
	r := techReading(t, f)
	if r.State != "missing_input" || r.Value != nil || r.ReasonCode == nil || *r.ReasonCode != 2 {
		t.Fatalf("reading %+v", r)
	}
	if _, ok := techTag(r, "state"); ok {
		t.Fatal("a gap carries a state tag")
	}
}

func TestTechSameDayTwiceStoresOnce(t *testing.T) {
	t.Parallel()
	j, f, _ := techWorld(t, &techFake{res: okTech()}, newIndexes())
	runTech(j)
	runTech(j)
	if f.inserts != 1 {
		t.Fatalf("%d inserts, want 1", f.inserts)
	}
}

func TestTechUnknownIndexIsRetryable(t *testing.T) {
	t.Parallel()
	idx := newIndexes()
	idx.rows = idx.rows[:1] // SPX only
	fake := &techFake{res: okTech()}
	j, f, _ := techWorld(t, fake, idx)
	res := runTech(j)
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || fake.calls != 0 || len(f.readings) != 0 {
		t.Fatalf("result %+v calls %d stored %d", res, fake.calls, len(f.readings))
	}
}

func TestTechAmbiguousIndexIsPermanent(t *testing.T) {
	t.Parallel()
	idx := newIndexes()
	idx.rows = append(idx.rows, store.InstrumentName{InstrumentID: 99, Type: "index", Name: "NDX"})
	fake := &techFake{res: okTech()}
	j, f, _ := techWorld(t, fake, idx)
	res := runTech(j)
	if res.Err == nil || res.Err.Kind() != contracts.Permanent || !strings.Contains(res.Err.Error(), "ambiguous") ||
		fake.calls != 0 || len(f.readings) != 0 {
		t.Fatalf("result %+v calls %d", res, fake.calls)
	}
}

func TestTechReaderErrorIsRetryable(t *testing.T) {
	t.Parallel()
	idx := newIndexes()
	idx.err = errors.New("db down")
	fake := &techFake{res: okTech()}
	j, f, _ := techWorld(t, fake, idx)
	res := runTech(j)
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || fake.calls != 0 || len(f.readings) != 0 {
		t.Fatalf("result %+v", res)
	}
}

func TestTechReadsOneHundredFortyDays(t *testing.T) {
	t.Parallel()
	fake := &techFake{res: okTech()}
	j, _, b := techWorld(t, fake, newIndexes())
	runTech(j)
	if len(b.calls) != 2 {
		t.Fatalf("%d bar reads", len(b.calls))
	}
	for _, c := range b.calls {
		if c.basis != "trades" || c.from != shiftDay(dayD, -140) || c.to != dayD || (c.id != ndxID && c.id != spxID) {
			t.Fatalf("read %+v", c)
		}
	}
	in := fake.in[0]
	if in.Day != dayD || in.NDX.Name != "NDX" || in.SPX.Name != "SPX" || in.Params != testTechParams ||
		len(in.NDX.Closes) != 3 || in.NDX.Days[0] != dayD || in.NDX.Closes[0] != "100.52" {
		t.Fatalf("classify input %+v", in)
	}
}

func TestTechRowsHashIsTheCloses(t *testing.T) {
	t.Parallel()
	j, f, _ := techWorld(t, &techFake{res: okTech()}, newIndexes())
	runTech(j)
	var lines []string
	for i := 0; i < 3; i++ {
		d := string(shiftDay(dayD, i-2))
		lines = append(lines, "NDX|"+d+"|100.5"+strconv.Itoa(i), "SPX|"+d+"|50.2"+strconv.Itoa(i))
	}
	if got := techReading(t, f).RowsSha256; got != hashLines(lines) {
		t.Fatalf("rows_sha256 %s, want %s", got, hashLines(lines))
	}
}
