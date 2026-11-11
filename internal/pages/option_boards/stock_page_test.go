package option_boards

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden files")

const (
	stockDay  = contracts.Day("2026-11-10")
	gatesPass = `[{"gate":"quote","status":"pass","reason":""},{"gate":"events","status":"pass","reason":""}]`
	gatesThin = `[{"gate":"quote","status":"unknown","reason":"no_ask"},{"gate":"events","status":"block","reason":"earnings_before_expiry"}]`
)

// underOf is the underlying id of an issuer text of the fixtures: 7 for Apple, 8 for Microsoft.
func underOf(issuer string) int {
	if issuer == issuerMsft {
		return 8
	}
	return 7
}

func sPtr(s string) *string { return &s }
func iPtr(i int) *int       { return &i }

func stockRowOf(id int, issuer, expiry string, dte int, strike, status string, rank, inU *int, gates string) store.Candidate {
	return store.Candidate{
		SessionDay: stockDay, Kind: "short_put", InstrumentID: id, Issuer: issuer, UnderlyingID: underOf(issuer), Expiry: contracts.Day(expiry), DTE: dte,
		Right: "put", Strike: strike, Multiplier: 100, BidEod: "1.20", Ask: sPtr("1.30"), Spot: sPtr("68"), BreakEven: "60.8",
		AnnualizedYield: sPtr("0.123456"), Delta: sPtr("-0.15"), Iv: sPtr("0.38"), Status: status, Gates: gates, Marks: "[]",
		NextEarnings: nil, EarningsState: "absent", Rank: rank, RankInUnderlying: inU,
		ComputedAt: time.Date(2026, 11, 10, 6, 30, 0, 0, time.UTC),
	}
}

func stockFixture() []store.Candidate {
	return []store.Candidate{
		stockRowOf(102, issuerApple, "2026-11-20", 10, "62", "ok", iPtr(2), iPtr(2), gatesPass),
		stockRowOf(101, issuerApple, "2026-11-13", 3, "65", "ok", iPtr(1), iPtr(1), gatesPass),
		stockRowOf(103, issuerApple, "2026-11-13", 3, "60", "blocked", nil, nil, gatesThin),
		stockRowOf(201, issuerMsft, "2026-11-13", 3, "300", "ok", iPtr(3), iPtr(1), gatesPass),
	}
}

func stockEod() *countingEod {
	return &countingEod{rows: map[int64][]contracts.OptionEod{7: {
		{
			InstrumentID: 101, SessionDay: sdate("2026-11-09"), CaptureID: "a", Volume: fp(250), OpenInterest: fp(1200),
			ThetaPerCalendarDay: fp(-0.02), Gamma: fp(0.031), VegaPerVolPoint: fp(0.05),
		},
		{InstrumentID: 102, SessionDay: sdate("2026-11-09"), CaptureID: "a", Volume: fp(10)},
	}}}
}

func stockCfg(day contracts.Day, rows []store.Candidate, key string) StockConfig {
	return StockConfig{
		RunID: "run1", BuildID: "b1", Scope: "stock-options", Zone: time.FixedZone("CST", 8*3600),
		Candidates: &fakeCands{byDay: map[contracts.Day][]store.Candidate{day: rows}},
		Eod:        stockEod(), Names: tickerNames(), Key: key,
	}
}

func buildStockPayload(t *testing.T, c StockConfig, day contracts.Day) (stockPayload, []byte) {
	t.Helper()
	b, err := BuildStock(context.Background(), c, nil, day, time.Date(2026, 11, 10, 3, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var p stockPayload
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return p, b
}

func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Indent(&buf, got, "", "  "); err != nil {
		t.Fatal(err)
	}
	buf.WriteByte('\n')
	path := "testdata/" + name + ".golden.json"
	if *updateGolden {
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, buf.Bytes()) {
		t.Errorf("%s differs from the golden file; run with -update to see the change\n%s", name, buf.String())
	}
}

func TestStockGolden(t *testing.T) {
	t.Parallel()
	_, b := buildStockPayload(t, stockCfg(stockDay, stockFixture(), "AAPL"), stockDay)
	checkGolden(t, "stock_option_page_ok", b)
}

func TestStockNoKeyGolden(t *testing.T) {
	t.Parallel()
	p, b := buildStockPayload(t, stockCfg(stockDay, stockFixture(), "NOPE"), stockDay)
	checkGolden(t, "stock_option_page_no_key", b)
	if p.NoCard == nil || *p.NoCard != "No candidate rows for this underlying" || p.RunStatus != "partial" || p.Stale != nil || len(p.Matrix.Cells) != 0 {
		t.Errorf("unexpected empty page: %+v", p)
	}
}

func TestBareKeyIsFirstOkRow(t *testing.T) {
	t.Parallel()
	rows := stockFixture()
	rows[0].Status, rows[1].Status, rows[2].Status = "blocked", "blocked", "blocked"
	p, _ := buildStockPayload(t, stockCfg(stockDay, rows, ""), stockDay)
	if p.Top.Symbol != "MSFT" {
		t.Errorf("bare key with one ok row: symbol = %q, want MSFT", p.Top.Symbol)
	}
	rows[0].Status = "unknown"
	rows[3].Status = "blocked"
	p, _ = buildStockPayload(t, stockCfg(stockDay, rows, ""), stockDay)
	if p.Top.Symbol != "AAPL" {
		t.Errorf("bare key with no ok row: symbol = %q, want the first row's AAPL", p.Top.Symbol)
	}
}

func TestMatrixOrder(t *testing.T) {
	t.Parallel()
	p, _ := buildStockPayload(t, stockCfg(stockDay, stockFixture(), "AAPL"), stockDay)
	if got := strings.Join(p.Matrix.Strikes, ","); got != "60,62,65" {
		t.Errorf("strikes = %s", got)
	}
	if len(p.Matrix.Expiries) != 2 || p.Matrix.Expiries[0].Expiry != "2026-11-13" || p.Matrix.Expiries[0].Text != "11-13 Fri · 3 days" {
		t.Errorf("expiries = %+v", p.Matrix.Expiries)
	}
	if len(p.Matrix.Cells) != 3 || p.Matrix.Cells[0].Key != "c102" {
		t.Errorf("cells = %+v", p.Matrix.Cells)
	}
	rows := stockFixture()
	rows[0].Strike = "9.5"
	rows[1].Strike = "10"
	p, _ = buildStockPayload(t, stockCfg(stockDay, rows, "AAPL"), stockDay)
	if got := strings.Join(p.Matrix.Strikes, ","); got != "9.5,10,60" {
		t.Errorf("strikes compare as decimals, got %s", got)
	}
}

func TestMatrixFlagsFromGates(t *testing.T) {
	t.Parallel()
	p, _ := buildStockPayload(t, stockCfg(stockDay, stockFixture(), "AAPL"), stockDay)
	byKey := map[string]matrixCell{}
	for _, c := range p.Matrix.Cells {
		byKey[c.Key] = c
	}
	if c := byKey["c101"]; c.Shade != "worth" || !c.Star || !c.Chosen || c.Thin || c.Earnings || c.Out {
		t.Errorf("c101 = %+v", c)
	}
	if c := byKey["c102"]; c.Shade != "worth" || c.Star || c.Chosen {
		t.Errorf("c102 = %+v", c)
	}
	if c := byKey["c103"]; c.Shade != "none" || !c.Thin || !c.Earnings || c.Chosen || c.Out {
		t.Errorf("c103 = %+v", c)
	}
	checkFlagTexts(t, p, byKey)
}

func checkFlagTexts(t *testing.T, p stockPayload, byKey map[string]matrixCell) {
	t.Helper()
	if c := byKey["c101"]; c.YieldText != "Annualised 12.3456%" || c.Premium != "Premium 1.20" || c.Delta != "Δ −0.15" || c.Volume != "Volume 250" {
		t.Errorf("c101 texts = %+v", c)
	}
	if byKey["c103"].Volume != "Volume n/a" {
		t.Errorf("c103 volume = %q", byKey["c103"].Volume)
	}
	if len(p.Cards) != 2 || !p.Cards[1].Chosen || p.Cards[0].Chosen || p.Cards[0].Title != "put 62 11-20" {
		t.Errorf("cards = %+v", p.Cards)
	}
}

func TestContractsElevenColumns(t *testing.T) {
	t.Parallel()
	p, _ := buildStockPayload(t, stockCfg(stockDay, stockFixture(), "AAPL"), stockDay)
	if len(p.Contracts.Columns) != 11 || p.Contracts.Columns[6] != "Θ/day" {
		t.Fatalf("columns = %v", p.Contracts.Columns)
	}
	if len(p.Contracts.Rows) != 3 {
		t.Fatalf("rows = %d", len(p.Contracts.Rows))
	}
	for _, r := range p.Contracts.Rows {
		if len(r.Cells) != 11 {
			t.Errorf("row %s has %d cells", r.Expiry, len(r.Cells))
		}
	}
	got := strings.Join(p.Contracts.Rows[1].Cells, "|")
	if want := "11-13|65|1.20|1.30|−0.15|38%|-0.02|0.031|0.05|250|12.3456%"; got != want {
		t.Errorf("row 1 = %s, want %s", got, want)
	}
	if !strings.Contains(strings.Join(p.Contracts.Rows[2].Cells, "|"), "n/a|n/a|n/a|n/a") {
		t.Errorf("row without eod = %v", p.Contracts.Rows[2].Cells)
	}
}

func TestMatrixCutAtLimits(t *testing.T) {
	t.Parallel()
	var rows []store.Candidate
	for i := 0; i < 105; i++ {
		rows = append(rows, stockRowOf(1000+i, issuerApple, "2026-11-13", 3, fmt.Sprint(10+i), "ok", nil, nil, gatesPass))
	}
	for i := 0; i < 32; i++ {
		d := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i).Format("2006-01-02")
		rows = append(rows, stockRowOf(2000+i, issuerApple, d, 21+i, "10", "ok", nil, nil, gatesPass))
	}
	p, _ := buildStockPayload(t, stockCfg(stockDay, rows, "AAPL"), stockDay)
	if len(p.Matrix.Strikes) != 100 || p.Matrix.Strikes[99] != "109" {
		t.Errorf("strikes = %d, last %s", len(p.Matrix.Strikes), p.Matrix.Strikes[len(p.Matrix.Strikes)-1])
	}
	if len(p.Matrix.Expiries) != 30 {
		t.Errorf("expiries = %d", len(p.Matrix.Expiries))
	}
	// 5 strikes over the limit, then 2 expiries over the limit
	if len(p.Matrix.Cells) != 100+29 {
		t.Errorf("cells = %d", len(p.Matrix.Cells))
	}
	if p.Notice == nil || !strings.Contains(*p.Notice, "Matrix over the limit, 8 contracts not shown") {
		t.Errorf("notice = %v", p.Notice)
	}
	if len(p.Maps) != 30 || len(p.Cards) != 10 || len(p.Contracts.Rows) != 137 {
		t.Errorf("maps %d cards %d contract rows %d", len(p.Maps), len(p.Cards), len(p.Contracts.Rows))
	}
}

func TestStockStaleLabel(t *testing.T) {
	t.Parallel()
	prev := contracts.Day("2026-11-09")
	rows := stockFixture()
	for i := range rows {
		rows[i].SessionDay = prev
	}
	p, _ := buildStockPayload(t, stockCfg(prev, rows, "AAPL"), stockDay)
	if p.Stale == nil || p.Stale.Text != "Data day 2026-11-09, before today" || p.Stale.StaleAfter != "2026-11-09T23:59:59+08:00" ||
		p.Stale.DataTime != "2026-11-10T14:30:00+08:00" {
		t.Errorf("stale = %+v", p.Stale)
	}
	if p.RunStatus != "partial" {
		t.Errorf("run status = %s", p.RunStatus)
	}
	fresh, _ := buildStockPayload(t, stockCfg(stockDay, stockFixture(), "AAPL"), stockDay)
	if fresh.Stale != nil || fresh.RunStatus != "ok" {
		t.Errorf("fresh day: stale %+v status %s", fresh.Stale, fresh.RunStatus)
	}
}

func TestStockZoneRequired(t *testing.T) {
	t.Parallel()
	c := stockCfg(stockDay, stockFixture(), "AAPL")
	c.Zone = nil
	if _, err := BuildStock(context.Background(), c, nil, stockDay, time.Time{}); err == nil {
		t.Error("nil zone gives no error")
	}
}

func TestPremiumTextMissingBidIsGap(t *testing.T) {
	t.Parallel()
	if got := premiumText(""); got != "Premium n/a" {
		t.Errorf("empty bid: %q", got)
	}
	if got := premiumText("n/a"); got != "Premium n/a" {
		t.Errorf("n/a bid: %q", got)
	}
	if got := premiumText("1.20"); got != "Premium 1.20" {
		t.Errorf("bid 1.20: %q", got)
	}
}

func TestStockEmptyBidIsGap(t *testing.T) {
	t.Parallel()
	rows := stockFixture()
	rows[1].BidEod = ""
	rows[1].AnnualizedYield = sPtr("0")
	p, _ := buildStockPayload(t, stockCfg(stockDay, rows, "AAPL"), stockDay)
	found := false
	for _, c := range p.Matrix.Cells {
		if c.Key == cellKey(101) {
			found = true
			if c.YieldText != "Annualised n/a" || c.Premium != "Premium n/a" {
				t.Errorf("matrix cell = %q, %q", c.YieldText, c.Premium)
			}
		}
	}
	if !found {
		t.Fatal("cell 101 missing")
	}
	cells := p.Contracts.Rows[1].Cells
	if cells[2] != "n/a" || cells[10] != "n/a" {
		t.Errorf("contract cells = %v", cells)
	}
}
