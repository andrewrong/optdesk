package main

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/buypriceapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store/underlying"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// bpReader is a valuation reader that counts its reads and answers one state or one error.
type bpReader struct {
	valuation.Reader
	st    valuation.State
	err   error
	mu    sync.Mutex
	reads int
}

func (r *bpReader) Current(context.Context) (valuation.State, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reads++
	return r.st, r.err
}

func (r *bpReader) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reads
}

// bpResolver gives the valuation price 91.12 to a symbol that has a zone, else its hand price.
type bpResolver struct{}

func (bpResolver) Resolve(in buypriceapi.Input) buypriceapi.Result {
	for _, z := range in.Zones {
		if z.Symbol == in.Symbol {
			return buypriceapi.Result{Price: "91.12", Source: buypriceapi.SourceValuation}
		}
	}
	if in.Hand != "" {
		return buypriceapi.Result{Price: in.Hand, Source: buypriceapi.SourceHand}
	}
	return buypriceapi.Result{Source: buypriceapi.SourceNone}
}

func bpState(symbol string) valuation.State {
	return valuation.State{Entries: []valuation.Entry{{Profile: valuation.Profile{Symbol: symbol}}}}
}

func bpRow() underlying.Row {
	r := activeRow("AAA")
	r.Issuer = "AAA"
	r.BuyPrice = "90.00"
	r.Options = true
	return r
}

// bpFactory is a factory over one list row with a log into buf.
func bpFactory(buf *bytes.Buffer, rd valuation.Reader) jobFactory {
	f := jobFactory{valuation: rd, buy: bpResolver{}, log: slog.New(slog.NewTextHandler(buf, nil))}
	if rd == nil {
		f.valuation = nil
	}
	return f.withList(bpRow())
}

// bpCapture builds a family of group and returns the entries the newest inner job was built with.
func bpCapture(f jobFactory, group string) (contracts.Job, *[][]jobs.WatchEntry, error) {
	var built [][]jobs.WatchEntry
	j, err := f.listed(group, func(es []jobs.WatchEntry) (contracts.Job, error) {
		built = append(built, es)
		return &fakeInner{runs: new([]*fakeInner), tag: "x"}, nil
	})
	return j, &built, err
}

func TestBuyPricePlanReadsValuationNotStart(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	rd := &bpReader{st: bpState("AAA")}
	j, built, err := bpCapture(bpFactory(&buf, rd), groupOptions)
	if err != nil {
		t.Fatal(err)
	}
	if rd.count() != 0 {
		t.Fatalf("start read the valuation %d times", rd.count())
	}
	if e := (*built)[0][0]; e.BuyPrice != "90.00" || e.BuySource != buypriceapi.SourceHand {
		t.Errorf("start entry %q %q", e.BuyPrice, e.BuySource)
	}
	if n := strings.Count(buf.String(), "buyprice: start builds with hand prices, each Plan reads the valuation"); n != 1 {
		t.Errorf("start line %d times: %s", n, buf.String())
	}
	if _, _, err := j.Plan(context.Background(), "2026-11-11"); err != nil {
		t.Fatal(err)
	}
	if rd.count() != 1 {
		t.Errorf("reads %d after one Plan", rd.count())
	}
	if e := (*built)[1][0]; e.BuyPrice != "91.12" || e.BuySource != buypriceapi.SourceValuation {
		t.Errorf("plan entry %q %q", e.BuyPrice, e.BuySource)
	}
}

func TestBuyPricePlanReadErrorIsRetryable(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	rd := &bpReader{err: errors.New("boom")}
	j, _, err := bpCapture(bpFactory(&buf, rd), groupOptions)
	if err != nil {
		t.Fatalf("start failed for the valuation: %v", err)
	}
	_, _, err = j.Plan(context.Background(), "2026-11-11")
	var le *listedError
	if !errors.As(err, &le) || le.Kind() != contracts.Retryable {
		t.Fatalf("err %v", err)
	}
	if !strings.Contains(err.Error(), "valuation") || strings.Contains(err.Error(), "postgres") {
		t.Errorf("text %q", err.Error())
	}
}

func TestBuyPriceBarsNeverReadValuation(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	rd := &bpReader{err: errors.New("boom")}
	j, built, err := bpCapture(bpFactory(&buf, rd), groupBars)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := j.Plan(context.Background(), "2026-11-11"); err != nil {
		t.Fatalf("bars Plan: %v", err)
	}
	if rd.count() != 0 {
		t.Errorf("reads %d", rd.count())
	}
	if e := (*built)[1][0]; e.BuyPrice != "90.00" || e.BuySource != "" {
		t.Errorf("bars entry changed: %q %q", e.BuyPrice, e.BuySource)
	}
}

func TestBuyPriceNoReaderKeepsHand(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	j, built, err := bpCapture(bpFactory(&buf, nil), groupOptions)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, _, err := j.Plan(context.Background(), "2026-11-11"); err != nil {
			t.Fatal(err)
		}
	}
	for _, es := range *built {
		if es[0].BuyPrice != "90.00" || es[0].BuySource != buypriceapi.SourceHand {
			t.Errorf("entry %q %q", es[0].BuyPrice, es[0].BuySource)
		}
	}
	if n := strings.Count(buf.String(), "buyprice: no valuation reader, hand prices only"); n != 1 {
		t.Errorf("line %d times: %s", n, buf.String())
	}
}

func TestBuyPriceOptionFamiliesGetOnePrice(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	f := bpFactory(&buf, &bpReader{st: bpState("AAA")})
	var got [2]jobs.WatchEntry
	for i, id := range []string{jobOptionEod, jobOptionOpen} {
		j, built, err := bpCapture(f, groupOptions)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if _, _, err := j.Plan(context.Background(), "2026-11-11"); err != nil {
			t.Fatal(err)
		}
		got[i] = (*built)[len(*built)-1][0]
	}
	if got[0].BuyPrice != got[1].BuyPrice || got[0].BuySource != got[1].BuySource || got[0].BuySource != buypriceapi.SourceValuation {
		t.Errorf("eod %q %q, open %q %q", got[0].BuyPrice, got[0].BuySource, got[1].BuyPrice, got[1].BuySource)
	}
}

type bpCloser struct{ closed *bool }

func (c bpCloser) Close() error { *c.closed = true; return nil }

func TestOpenJobValuation(t *testing.T) {
	t.Parallel()
	t.Run("opens with the pages line and gives its closer to extra", func(t *testing.T) {
		t.Parallel()
		var dsn string
		closed := false
		rd := &bpReader{}
		a := &app{}
		a.deps.openValuation = func(d string) (valuation.Reader, valuation.Writer, io.Closer, error) {
			dsn = d
			return rd, nil, bpCloser{&closed}, nil
		}
		f, extra := &jobFactory{}, &closerSet{}
		if err := openJobValuation(a, runOnceConfig{pagesDSN: "postgres://pages", runnerDSN: "postgres://runner"}, f, extra); err != nil {
			t.Fatal(err)
		}
		extra.closeAll()
		if dsn != "postgres://pages" || f.valuation != valuation.Reader(rd) || f.buy == nil || !closed {
			t.Errorf("dsn %q valuation %v buy %v closed %v", dsn, f.valuation, f.buy, closed)
		}
	})
	t.Run("an open error holds no DSN", func(t *testing.T) {
		t.Parallel()
		a := &app{}
		a.deps.openValuation = func(d string) (valuation.Reader, valuation.Writer, io.Closer, error) {
			return nil, nil, nil, errors.New("dial " + d)
		}
		err := openJobValuation(a, runOnceConfig{pagesDSN: "postgres://secret"}, &jobFactory{}, &closerSet{})
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Errorf("err %v", err)
		}
	})
	t.Run("no line or no opener gives no reader", func(t *testing.T) {
		t.Parallel()
		a := &app{}
		f := &jobFactory{}
		if err := openJobValuation(a, runOnceConfig{pagesDSN: "postgres://pages"}, f, &closerSet{}); err != nil || f.valuation != nil || f.buy == nil {
			t.Errorf("no opener: %v %v %v", err, f.valuation, f.buy)
		}
	})
}

func TestBuyPriceWiringHasNoLogic(t *testing.T) {
	t.Parallel()
	file, err := parser.ParseFile(token.NewFileSet(), "buyprice_wire.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			t.Errorf("buyprice_wire.go declares a loop")
		}
		return true
	})
}
