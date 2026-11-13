package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/computefigures"
	"github.com/andrewrong/optdesk/internal/jobs/optioneod"
	"github.com/andrewrong/optdesk/internal/store"
)

// noFredJobs is the job list of a scheduler wired without the fred key.
var noFredJobs = []string{jobCalendarRefresh, jobComputeFigures, jobEarningsRefresh, jobOptionEod, jobOptionEodIndex, jobQuoteSnapshot}

type idxBars struct {
	rows     []contracts.PriceBar
	id       int64
	basis    string
	from, to contracts.Day
}

func (b *idxBars) PriceBars(_ context.Context, id int64, basis string, from, to contracts.Day) ([]contracts.PriceBar, error) {
	b.id, b.basis, b.from, b.to = id, basis, from, to
	return b.rows, nil
}

type idxIDs struct{ got []store.InstrumentName }

func (r idxIDs) IndexIDs(context.Context, []string) ([]store.InstrumentName, error) {
	return r.got, nil
}

type idxJob struct{ contracts.Job }

func (idxJob) ID() string { return jobOptionEodIndex }

// idxCtor records what the family passes to the constructor.
type idxCtor struct {
	calls int
	rd    optioneod.IndexCloseReader
	err   error
}

func (c *idxCtor) New(_ broker.Broker, _ store.InstrumentResolver, _ store.OptionEodWriter,
	rd optioneod.IndexCloseReader, _ clock.Clock) (contracts.Job, error) {
	c.calls++
	c.rd = rd
	return idxJob{}, c.err
}

func idxLog() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewTextHandler(&buf, nil)), &buf
}

func idxFactory(bars *idxBars, ids computefigures.IndexIDReader) jobFactory {
	f := jobFactory{clk: fakeClock{time.Date(2026, 11, 9, 16, 0, 0, 0, time.UTC)}}
	if bars != nil {
		f.compute.PriceBars = bars
	}
	if ids != nil {
		f.compute.IndexIDs = ids
	}
	return f
}

func TestIndexFamilyIsRegisteredAtOrder75(t *testing.T) {
	t.Parallel()
	fam, ok := jobFamilies[jobOptionEodIndex]
	if !ok || fam.order != 75 {
		t.Fatalf("family %+v registered %v", fam, ok)
	}
	if got := fam.startOff(runOnceConfig{}, false); got != reasonBrokerDown {
		t.Fatalf("startOff with the broker down: %q", got)
	}
	if got := fam.startOff(runOnceConfig{}, true); got != "" {
		t.Fatalf("startOff with the broker up: %q", got)
	}
}

func TestIndexFamilyRunsWithoutBarReader(t *testing.T) {
	t.Parallel()
	c := &idxCtor{}
	log, buf := idxLog()
	if _, err := idxFactory(nil, idxIDs{}).optionEodIndex(nil, c.New, log); err != nil {
		t.Fatal(err)
	}
	if c.calls != 1 || c.rd != nil {
		t.Fatalf("constructor calls %d reader %v", c.calls, c.rd)
	}
	if n := strings.Count(buf.String(), "level=WARN msg=\"option_eod_index runs without closes\""); n != 1 {
		t.Fatalf("log %q", buf.String())
	}
}

func TestIndexFamilyRunsWithoutIndexIDReader(t *testing.T) {
	t.Parallel()
	c := &idxCtor{}
	log, buf := idxLog()
	if _, err := idxFactory(&idxBars{}, nil).optionEodIndex(nil, c.New, log); err != nil {
		t.Fatal(err)
	}
	if c.calls != 1 || c.rd != nil {
		t.Fatalf("constructor calls %d reader %v", c.calls, c.rd)
	}
	if n := strings.Count(buf.String(), "option_eod_index runs without closes"); n != 1 {
		t.Fatalf("log %q", buf.String())
	}
}

func TestIndexFamilyBuildsThroughTheGivenConstructor(t *testing.T) {
	t.Parallel()
	c := &idxCtor{}
	log, buf := idxLog()
	j, err := idxFactory(&idxBars{}, idxIDs{}).optionEodIndex(nil, c.New, log)
	if err != nil || j == nil || j.ID() != jobOptionEodIndex {
		t.Fatalf("job %v err %v", j, err)
	}
	if c.calls != 1 || c.rd == nil || buf.Len() != 0 {
		t.Fatalf("calls %d reader %v log %q", c.calls, c.rd, buf.String())
	}
	boom := &idxCtor{err: errors.New("boom")}
	if _, err := idxFactory(&idxBars{}, idxIDs{}).optionEodIndex(nil, boom.New, log); err == nil {
		t.Fatal("a constructor error was dropped")
	}
}

func TestIndexFamilyBuildsWithNilStoreOrClock(t *testing.T) {
	t.Parallel()
	fam := jobFamilies[jobOptionEodIndex]
	log, buf := idxLog()
	f := jobFactory{}
	got, reason, err := fam.build(context.Background(), &app{log: log}, &jobCtx{f: &f})
	if err != nil || got != nil || reason != "" {
		t.Fatalf("family build %v %q err %v: a nil store must not stop the wiring", got, reason, err)
	}
	if !strings.Contains(buf.String(), "job left out") {
		t.Fatalf("log %q", buf.String())
	}
	// The probe of a factory without stores answers a reason and no error either.
	p := &prober{log: log, w: wiring{factory: f, dial: func(context.Context) (broker.Broker, error) { return struct{ broker.Broker }{}, nil }}}
	if j, r := p.probe(context.Background(), jobOptionEodIndex); j != nil || r != reasonBuildJob {
		t.Fatalf("probe %v %q", j, r)
	}
}

func TestIndexClosesResolveTheNameAndReturnTheCloses(t *testing.T) {
	t.Parallel()
	bars := &idxBars{rows: []contracts.PriceBar{{Close: "5000.10"}, {Close: "5001.20"}, {Close: "4999.00"}}}
	rd := indexCloses{bars: bars, ids: idxIDs{got: []store.InstrumentName{{InstrumentID: 77, Type: "index", Name: "SPX"}}}}
	got, err := rd.IndexCloses(context.Background(), "SPX", "2026-11-01", "2026-11-09")
	if err != nil || !slices.Equal(got, []string{"5000.10", "5001.20", "4999.00"}) {
		t.Fatalf("closes %v err %v", got, err)
	}
	if bars.id != 77 || bars.basis != "trades" || bars.from != "2026-11-01" || bars.to != "2026-11-09" {
		t.Fatalf("bar query %+v", bars)
	}
}

func TestIndexClosesUnknownNameIsAnError(t *testing.T) {
	t.Parallel()
	rd := indexCloses{bars: &idxBars{}, ids: idxIDs{}}
	if _, err := rd.IndexCloses(context.Background(), "SPX", "2026-11-01", "2026-11-09"); err == nil {
		t.Fatal("unknown name gave no error")
	}
}

func TestIndexClosesAmbiguousNameIsAnError(t *testing.T) {
	t.Parallel()
	rd := indexCloses{bars: &idxBars{}, ids: idxIDs{got: []store.InstrumentName{{InstrumentID: 1, Name: "SPX"}, {InstrumentID: 2, Name: "SPX"}}}}
	if _, err := rd.IndexCloses(context.Background(), "SPX", "2026-11-01", "2026-11-09"); err == nil {
		t.Fatal("ambiguous name gave no error")
	}
}

func TestIndexProbeCaseAnswersBrokerDown(t *testing.T) {
	t.Parallel()
	log, _ := idxLog()
	down := &prober{log: log, w: wiring{dial: func(context.Context) (broker.Broker, error) { return nil, errors.New("no socket") }}}
	if j, r := down.probe(context.Background(), jobOptionEodIndex); j != nil || r != reasonBrokerDown {
		t.Fatalf("broker down: %v %q", j, r)
	}
	up := &prober{log: log, w: wiring{
		factory: jobFactory{clk: fakeClock{time.Date(2026, 11, 9, 16, 0, 0, 0, time.UTC)}, inst: conidInst{}, eod: conidEod{}},
		dial:    func(context.Context) (broker.Broker, error) { return struct{ broker.Broker }{}, nil },
	}}
	j, r := up.probe(context.Background(), jobOptionEodIndex)
	if j == nil || r != "" || j.ID() != jobOptionEodIndex {
		t.Fatalf("broker up: %v %q", j, r)
	}
}
