//go:build pg

package instrument

import (
	"context"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/stockreg"
)

// seedInstrument inserts an instrument of the type and currency with an ibkr_conid identifier (none when conID is 0).
func seedInstrument(t *testing.T, c *pgx.Conn, typ, name, currency string, conID int64) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	err := c.QueryRow(ctx, `INSERT INTO master.instrument (instrument_type, name, currency, mic, calendar_id, status, price_increment, lot_size, recorded_at)
VALUES ($1, $2, $3, '', 'XNYS', 'active', 0.01, 1, now()) RETURNING instrument_id`, typ, name, currency).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	if conID > 0 {
		if _, err := c.Exec(ctx, `INSERT INTO master.instrument_identifier (instrument_id, scheme, value, valid_from) VALUES ($1, 'ibkr_conid', $2, now())`, id, strconv.FormatInt(conID, 10)); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

// seedSetting inserts one version of a name of settings.underlying, set days ago.
func seedSetting(t *testing.T, c *pgx.Conn, nameKey, market, exchange, currency, name, issuer string, conID *int64, daysAgo int) {
	t.Helper()
	_, err := c.Exec(context.Background(), `INSERT INTO settings.underlying (name_key, symbol, market, kind, exchange, currency, name, issuer, con_id, status, set_at)
VALUES ($1, $1, $2, 'stock', $3, $4, $5, $6, $7, 'active', now() - make_interval(days => $8))`,
		nameKey, market, exchange, currency, name, issuer, conID, daysAgo)
	if err != nil {
		t.Fatal(err)
	}
}

func conIDp(v int64) *int64 { return &v }

type stockRow struct {
	name, issuerName string
	market, exchange *string
	issuerID         *int64
}

func readStocks(t *testing.T, c *pgx.Conn) map[int64]stockRow {
	t.Helper()
	rows, err := c.Query(context.Background(), `SELECT instrument_id, name, market, primary_exchange, issuer_id, COALESCE(issuer_name, '') FROM rd.stock`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[int64]stockRow{}
	for rows.Next() {
		var id int64
		var r stockRow
		if err := rows.Scan(&id, &r.name, &r.market, &r.exchange, &r.issuerID, &r.issuerName); err != nil {
			t.Fatal(err)
		}
		out[id] = r
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func str(p *string) string {
	if p == nil {
		return "<NULL>"
	}
	return *p
}

func TestBackfillStockRowsPG(t *testing.T) {
	name := newDatabaseBefore0044(t)
	owner := connectAs(t, name, "")
	apple := seedInstrument(t, owner, "stock", "AAPL", "USD", 1)
	toyota := seedInstrument(t, owner, "stock", "7203", "JPY", 2)
	chf := seedInstrument(t, owner, "stock", "NESN", "CHF", 3)
	apple2 := seedInstrument(t, owner, "etf", "AAPB", "USD", 4)
	bad := seedInstrument(t, owner, "stock", "BAD", "USD", 5)
	index := seedInstrument(t, owner, "index", "SPX", "USD", 0)
	seedSetting(t, owner, "AAPL", "US", "NASDAQ", "USD", "Apple Inc", "cik:0000320193", conIDp(1), 1)
	seedSetting(t, owner, "7203", "JP", "SMART", "JPY", "Toyota Motor", "edinet:E02144", conIDp(2), 1)
	seedSetting(t, owner, "AAPB", "US", "ARCA", "USD", "Apple Leveraged", "cik:0000320193", conIDp(4), 1)
	seedSetting(t, owner, "BAD", "US", "NYSE", "USD", "Bad Issuer Co", "cik:12", conIDp(5), 1)
	applyMigration(t, connURL(t, name, ""), "0044_instrument_company.sql")

	got := readStocks(t, owner)
	if len(got) != 6 {
		t.Fatalf("%d stock rows, want 6 (stock, etf and index)", len(got))
	}
	if r := got[apple]; r.name != "Apple Inc" || str(r.market) != "US" || str(r.exchange) != "NASDAQ" || r.issuerName != "Apple Inc" {
		t.Errorf("apple = %+v", r)
	}
	if r := got[toyota]; r.name != "Toyota Motor" || str(r.market) != "JP" || r.exchange != nil || r.issuerName != "Toyota Motor" {
		t.Errorf("toyota = %+v (SMART is no exchange)", r)
	}
	if r := got[chf]; r.name != "NESN" || r.market != nil || r.issuerID != nil {
		t.Errorf("chf = %+v: want the instrument name, NULL market and no issuer", r)
	}
	if got[apple].issuerID == nil || got[apple2].issuerID == nil || *got[apple].issuerID != *got[apple2].issuerID {
		t.Errorf("two instruments with one cik: issuers %v and %v", got[apple].issuerID, got[apple2].issuerID)
	}
	if r := got[bad]; r.issuerID != nil || r.name != "Bad Issuer Co" {
		t.Errorf("malformed issuer text: %+v", r)
	}
	if r := got[index]; r.name != "SPX" || r.issuerID != nil {
		t.Errorf("index = %+v", r)
	}
	var n int
	if err := owner.QueryRow(context.Background(), `SELECT count(*) FROM master.issuer`).Scan(&n); err != nil || n != 2 {
		t.Errorf("issuers = %d, %v; want 2 (one cik, one edinet code)", n, err)
	}
	var iss *int64
	if err := owner.QueryRow(context.Background(), `SELECT issuer_id FROM master.instrument WHERE instrument_id = $1`, apple).Scan(&iss); err != nil || iss == nil || *iss != *got[apple].issuerID {
		t.Errorf("instrument issuer_id = %v, %v; want the stock's", iss, err)
	}
}

func TestBackfillTieAndNewestPG(t *testing.T) {
	name := newDatabaseBefore0044(t)
	owner := connectAs(t, name, "")
	tie := seedInstrument(t, owner, "stock", "TIE", "USD", 10)
	older := seedInstrument(t, owner, "stock", "OLDER", "USD", 11)
	ver := seedInstrument(t, owner, "stock", "VER", "USD", 12)
	seedSetting(t, owner, "ZZ", "US", "NYSE", "USD", "Name of ZZ", "", conIDp(10), 1)
	seedSetting(t, owner, "AA", "US", "NYSE", "USD", "Name of AA", "", conIDp(10), 1)
	// the older version of OLD carries con_id 11, the newest has none: the older never wins
	seedSetting(t, owner, "OLD", "US", "NYSE", "USD", "Old version", "", conIDp(11), 5)
	seedSetting(t, owner, "OLD", "US", "NYSE", "USD", "New version", "", nil, 1)
	seedSetting(t, owner, "VER", "US", "NYSE", "USD", "v1", "", conIDp(12), 5)
	seedSetting(t, owner, "VER", "US", "NYSE", "USD", "v2", "", conIDp(12), 1)
	applyMigration(t, connURL(t, name, ""), "0044_instrument_company.sql")
	got := readStocks(t, owner)
	if got[tie].name != "Name of AA" {
		t.Errorf("tie = %q, want the smaller name_key AA", got[tie].name)
	}
	if got[older].name != "OLDER" {
		t.Errorf("older = %q, want the instrument name", got[older].name)
	}
	if got[ver].name != "v2" {
		t.Errorf("ver = %q, want the newest version", got[ver].name)
	}
}

func TestBackfillSymbolPG(t *testing.T) {
	name := newDatabaseBefore0044(t)
	owner := connectAs(t, name, "")
	stk := seedInstrument(t, owner, "stock", "AAPL", "USD", 1)
	withOSI := seedInstrument(t, owner, "option", "an option", "USD", 0)
	noOSI := seedInstrument(t, owner, "option", "other option", "USD", 0)
	if _, err := owner.Exec(context.Background(), `INSERT INTO master.instrument_identifier (instrument_id, scheme, value, valid_from) VALUES ($1, 'osi', 'AAPL  261120C00200000', now())`, withOSI); err != nil {
		t.Fatal(err)
	}
	applyMigration(t, connURL(t, name, ""), "0044_instrument_company.sql")
	want := map[int64]string{stk: "AAPL", withOSI: "AAPL  261120C00200000", noOSI: "other option"}
	for id, sym := range want {
		var got string
		if err := owner.QueryRow(context.Background(), `SELECT symbol FROM master.instrument WHERE instrument_id = $1`, id).Scan(&got); err != nil || got != sym {
			t.Errorf("symbol of %d = %q, %v; want %q", id, got, err, sym)
		}
	}
}

func TestSchemeListAndCurrentRowPG(t *testing.T) {
	name := newDatabase(t)
	owner := connectAs(t, name, "")
	id := seedInstrument(t, owner, "stock", "AAPL", "USD", 1)
	const ins = `INSERT INTO master.instrument_identifier (instrument_id, scheme, value, valid_from, valid_to) VALUES ($1, $2, $3, now(), $4)`
	ctx := context.Background()
	for _, s := range []string{"cusip", "figi", "sym_fred", "sym_alphavantage", "sym_jquants", "sym_sec", "sym_eodhd", "isin"} {
		if _, err := owner.Exec(ctx, ins, id, s, "v-"+s, nil); err != nil {
			t.Errorf("scheme %s refused: %v", s, err)
		}
	}
	mustFail(t, owner, ins, id, "bogus", "x", nil)
	mustFail(t, owner, ins, id, "cusip", "another", nil) // a second current row of one scheme
	if _, err := owner.Exec(ctx, ins, id, "cusip", "closed", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Errorf("a closed row of the scheme: %v", err)
	}
}

func TestResolveUnknownCurrencyStockRowPG(t *testing.T) {
	name := newDatabase(t)
	s := openMaster(t, name)
	id, err := s.ResolveInstrument(context.Background(), store.Identifier{Scheme: "ibkr_conid", Value: "77"},
		store.InstrumentSpec{Type: "stock", Name: "NESN", Currency: "CHF", CalendarID: "XNYS", PriceIncrement: "0.01", LotSize: "1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	owner := connectAs(t, name, "")
	var sym string
	var market *string
	err = owner.QueryRow(context.Background(), `SELECT i.symbol, s.market FROM master.instrument i JOIN master.stock s USING (instrument_id) WHERE i.instrument_id = $1`, id).Scan(&sym, &market)
	if err != nil || sym != "NESN" || market != nil {
		t.Fatalf("symbol %q, market %v, %v; want NESN and NULL", sym, market, err)
	}
}

func TestStockAndOptionViewsPG(t *testing.T) {
	name := newDatabase(t)
	owner := connectAs(t, name, "")
	ctx := context.Background()
	stk := seedInstrument(t, owner, "stock", "AAPL", "USD", 1)
	opt := seedInstrument(t, owner, "option", "AAPL C", "USD", 0)
	bare := seedInstrument(t, owner, "option", "AAPL P", "USD", 0)
	if _, err := owner.Exec(ctx, `INSERT INTO master.stock (instrument_id, name, market) VALUES ($1, 'Apple Inc', 'US')`, stk); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, `UPDATE master.instrument SET symbol = 'AAPL' WHERE instrument_id = $1`, stk); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, `INSERT INTO master.option_contract (instrument_id, underlying_id, currency, exchange, trading_class, multiplier, expiry, strike, "right")
VALUES ($1, $2, 'USD', 'SMART', 'AAPL', 100, '2026-11-20', '200', 'C')`, opt, stk); err != nil {
		t.Fatal(err)
	}
	cols := func(q string) []string {
		rows, err := owner.Query(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for _, f := range rows.FieldDescriptions() {
			out = append(out, f.Name)
		}
		return out
	}
	wantStock := []string{"instrument_id", "symbol", "name", "market", "primary_exchange", "currency", "mic", "sector_fund", "issuer_id", "issuer_name"}
	if got := cols(`SELECT * FROM rd.stock`); !slices.Equal(got, wantStock) {
		t.Errorf("rd.stock columns %v", got)
	}
	wantOption := []string{"instrument_id", "symbol", "currency", "underlying_id", "underlying_symbol", "expiry", "strike", "right", "multiplier", "trading_class", "exchange"}
	if got := cols(`SELECT * FROM rd.option`); !slices.Equal(got, wantOption) {
		t.Errorf("rd.option columns %v", got)
	}
	var usym *string
	var uid *int64
	err := owner.QueryRow(ctx, `SELECT underlying_id, underlying_symbol FROM rd.option WHERE instrument_id = $1`, opt).Scan(&uid, &usym)
	if err != nil || uid == nil || *uid != stk || str(usym) != "AAPL" {
		t.Errorf("option with contract: %v %v, %v", uid, str(usym), err)
	}
	var n int
	err = owner.QueryRow(ctx, `SELECT count(*) FROM rd.option WHERE instrument_id = $1 AND underlying_id IS NULL AND underlying_symbol IS NULL AND expiry IS NULL AND strike IS NULL AND "right" IS NULL AND multiplier IS NULL AND trading_class IS NULL AND exchange IS NULL`, bare).Scan(&n)
	if err != nil || n != 1 {
		t.Errorf("option without option_contract: %d rows with NULL columns, %v", n, err)
	}
	if err := owner.QueryRow(ctx, `SELECT count(*) FROM rd.option`).Scan(&n); err != nil || n != 2 {
		t.Errorf("rd.option has %d rows, want the 2 options", n)
	}
}

// TestStockViewsGrantsAfter0044 reads both views as the page role and as the runner role, which hold no table grant.
func TestStockViewsGrantsAfter0044(t *testing.T) {
	name := newDatabase(t)
	owner := connectAs(t, name, "")
	ctx := context.Background()
	stk := seedInstrument(t, owner, "stock", "AAPL", "USD", 1)
	seedInstrument(t, owner, "option", "AAPL C", "USD", 0)
	if _, err := owner.Exec(ctx, `INSERT INTO master.stock (instrument_id, name, market) VALUES ($1, 'Apple Inc', 'US')`, stk); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"ot_ro_pages", "ot_runner"} {
		c := connectAs(t, name, role)
		for _, view := range []string{"rd.stock", "rd.option"} {
			var n int
			if err := c.QueryRow(ctx, `SELECT count(*) FROM `+view).Scan(&n); err != nil || n != 1 {
				t.Errorf("%s reads %s: %d rows, %v; want 1", role, view, n, err)
			}
		}
		mustFail(t, c, `SELECT count(*) FROM master.stock`)
		mustFail(t, c, `SELECT count(*) FROM master.issuer`)
	}
}

func TestRegisterStockAsMasterRolePG(t *testing.T) {
	name := newDatabase(t)
	s := openMaster(t, name)
	reg, ok := s.(stockreg.StockRegistrar)
	if !ok {
		t.Fatal("the store has no RegisterStock")
	}
	dir := s.(stockreg.StockDirectory)
	ctx := context.Background()
	r := stockreg.Registration{ConID: 265598, Kind: "stock", Symbol: "AAPL", Market: "US", PrimaryExchange: "NASDAQ", Name: "Apple Inc",
		Currency: "USD", CalendarID: "XNYS", Issuer: &stockreg.Issuer{Name: "Apple Inc", Country: "US", CIK: "0000320193"}}
	id, err := reg.RegisterStock(ctx, r)
	if err != nil || id == 0 {
		t.Fatalf("register: %d, %v", id, err)
	}
	again, err := reg.RegisterStock(ctx, r)
	if err != nil || again != id {
		t.Fatalf("second register: %d, %v; want %d", again, err, id)
	}
	r.Issuer.Name = "Apple Inc."
	r.SectorFund = "Technology"
	if _, err := reg.RegisterStock(ctx, r); err != nil {
		t.Fatal(err)
	}
	owner := connectAs(t, name, "")
	got := readStocks(t, owner)
	if len(got) != 1 || got[id].issuerName != "Apple Inc." || str(got[id].exchange) != "NASDAQ" || got[id].issuerID == nil {
		t.Fatalf("stocks %+v", got)
	}
	var sector, sym string
	var inst *int64
	err = owner.QueryRow(ctx, `SELECT s.sector_fund, i.symbol, i.issuer_id FROM master.stock s JOIN master.instrument i USING (instrument_id)`).Scan(&sector, &sym, &inst)
	if err != nil || sector != "Technology" || sym != "AAPL" || inst == nil || *inst != *got[id].issuerID {
		t.Fatalf("sector %q symbol %q issuer %v, %v", sector, sym, inst, err)
	}
	found, ok, err := dir.LookupStock(ctx, "AAPL", "US")
	if err != nil || !ok || found != id {
		t.Fatalf("LookupStock = %d %v %v", found, ok, err)
	}
	if _, ok, err := dir.LookupStock(ctx, "AAPL", "JP"); err != nil || ok {
		t.Fatalf("LookupStock in JP = %v %v", ok, err)
	}
	if v, ok, err := dir.Identifier(ctx, id, "ibkr_conid"); err != nil || !ok || v != "265598" {
		t.Fatalf("Identifier = %q %v %v", v, ok, err)
	}
	if _, ok, err := dir.Identifier(ctx, id, "cusip"); err != nil || ok {
		t.Fatalf("Identifier cusip = %v %v", ok, err)
	}
	w := connectAs(t, name, "ot_w_master")
	mustFail(t, w, `DELETE FROM master.stock`)
	mustFail(t, w, `DELETE FROM master.issuer`)
	mustFail(t, w, `TRUNCATE master.stock`)
	mustFail(t, w, `UPDATE master.instrument SET name = 'x'`)
	mustFail(t, w, `UPDATE master.instrument_identifier SET valid_to = now()`)
}

func TestOldInsertStillWorksAfter0044PG(t *testing.T) {
	name := newDatabase(t)
	w := connectAs(t, name, "ot_w_master")
	var id int64
	err := w.QueryRow(context.Background(), insertInstrumentSQL, "stock", "SPY", "USD", "", "XNYS", "active", "0.01", "1").Scan(&id)
	if err != nil {
		t.Fatalf("the old insert: %v", err)
	}
	var sym string
	if err := connectAs(t, name, "").QueryRow(context.Background(), `SELECT symbol FROM master.instrument WHERE instrument_id = $1`, id).Scan(&sym); err != nil || sym != "" {
		t.Fatalf("symbol %q, %v; want the default", sym, err)
	}
	// the old binary also wrote the identifier; the next resolve repairs the symbol and adds the stock row
	if _, err := connectAs(t, name, "ot_w_master").Exec(context.Background(), `INSERT INTO master.instrument_identifier (instrument_id, scheme, value, valid_from) VALUES ($1, 'ibkr_conid', '1', now())`, id); err != nil {
		t.Fatal(err)
	}
	s := openMaster(t, name)
	if got, err := s.ResolveInstrument(context.Background(), store.Identifier{Scheme: "ibkr_conid", Value: "1"},
		store.InstrumentSpec{Type: "stock", Name: "SPY", Currency: "USD", CalendarID: "XNYS", PriceIncrement: "0.01", LotSize: "1"}, nil); err != nil || got != id {
		t.Fatalf("resolve = %d, %v; want %d", got, err, id)
	}
	if got := readStocks(t, connectAs(t, name, "")); got[id].name != "SPY" || str(got[id].market) != "US" {
		t.Fatalf("stock rows %+v", got)
	}
}
