package option_boards

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

type fakeOpens struct {
	rows []contracts.OptionOpen
	err  error
	days []contracts.Day
}

func (f *fakeOpens) OptionOpen(_ context.Context, day contracts.Day) ([]contracts.OptionOpen, error) {
	f.days = append(f.days, day)
	return f.rows, f.err
}

func tp(t time.Time) *time.Time { return &t }

// openRows: 11 is an open pull row with a skew, 12 a refresh row, 21 an error row (no usable quote).
func openRows() []contracts.OptionOpen {
	return []contracts.OptionOpen{
		{InstrumentID: 11, Slot: "0945", Status: 1, MarketDataType: 1, Skew: true,
			Bid: fp(1.25), Ask: fp(1.35), Delta: fp(-0.16), IvDecimal: fp(0.4), Volume: fp(300),
			EventTs: tp(time.Date(2026, 11, 10, 14, 45, 12, 0, time.UTC))},
		{InstrumentID: 12, Slot: openquote.SlotRefresh, Status: 2, MarketDataType: 3, OutsideBand: true,
			Bid: fp(0.9), Ask: fp(1.1), ReceivedAt: time.Date(2026, 11, 10, 15, 2, 3, 0, time.UTC)},
		{InstrumentID: 21, Slot: "0945", Status: 8, MarketDataType: 1, EventTs: tp(time.Date(2026, 11, 10, 14, 45, 0, 0, time.UTC))},
	}
}

func refreshState() RefreshState {
	return RefreshState{Enabled: true, Symbols: []string{"AAA", "BBB"}, Notice: "27 of 31 quotes fresh, 4 from 09:45", Fresh: 27, Total: 31}
}

func optionWithQuotes(t *testing.T, q openquote.OptionOpenReader, rs RefreshState) []byte {
	t.Helper()
	c := OptionConfig{
		RunID: "run-1", BuildID: "build-1", Scope: "options", Zone: testZone,
		Candidates: &fakeCands{byDay: map[contracts.Day][]store.Candidate{buildDay: fixtureRows()}},
		Eod:        fixtureEod(), Names: tickerNames(), Quotes: q, Refresh: rs,
	}
	b, err := BuildOption(context.Background(), c, nil, buildDay, buildAt)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func quoteOfRow(t *testing.T, m map[string]any, id string) map[string]any {
	t.Helper()
	for _, r := range tableRowsOf(m) {
		if rm := r.(map[string]any); rm["id"] == id {
			return rm["quote"].(map[string]any)
		}
	}
	t.Fatalf("no row %s", id)
	return nil
}

func TestOptionPageQuotePerRow(t *testing.T) {
	t.Parallel()
	reader := &fakeOpens{rows: openRows()}
	m := decodeMap(t, optionWithQuotes(t, reader, refreshState()))
	if len(reader.days) != 1 || reader.days[0] != buildDay {
		t.Fatalf("open rows asked for %v, want the build day", reader.days)
	}
	want := map[string]map[string]any{
		"r11": {"as_of": "22:45:12", "data_type": "live", "source": "open_pull", "skew": true, "outside_band": false},
		"r12": {"as_of": "23:02:03", "data_type": "delayed", "source": "refresh", "skew": false, "outside_band": true},
		"r21": {"as_of": "close archive, 9 Nov", "data_type": nil, "source": "close_archive", "skew": false, "outside_band": false},
		"r22": {"as_of": "close archive, 10 Nov", "data_type": nil, "source": "close_archive", "skew": false, "outside_band": false},
	}
	for id, w := range want {
		got := quoteOfRow(t, m, id)
		for k, v := range w {
			if got[k] != v {
				t.Errorf("%s quote.%s = %v, want %v", id, k, got[k], v)
			}
		}
	}
}

func TestOptionPageRefreshCopied(t *testing.T) {
	t.Parallel()
	got := decodeMap(t, optionWithQuotes(t, nil, refreshState()))["refresh"].(map[string]any)
	if got["enabled"] != true || got["notice"] != "27 of 31 quotes fresh, 4 from 09:45" || got["fresh_count"] != 27.0 || got["total_count"] != 31.0 {
		t.Fatalf("refresh = %v", got)
	}
	if s := got["symbols"].([]any); len(s) != 2 || s[0] != "AAA" {
		t.Fatalf("symbols = %v", s)
	}
}

func TestOptionPageDisabledRefreshSaysWhy(t *testing.T) {
	t.Parallel()
	got := decodeMap(t, optionWithQuotes(t, nil, RefreshState{Reason: "market closed; stored quotes of 16:00"}))["refresh"].(map[string]any)
	if got["enabled"] != false || got["notice"] != "market closed; stored quotes of 16:00" {
		t.Fatalf("refresh = %v", got)
	}
	if s := got["symbols"].([]any); len(s) != 0 {
		t.Fatalf("symbols = %v, want []", s)
	}
	empty := decodeMap(t, optionWithQuotes(t, nil, RefreshState{Enabled: true}))["refresh"].(map[string]any)
	if empty["notice"] != nil {
		t.Fatalf("empty notice = %v, want null", empty["notice"])
	}
}

func TestOptionPageOpenReaderError(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	c := OptionConfig{Zone: testZone, Candidates: &fakeCands{}, Quotes: &fakeOpens{err: boom}}
	if _, err := BuildOption(context.Background(), c, nil, buildDay, buildAt); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the reader error", err)
	}
}

func TestOptionPageOpenGolden(t *testing.T) {
	t.Parallel()
	b := optionWithQuotes(t, &fakeOpens{rows: openRows()}, refreshState())
	goldenCheck(t, "option_page_open", b)
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := compileSchema(t, "page.option_page.v2.json").Validate(v); err != nil {
		t.Fatal(err)
	}
}

func stockWithQuotes(t *testing.T, q openquote.OptionOpenReader) (stockPayload, []byte) {
	t.Helper()
	c := stockCfg(stockDay, stockFixture(), "AAPL")
	c.Quotes, c.Refresh = q, refreshState()
	return buildStockPayload(t, c, stockDay)
}

func TestStockPageQuotePerRow(t *testing.T) {
	t.Parallel()
	opens := []contracts.OptionOpen{
		{InstrumentID: 101, Slot: openquote.SlotRefresh, Status: 1, MarketDataType: 2, Skew: true, EventTs: tp(time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC))},
	}
	p, b := stockWithQuotes(t, &fakeOpens{rows: opens})
	wantOpen := quote{AsOf: "23:00:00", DataType: dataTypeOf(2), Source: "refresh", Skew: true}
	wantArchive := quote{AsOf: "close archive, 9 Nov", Source: "close_archive"}
	for _, c := range p.Matrix.Cells {
		w := wantArchive
		if c.Key == "c101" {
			w = wantOpen
		}
		if c.Key == "c103" || c.Key == "c201" {
			w.AsOf = "close archive, 10 Nov"
		}
		if c.Quote.AsOf != w.AsOf || c.Quote.Source != w.Source || c.Quote.Skew != w.Skew || (c.Quote.DataType == nil) != (w.DataType == nil) {
			t.Errorf("cell %s quote = %+v, want %+v", c.Key, c.Quote, w)
		}
	}
	if len(p.Contracts.Rows) != 3 {
		t.Fatalf("contract rows = %d", len(p.Contracts.Rows))
	}
	if p.Refresh.TotalCount != 31 || p.Refresh.FreshCount != 27 {
		t.Fatalf("refresh = %+v", p.Refresh)
	}
	checkGolden(t, "stock_option_page_open", b)
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := compileSchema(t, "page.stock_option_page.v2.json").Validate(v); err != nil {
		t.Fatal(err)
	}
}

func TestStockContractLineShowsRefreshBid(t *testing.T) {
	t.Parallel()
	bid := 9.99
	opens := []contracts.OptionOpen{{InstrumentID: 101, Slot: openquote.SlotRefresh, Status: 1, MarketDataType: 1, Bid: &bid,
		EventTs: tp(time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC))}}
	p, _ := stockWithQuotes(t, &fakeOpens{rows: opens})
	for _, l := range p.Contracts.Rows {
		if l.Quote.Source == "refresh" && l.Cells[2] != "9.99" {
			t.Fatalf("line labelled refresh shows bid %s, want the refresh bid 9.99", l.Cells[2])
		}
	}
}

func TestStockRowsOfAnOpenQuoteShowItsPrices(t *testing.T) {
	t.Parallel()
	f := func(v float64) *float64 { return &v }
	opens := []contracts.OptionOpen{{
		InstrumentID: 101, Slot: "0945", Status: 1, MarketDataType: 1, Bid: f(1.5), Ask: f(1.6), Delta: f(-0.2), IvDecimal: f(0.31),
		ThetaPerCalendarDay: f(-0.03), Gamma: f(0.04), VegaPerVolPoint: f(0.06), Volume: f(77),
		EventTs: tp(time.Date(2026, 11, 10, 14, 45, 0, 0, time.UTC)),
	}}
	p, _ := stockWithQuotes(t, &fakeOpens{rows: opens})
	var line contractLine
	for _, l := range p.Contracts.Rows {
		if l.Quote.Source == "open_pull" {
			line = l
		}
	}
	want := []string{"1.5", "1.6", "−0.2", "31%", "-0.0300", "0.0400", "0.0600", "77"}
	for i, w := range want {
		if got := line.Cells[2+i]; got != w {
			t.Errorf("contract cell %d = %q, want %q", 2+i, got, w)
		}
	}
	for _, c := range p.Matrix.Cells {
		switch c.Key {
		case "c101":
			if c.Premium != "Premium 1.5" || c.Delta != "Δ −0.2" || c.Volume != "Volume 77" {
				t.Errorf("open cell = %+v", c)
			}
		case "c102":
			if c.Premium != "Premium 1.20" {
				t.Errorf("close cell premium = %q, want the stored close bid", c.Premium)
			}
		}
	}
}

func TestLiveGreeksFourDecimals(t *testing.T) {
	t.Parallel()
	f := func(v float64) *float64 { return &v }
	opens := []contracts.OptionOpen{{
		InstrumentID: 101, Slot: "0945", Status: 1, MarketDataType: 1, Bid: f(1.5), Ask: f(1.6), Delta: f(-0.2), IvDecimal: f(0.31),
		ThetaPerCalendarDay: f(-0.03), Gamma: f(0.04), VegaPerVolPoint: nil, Volume: f(77),
		EventTs: tp(time.Date(2026, 11, 10, 14, 45, 0, 0, time.UTC)),
	}, {
		InstrumentID: 102, Slot: "0945", Status: 1, MarketDataType: 1, Bid: f(1.5), Ask: f(1.6), Delta: f(-0.2), IvDecimal: f(0.31),
		ThetaPerCalendarDay: f(-0.03), Gamma: f(0.04), VegaPerVolPoint: f(0.06), Volume: f(77),
		EventTs: tp(time.Date(2026, 11, 10, 14, 45, 0, 0, time.UTC)),
	}}
	p, _ := stockWithQuotes(t, &fakeOpens{rows: opens})
	for _, l := range p.Contracts.Rows {
		if l.Quote.Source != "open_pull" {
			continue
		}
		vega := "0.0600"
		if len(l.Cells) > 8 && l.Cells[8] == "n/a" {
			vega = "n/a"
		}
		want := []string{"-0.0300", "0.0400", vega, "77"}
		got := l.Cells[6:10]
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("cell %d = %q, want %q", 6+i, got[i], want[i])
			}
		}
	}
}

func TestOptionTableRowOfAnOpenQuoteShowsItsPrices(t *testing.T) {
	t.Parallel()
	f := func(v float64) *float64 { return &v }
	opens := []contracts.OptionOpen{{
		InstrumentID: 11, Slot: "0945", Status: 1, MarketDataType: 1, Bid: f(2), Ask: f(2.2), Delta: f(-0.25), Volume: f(5),
		EventTs: tp(time.Date(2026, 11, 10, 14, 45, 0, 0, time.UTC)),
	}}
	m := decodeMap(t, optionWithQuotes(t, &fakeOpens{rows: opens}, RefreshState{}))
	var open, closed any
	for _, r := range tableRowsOf(m) {
		switch r.(map[string]any)["id"] {
		case "r11":
			open = r
		case "r12":
			closed = r
		}
	}
	if got := cellOf(t, open, "bid_ask")["text"]; got != "$2 / $2.2" {
		t.Errorf("open bid_ask = %v", got)
	}
	if got := cellOf(t, open, "delta")["text"]; got != "−0.25" {
		t.Errorf("open delta = %v", got)
	}
	if got := cellOf(t, closed, "bid_ask")["text"]; got != "$0.95 / $1.05" {
		t.Errorf("close bid_ask = %v", got)
	}
}
