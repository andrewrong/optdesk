package positions_page_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/compute/pagefig"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/positions_page"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

var (
	zone = time.FixedZone("owner", 8*3600)
	at   = time.Date(2026, 11, 10, 2, 0, 0, 0, time.UTC)
)

const day = contracts.Day("2026-11-10")

type fakeAccount struct {
	store.AccountReader
	positions map[string][]store.Position // by alias|day
	execs     []store.Execution
	failOn    string
}

func (f *fakeAccount) Positions(_ context.Context, alias, d string) ([]store.Position, error) {
	if alias == f.failOn {
		return nil, errors.New("boom")
	}
	return f.positions[alias+"|"+d], nil
}

func (f *fakeAccount) Executions(_ context.Context, _, from, to string) ([]store.Execution, error) {
	var out []store.Execution
	for _, e := range f.execs {
		if e.SessionDay >= from && e.SessionDay <= to {
			out = append(out, e)
		}
	}
	return out, nil
}

type fakeMarks struct{ rows []mark.PositionMark }

func (f fakeMarks) Marks(context.Context, string, string) ([]mark.PositionMark, error) {
	return f.rows, nil
}

type fakeLedger struct{}

func (fakeLedger) FlexTradeRows(context.Context, string) ([]store.FlexTradeRow, error) {
	return []store.FlexTradeRow{{TransactionID: "t1", Revision: 1, Attributes: `{"symbol":"XYZ"}`, RecordedAt: at}}, nil
}
func (fakeLedger) LatestOkFetch(context.Context, string, string) (store.FlexFetchRecord, bool, error) {
	return store.FlexFetchRecord{}, false, nil
}
func (fakeLedger) StoredAliases(context.Context) ([]string, error) { return nil, nil }

type fakeSessions struct{}

func (fakeSessions) SessionDays(_ context.Context, _ string, from, to contracts.Day) ([]contracts.SessionDay, error) {
	f, _ := time.Parse("2006-01-02", string(from))
	e, _ := time.Parse("2006-01-02", string(to))
	var out []contracts.SessionDay
	for d := f; !d.After(e); d = d.AddDate(0, 0, 1) {
		out = append(out, contracts.SessionDay{CalendarID: "XNYS", SessionDay: types.SerializableDate{Time: d},
			IsSession: d.Weekday() != time.Saturday && d.Weekday() != time.Sunday})
	}
	return out, nil
}

type replayer struct{}

func (replayer) Replay(compute.LedgerInput, compute.LedgerParams) compute.LedgerResult {
	return compute.LedgerResult{Positions: []compute.LedgerPosition{
		{Underlying: "XYZ", Status: "open", Right: "P", Expiry: "2026-11-20", Strike: "100.0000", PremiumIncome: "258.0000"},
		{Underlying: "OLD", Status: "closed", Right: "P", Expiry: "2026-12-18", Strike: "50", PremiumIncome: "10"},
	}}
}

func pos(d, sym, sec, cur string, conID int64, qty, cost string) store.Position {
	p := store.Position{
		Alias: "main", SessionDay: d, Symbol: sym, SecType: sec, Currency: cur, ConID: conID, Quantity: qty, AvgCost: cost,
		Multiplier: "1", CapturedAt: time.Date(2026, 11, 10, 1, 0, 0, 0, time.UTC),
	}
	if sec == "option" {
		p.Multiplier = "100"
	}
	return p
}

func option(p store.Position, right, expiry, strike string) store.Position {
	p.Right, p.Expiry, p.Strike = right, expiry, strike
	return p
}

func account() *fakeAccount {
	return &fakeAccount{
		positions: map[string][]store.Position{
			"main|2026-11-10": {
				pos("2026-11-10", "AAA", "stock", "USD", 1, "200", "138.2"),
				pos("2026-11-10", "BBB", "stock", "USD", 2, "10", "12.5"),
				option(pos("2026-11-10", "XYZ", "option", "USD", 3, "-1", "2.6"), "put", "2026-11-20", "100"),
				option(pos("2026-11-10", "ZZZ", "option", "USD", 4, "1", "1.1"), "call", "2026-12-18", "50"),
			},
			"main|2026-11-09": {pos("2026-11-09", "AAA", "stock", "USD", 1, "200", "138.2")},
		},
		execs: []store.Execution{{
			Alias: "main", ExecID: "e1", SessionDay: "2026-11-10", Symbol: "XYZ", SecType: "option", Expiry: "2026-11-20",
			Strike: "100", Right: "put", Currency: "USD", Side: "SLD", Quantity: "1", Price: "2.6",
			ExecutedAt: time.Date(2026, 11, 10, 14, 30, 0, 0, time.UTC),
		}},
	}
}

func config(acc *fakeAccount) positions_page.Config {
	m := func(id int64, price string) mark.PositionMark {
		return mark.PositionMark{ConID: id, Price: price, Basis: mark.BasisMid}
	}
	return positions_page.Config{
		RunID: "run1", BuildID: "b1", Zone: zone, Aliases: []string{"main", "wife"},
		Account: acc, Marks: fakeMarks{rows: []mark.PositionMark{m(1, "151.4"), m(3, "3.15"), m(4, "1.0")}},
		Ledger: fakeLedger{}, Sessions: fakeSessions{}, Ledgers: func() compute.OptionLedger { return replayer{} },
		Figures: pagefig.New, Holdings: pagefig.NewHoldings,
	}
}

func build(t *testing.T, c positions_page.Config, alias string) (map[string]any, []byte) {
	t.Helper()
	b, err := positions_page.NewPage(c, alias).Build(context.Background(), nil, day, at)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m, b
}

func at0(t *testing.T, v any, path ...any) any {
	t.Helper()
	for _, p := range path {
		switch k := p.(type) {
		case string:
			m, ok := v.(map[string]any)
			if !ok {
				t.Fatalf("no object at %q in %v", k, v)
			}
			v = m[k]
		case int:
			a, ok := v.([]any)
			if !ok || k >= len(a) {
				t.Fatalf("no element %d in %v", k, v)
			}
			v = a[k]
		}
	}
	return v
}

func TestMeta(t *testing.T) {
	t.Parallel()
	p := positions_page.NewPage(positions_page.Config{}, "main")
	if p.Meta.Name != "positions_page" || p.Meta.Schema != "page.positions_page.v3" {
		t.Fatalf("meta = %+v", p.Meta)
	}
}

type fileLoader struct{}

func (fileLoader) Load(url string) (any, error) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "schemas", filepath.Base(url)))
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}

// validate checks b against the schema with the views and role rules of issue 18 (one to eight views,
// any non-empty key, label and role text), so that it holds before and after that issue lands.
func validate(t *testing.T, b []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "schemas", "page.positions_page.v3.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	free := map[string]any{"type": "string", "minLength": 1, "maxLength": 200}
	defs := doc["$defs"].(map[string]any)
	view := defs["PositionsView"].(map[string]any)["properties"].(map[string]any)
	view["key"], view["label"] = free, free
	role := defs["RoleCell"].(map[string]any)["properties"].(map[string]any)
	role["text"], role["value"] = free, free
	views := doc["properties"].(map[string]any)["views"].(map[string]any)
	views["minItems"], views["maxItems"] = 1, 8

	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(fileLoader{})
	if err := c.AddResource("https://optdesk.invalid/schemas/page.positions_page.v3.json", doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile("https://optdesk.invalid/schemas/page.positions_page.v3.json")
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(v); err != nil {
		t.Fatalf("output is invalid: %v\n%s", err, b)
	}
}

func TestGolden(t *testing.T) {
	t.Parallel()
	_, b := build(t, config(account()), "main")
	validate(t, b)
	var out bytes.Buffer
	if err := json.Indent(&out, b, "", "  "); err != nil {
		t.Fatal(err)
	}
	out.WriteByte('\n')
	golden := filepath.Join("testdata", "ok.golden.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(golden, out.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, out.Bytes()) {
		t.Fatalf("output differs from %s:\n%s", golden, out.Bytes())
	}
}

func TestRowsCarryTheAccountLabelAndTheFigures(t *testing.T) {
	t.Parallel()
	m, _ := build(t, config(account()), "main")
	if k := at0(t, m, "views", 0, "key"); k != "main" {
		t.Fatalf("key = %v", k)
	}
	for _, p := range [][]any{{"views", 0, "stocks", "rows", 0, "role", "text"}, {"views", 0, "options", "rows", 0, "role", "text"}} {
		if r := at0(t, m, p...); r != "账户（main）" {
			t.Fatalf("role = %v", r)
		}
	}
	if u := at0(t, m, "views", 0, "stocks", "rows", 0, "unrealised", "text"); u != "+$2,640.00" {
		t.Fatalf("unrealised = %v", u)
	}
	if u := at0(t, m, "views", 0, "options", "rows", 0, "unrealised", "text"); u != "−$55.00" {
		t.Fatalf("option unrealised = %v", u)
	}
	if p := at0(t, m, "views", 0, "options", "rows", 0, "open_premium", "text"); p != "$258.00" {
		t.Fatalf("premium = %v", p)
	}
	if d := at0(t, m, "views", 0, "options", "rows", 0, "days_left", "value"); fmt.Sprint(d) != "10" {
		t.Fatalf("days left = %v", d)
	}
	if g := at0(t, m, "views", 0, "stocks", "rows", 0, "day", "reason"); g != "no_data" {
		t.Fatalf("day = %v", g)
	}
}

func TestPositionWithoutMarkIsAGapNotZero(t *testing.T) {
	t.Parallel()
	m, _ := build(t, config(account()), "main")
	for _, f := range []string{"price", "unrealised"} {
		if g := at0(t, m, "views", 0, "stocks", "rows", 1, f, "reason"); g != "missing_quote" {
			t.Fatalf("%s = %v", f, g)
		}
	}
}

func TestOptionWithoutLedgerMatchHasAGapPremium(t *testing.T) {
	t.Parallel()
	m, _ := build(t, config(account()), "main")
	if g := at0(t, m, "views", 0, "options", "rows", 1, "open_premium", "reason"); g != "no_data" {
		t.Fatalf("premium = %v", g)
	}
	if c := at0(t, m, "views", 0, "options", "rows", 1, "code", "text"); c != "ZZZ 12-18 C50" {
		t.Fatalf("code = %v", c)
	}
}

func TestTradesAreClassifiedByFigures(t *testing.T) {
	t.Parallel()
	m, _ := build(t, config(account()), "main")
	if a := at0(t, m, "views", 0, "trades", "rows", 0, "action", "text"); a != "卖出开仓" {
		t.Fatalf("action = %v", a)
	}
	if i := at0(t, m, "views", 0, "trades", "rows", 0, "instrument", "text"); i != "XYZ 2026-11-20 100 put" {
		t.Fatalf("instrument = %v", i)
	}
}

func TestGapPages(t *testing.T) {
	t.Parallel()
	none := map[string]func(*positions_page.Config){
		"account":  func(c *positions_page.Config) { c.Account = nil },
		"marks":    func(c *positions_page.Config) { c.Marks = nil },
		"ledger":   func(c *positions_page.Config) { c.Ledger = nil },
		"sessions": func(c *positions_page.Config) { c.Sessions = nil },
		"ledgers":  func(c *positions_page.Config) { c.Ledgers = nil },
		"figures":  func(c *positions_page.Config) { c.Figures = nil },
		"holdings": func(c *positions_page.Config) { c.Holdings = nil },
	}
	for name, unset := range none {
		t.Run("nil "+name, func(t *testing.T) {
			t.Parallel()
			c := config(account())
			unset(&c)
			m, b := build(t, c, "main")
			validate(t, b)
			if g := at0(t, m, "gaps", 0, "reason"); g != "no_reader" {
				t.Fatalf("gap = %v", g)
			}
			if g := at0(t, m, "views", 0, "stocks", "gap", "reason"); g != "no_reader" {
				t.Fatalf("stocks gap = %v", g)
			}
		})
	}
	t.Run("no capture", func(t *testing.T) {
		t.Parallel()
		m, b := build(t, config(account()), "wife")
		validate(t, b)
		if g := at0(t, m, "gaps", 0, "reason"); g != "no_data" {
			t.Fatalf("gap = %v", g)
		}
		if k := at0(t, m, "views", 0, "key"); k != "wife" {
			t.Fatalf("key = %v", k)
		}
	})
	t.Run("stale capture", func(t *testing.T) {
		t.Parallel()
		acc := account()
		acc.positions = map[string][]store.Position{"main|2026-10-30": acc.positions["main|2026-11-09"]}
		m, b := build(t, config(acc), "main")
		validate(t, b)
		if g := at0(t, m, "gaps", 0, "reason"); g != "stale_input" {
			t.Fatalf("gap = %v", g)
		}
	})
	t.Run("unknown alias", func(t *testing.T) {
		t.Parallel()
		m, b := build(t, config(account()), "stranger")
		validate(t, b)
		if g := at0(t, m, "gaps", 0, "reason"); g != "no_data" {
			t.Fatalf("gap = %v", g)
		}
	})
}

func TestReaderErrorIsAnError(t *testing.T) {
	t.Parallel()
	acc := account()
	acc.failOn = "main"
	if _, err := positions_page.NewPage(config(acc), "main").Build(context.Background(), nil, day, at); err == nil {
		t.Fatal("want an error")
	}
	c := config(account())
	c.Zone = nil
	if _, err := positions_page.NewPage(c, "main").Build(context.Background(), nil, day, at); err == nil {
		t.Fatal("want an error without a zone")
	}
}

var _ blockapi.PageFigures = pagefig.New()
