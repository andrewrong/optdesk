package instrument

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/stockreg"
)

// stockModel answers the reads of RegisterStock and of the symbol repair from fixed state.
type stockModel struct {
	emptySymbol string  // answer of emptySymbolSQL: the type of the instrument, "" for no row
	issuers     [][]any // rows of issuerSQL: id, name, country, cik, edinet_code (nil for NULL)
	state       []any   // the row of stockStateSQL, nil for none
	repaired    int
}

func (m *stockModel) answer(sql string, _ []any, scan func(scanner) error) (bool, error) {
	switch sql {
	case emptySymbolSQL:
		if m.emptySymbol != "" {
			typ := m.emptySymbol
			m.emptySymbol = "" // the first write fills it: the second call finds it filled
			m.repaired++
			return true, scan(valsScanner{typ})
		}
	case issuerSQL:
		for _, r := range m.issuers {
			if err := scan(valsScanner(r)); err != nil {
				return true, err
			}
		}
	case insertIssuerSQL:
		return true, scan(valsScanner{int64(55)})
	case stockStateSQL:
		if m.state != nil {
			return true, scan(valsScanner(m.state))
		}
	}
	return true, nil
}

func modelFake(m *stockModel) *fakeDB {
	f := newFake()
	f.answer = m.answer
	return f
}

var writeSQL = []string{insertInstrumentSQL, insertIdentifierSQL, setSymbolSQL, insertStockSQL, insertIssuerSQL,
	updateIssuerSQL, setInstrumentSQL, upsertStockSQL}

func writes(f *fakeDB) []string {
	var out []string
	for _, q := range f.stmts {
		if slices.Contains(writeSQL, q) {
			out = append(out, q)
		}
	}
	return out
}

// argsOf returns the arguments of the first statement equal to sql.
func argsOf(t *testing.T, f *fakeDB, sql string) []any {
	t.Helper()
	for i, q := range f.stmts {
		if q == sql {
			return f.args[i]
		}
	}
	t.Fatalf("statement not sent: %s", sql)
	return nil
}

func stockSpec() store.InstrumentSpec {
	return store.InstrumentSpec{Type: "stock", Name: "SPY", Currency: "USD", CalendarID: "XNYS", PriceIncrement: "0.01", LotSize: "1"}
}

func TestResolveNewStockInsertsStockRowAfterInstrument(t *testing.T) {
	t.Parallel()
	f := newFake()
	if _, err := (&instrumentStore{db: f}).ResolveInstrument(context.Background(), keyID, stockSpec(), nil); err != nil {
		t.Fatal(err)
	}
	got := writes(f)
	want := []string{insertInstrumentSQL, setSymbolSQL, insertIdentifierSQL, insertStockSQL}
	if !slices.Equal(got, want) {
		t.Fatalf("writes %q, want %q", got, want)
	}
	if a := argsOf(t, f, setSymbolSQL); a[0] != int64(101) || a[1] != "SPY" {
		t.Fatalf("symbol args %v", a)
	}
	if a := argsOf(t, f, insertStockSQL); a[0] != int64(101) || a[1] != "SPY" || a[2] != "US" {
		t.Fatalf("stock args %v", a)
	}
}

func TestResolveNewStockUnknownCurrencyHasNoMarket(t *testing.T) {
	t.Parallel()
	f := newFake()
	spec := stockSpec()
	spec.Currency = "CHF"
	if _, err := (&instrumentStore{db: f}).ResolveInstrument(context.Background(), keyID, spec, nil); err != nil {
		t.Fatal(err)
	}
	if a := argsOf(t, f, insertStockSQL); a[2] != nil {
		t.Fatalf("market %v, want nil", a[2])
	}
}

func TestResolveNewOptionSymbolFromOSIWithoutStockRow(t *testing.T) {
	t.Parallel()
	f := newFake()
	spec := goodSpec()
	spec.Name = "some name"
	if _, err := (&instrumentStore{db: f}).ResolveInstrument(context.Background(), keyID, spec, []store.Identifier{extraO}); err != nil {
		t.Fatal(err)
	}
	if a := argsOf(t, f, setSymbolSQL); a[1] != extraO.Value {
		t.Fatalf("symbol %v, want the osi identifier", a[1])
	}
	if slices.Contains(f.stmts, insertStockSQL) {
		t.Fatal("an option got a stock row")
	}
}

func TestResolveExistingStockWithEmptySymbolFilledOnce(t *testing.T) {
	t.Parallel()
	m := &stockModel{emptySymbol: "stock"}
	f := modelFake(m)
	f.ids[keyID] = 7
	s := &instrumentStore{db: f}
	for range 2 {
		if _, err := s.ResolveInstrument(context.Background(), keyID, stockSpec(), nil); err != nil {
			t.Fatal(err)
		}
	}
	if got := writes(f); !slices.Equal(got, []string{setSymbolSQL, insertStockSQL}) {
		t.Fatalf("writes %q: want one symbol and one stock row", got)
	}
}

func TestResolveExistingOptionIsNotTouched(t *testing.T) {
	t.Parallel()
	m := &stockModel{emptySymbol: "option"}
	f := modelFake(m)
	f.ids[keyID] = 7
	if _, err := (&instrumentStore{db: f}).ResolveInstrument(context.Background(), keyID, goodSpec(), nil); err != nil {
		t.Fatal(err)
	}
	if got := writes(f); len(got) != 0 || m.repaired != 0 {
		t.Fatalf("writes %q", got)
	}
}

func TestResolveDottedJPYSymbolIsPermanent(t *testing.T) {
	t.Parallel()
	f := newFake()
	spec := stockSpec()
	spec.Name, spec.Currency = "7203.T", "JPY"
	_, err := (&instrumentStore{db: f}).ResolveInstrument(context.Background(), keyID, spec, nil)
	assertPermanent(t, err)
	if len(f.stmts) != 0 {
		t.Fatalf("statements sent: %q", f.stmts)
	}
}

func registration() stockreg.Registration {
	return stockreg.Registration{ConID: 12345, Kind: "stock", Symbol: "AAPL", Market: "US", PrimaryExchange: "NASDAQ",
		Name: "Apple Inc", Currency: "USD", CalendarID: "XNYS",
		Issuer: &stockreg.Issuer{Name: "Apple Inc", Country: "US", CIK: "0000320193"}}
}

func TestRegisterStockFindsIssuerByCIK(t *testing.T) {
	t.Parallel()
	m := &stockModel{issuers: [][]any{{int64(9), "Apple Inc", "US", "0000320193", nil}},
		state: []any{"AAPL", nil, int64(7), "AAPL", "US", nil, nil, nil}}
	f := modelFake(m)
	f.ids[keyID] = 7
	id, err := (&instrumentStore{db: f}).RegisterStock(context.Background(), registration())
	if err != nil || id != 7 {
		t.Fatalf("id %d, %v", id, err)
	}
	if slices.Contains(f.stmts, insertIssuerSQL) || slices.Contains(f.stmts, updateIssuerSQL) {
		t.Fatalf("the issuer was written: %q", f.stmts)
	}
	if a := argsOf(t, f, setInstrumentSQL); a[2] != int64(9) {
		t.Fatalf("instrument issuer %v, want 9", a[2])
	}
	a := argsOf(t, f, upsertStockSQL)
	if a[1] != "Apple Inc" || a[2] != "US" || a[3] != "NASDAQ" || a[4] != int64(9) || a[5] != nil {
		t.Fatalf("stock args %v", a)
	}
}

func TestRegisterStockCreatesIssuerWhenAbsent(t *testing.T) {
	t.Parallel()
	m := &stockModel{state: []any{"AAPL", nil, int64(7), "AAPL", "US", nil, nil, nil}}
	f := modelFake(m)
	f.ids[keyID] = 7
	if _, err := (&instrumentStore{db: f}).RegisterStock(context.Background(), registration()); err != nil {
		t.Fatal(err)
	}
	a := argsOf(t, f, insertIssuerSQL)
	if a[0] != "Apple Inc" || a[1] != "US" || a[2] != "0000320193" || a[3] != nil {
		t.Fatalf("issuer args %v", a)
	}
	if a := argsOf(t, f, upsertStockSQL); a[4] != int64(55) {
		t.Fatalf("stock issuer %v, want 55", a[4])
	}
}

func TestRegisterStockEqualInputSendsNoWrite(t *testing.T) {
	t.Parallel()
	m := &stockModel{issuers: [][]any{{int64(9), "Apple Inc", "US", "0000320193", nil}},
		state: []any{"AAPL", int64(9), int64(7), "Apple Inc", "US", "NASDAQ", int64(9), nil}}
	f := modelFake(m)
	f.ids[keyID] = 7
	if _, err := (&instrumentStore{db: f}).RegisterStock(context.Background(), registration()); err != nil {
		t.Fatal(err)
	}
	if got := writes(f); len(got) != 0 {
		t.Fatalf("writes %q for equal input", got)
	}
}

func TestRegisterStockOverwritesIssuerName(t *testing.T) {
	t.Parallel()
	m := &stockModel{issuers: [][]any{{int64(9), "Old Name", "US", "0000320193", nil}},
		state: []any{"AAPL", int64(9), int64(7), "Apple Inc", "US", "NASDAQ", int64(9), nil}}
	f := modelFake(m)
	f.ids[keyID] = 7
	if _, err := (&instrumentStore{db: f}).RegisterStock(context.Background(), registration()); err != nil {
		t.Fatal(err)
	}
	if a := argsOf(t, f, updateIssuerSQL); a[0] != int64(9) || a[1] != "Apple Inc" {
		t.Fatalf("update args %v", a)
	}
}

func TestRegisterStockTwoIssuersIsPermanentWithoutWrite(t *testing.T) {
	t.Parallel()
	m := &stockModel{issuers: [][]any{{int64(3), "A", "US", "0000320193", nil}, {int64(4), "B", "JP", nil, "E02144"}}}
	f := modelFake(m)
	f.ids[keyID] = 7
	r := registration()
	r.Issuer.EdinetCode = "E02144"
	_, err := (&instrumentStore{db: f}).RegisterStock(context.Background(), r)
	assertPermanent(t, err)
	if !strings.Contains(err.Error(), "issuer 3") || !strings.Contains(err.Error(), "issuer 4") {
		t.Fatalf("error %q does not name both ids", err)
	}
	if got := writes(f); len(got) != 0 {
		t.Fatalf("writes %q", got)
	}
}

func TestRegisterStockChecks(t *testing.T) {
	t.Parallel()
	for name, mut := range map[string]func(*stockreg.Registration){
		"con id":   func(r *stockreg.Registration) { r.ConID = 0 },
		"kind":     func(r *stockreg.Registration) { r.Kind = "option" },
		"symbol":   func(r *stockreg.Registration) { r.Symbol = "" },
		"market":   func(r *stockreg.Registration) { r.Market = "XX" },
		"no code":  func(r *stockreg.Registration) { r.Issuer = &stockreg.Issuer{Name: "x", Country: "US"} },
		"jpy dot":  func(r *stockreg.Registration) { r.Currency, r.Symbol, r.Market = "JPY", "7203.T", "JP" },
		"currency": func(r *stockreg.Registration) { r.Currency = "usd" },
	} {
		f := newFake()
		r := registration()
		mut(&r)
		_, err := (&instrumentStore{db: f}).RegisterStock(context.Background(), r)
		assertPermanent(t, err)
		if len(f.stmts) != 0 {
			t.Errorf("%s: statements sent %q", name, f.stmts)
		}
	}
}

func TestLookupStockOneRow(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.rows = [][]any{{int64(5)}}
	id, ok, err := (&instrumentStore{db: f}).LookupStock(context.Background(), "AAPL", "US")
	if err != nil || !ok || id != 5 {
		t.Fatalf("got %d %v %v", id, ok, err)
	}
}

func TestLookupStockNotFound(t *testing.T) {
	t.Parallel()
	_, ok, err := (&instrumentStore{db: newFake()}).LookupStock(context.Background(), "AAPL", "US")
	if err != nil || ok {
		t.Fatalf("ok %v, err %v", ok, err)
	}
}

func TestLookupStockTwoRowsNamesBothIDs(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.rows = [][]any{{int64(5)}, {int64(6)}}
	_, _, err := (&instrumentStore{db: f}).LookupStock(context.Background(), "AAPL", "US")
	assertPermanent(t, err)
	if !strings.Contains(err.Error(), "5") || !strings.Contains(err.Error(), "6") {
		t.Fatalf("error %q does not name both ids", err)
	}
}

func TestLookupStockNoMarket(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.rows = [][]any{{int64(5)}}
	id, ok, err := (&instrumentStore{db: f}).LookupStock(context.Background(), "AAPL", "")
	if err != nil || ok || id != 0 {
		t.Fatalf("got %d %v %v, want not found and no error", id, ok, err)
	}
	if len(f.selSQL) != 0 {
		t.Fatal("a statement was sent for an empty market")
	}
}

func TestIdentifier(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := &instrumentStore{db: f}
	if _, ok, err := s.Identifier(context.Background(), 5, "cusip"); err != nil || ok {
		t.Fatalf("missing: ok %v, err %v", ok, err)
	}
	f.rows = [][]any{{"037833100"}}
	v, ok, err := s.Identifier(context.Background(), 5, "cusip")
	if err != nil || !ok || v != "037833100" {
		t.Fatalf("got %q %v %v", v, ok, err)
	}
}
