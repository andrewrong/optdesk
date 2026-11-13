package stockreg_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/andrewrong/optdesk/internal/store/stockreg"
)

// fake keeps the promises of the interfaces: equal input gives one id and
// changes nothing; a lookup that finds nothing says ok false, not an error.
type fake struct {
	byConID map[int64]int64
	regs    map[int64]stockreg.Registration
	idents  map[string]string // "id/scheme" -> value
	writes  int
}

var (
	_ stockreg.StockRegistrar = (*fake)(nil)
	_ stockreg.StockDirectory = (*fake)(nil)
)

func (f *fake) RegisterStock(_ context.Context, r stockreg.Registration) (int64, error) {
	if f.byConID == nil {
		f.byConID = map[int64]int64{}
		f.regs = map[int64]stockreg.Registration{}
		f.idents = map[string]string{}
	}
	id, ok := f.byConID[r.ConID]
	if !ok {
		id = int64(len(f.byConID) + 1)
		f.byConID[r.ConID] = id
		f.idents[fmt.Sprintf("%d/ibkr_conid", id)] = fmt.Sprint(r.ConID)
	}
	old, had := f.regs[id]
	if !had || old.Symbol != r.Symbol || old.Market != r.Market || old.Name != r.Name {
		f.regs[id] = r
		f.writes++
	}
	return id, nil
}

func (f *fake) LookupStock(_ context.Context, symbol, market string) (int64, bool, error) {
	for id, r := range f.regs {
		if r.Symbol == symbol && r.Market == market {
			return id, true, nil
		}
	}
	return 0, false, nil
}

func (f *fake) Identifier(_ context.Context, id int64, scheme string) (string, bool, error) {
	v, ok := f.idents[fmt.Sprintf("%d/%s", id, scheme)]
	return v, ok, nil
}

func TestStockRegistrarContract(t *testing.T) {
	t.Parallel()
	var reg stockreg.StockRegistrar = &fake{}
	r := stockreg.Registration{
		ConID: 265598, Kind: "stock", Symbol: "AAPL", Market: "US", Name: "Apple Inc.", Currency: "USD",
		Issuer: &stockreg.Issuer{Name: "Apple Inc.", Country: "US", CIK: "0000320193"},
	}
	a, err := reg.RegisterStock(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	b, err := reg.RegisterStock(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if a != b || a == 0 {
		t.Fatalf("ids %d and %d: equal input must give one nonzero id", a, b)
	}
	if f := reg.(*fake); f.writes != 1 {
		t.Fatalf("writes = %d, want 1: equal input changes nothing", f.writes)
	}
	dir := reg.(stockreg.StockDirectory)
	got, ok, err := dir.LookupStock(context.Background(), "AAPL", "US")
	if err != nil || !ok || got != a {
		t.Fatalf("read back = %d %v %v, want %d true nil", got, ok, err, a)
	}
}

func TestStockDirectoryContract(t *testing.T) {
	t.Parallel()
	f := &fake{}
	id, err := f.RegisterStock(context.Background(), stockreg.Registration{ConID: 14015, Kind: "stock", Symbol: "7203", Market: "JP"})
	if err != nil {
		t.Fatal(err)
	}
	var dir stockreg.StockDirectory = f
	ctx := context.Background()

	if got, ok, err := dir.LookupStock(ctx, "7203", "JP"); err != nil || !ok || got != id {
		t.Fatalf("found = %d %v %v, want %d true nil", got, ok, err, id)
	}
	if _, ok, err := dir.LookupStock(ctx, "7203", "US"); err != nil || ok {
		t.Fatalf("other market = ok %v err %v, want false nil", ok, err)
	}
	if v, ok, err := dir.Identifier(ctx, id, "ibkr_conid"); err != nil || !ok || v != "14015" {
		t.Fatalf("identifier = %q %v %v, want 14015 true nil", v, ok, err)
	}
	if v, ok, err := dir.Identifier(ctx, id, "figi"); err != nil || ok || v != "" {
		t.Fatalf("missing identifier = %q %v %v, want empty false nil", v, ok, err)
	}
}
