package main

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/jobs"
)

const watchTwo = `
[[underlying]]
issuer = "cik:0000320193"
market = "US"
symbol = "AAPL"
sec_type = "STK"
exchange = "SMART"
currency = "USD"
con_id = 1001
report_date_override = "2026-11-20"
buy_price = "150.5"
sale_price = "200"

[[underlying]]
issuer = "edinet:E02144"
market = "JP"
symbol = "7203"
sec_type = "STK"
exchange = "TSEJ"
currency = "JPY"
con_id = 1002
`

func TestWatchLoad(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	var read string
	a.deps.readFile = func(p string) ([]byte, error) { read = p; return []byte(watchTwo), nil }
	got, err := loadWatch(a)
	if err != nil {
		t.Fatal(err)
	}
	want := []jobs.WatchEntry{
		{
			Issuer: "cik:0000320193", Market: "US", ReportDateOverride: "2026-11-20", BuyPrice: "150.5", SalePrice: "200",
			Contract: broker.Contract{SecType: "STK", Symbol: "AAPL", Exchange: "SMART", Currency: "USD", ConID: 1001},
		},
		{
			Issuer: "edinet:E02144", Market: "JP",
			Contract: broker.Contract{SecType: "STK", Symbol: "7203", Exchange: "TSEJ", Currency: "JPY", ConID: 1002},
		},
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	if !strings.HasSuffix(read, "/watch.toml") || !strings.HasPrefix(read, a.getenv("OT_HOME")) {
		t.Fatalf("read %q", read)
	}

	// A missing file is an empty list; another read error is refused without the path.
	a.deps.readFile = func(string) ([]byte, error) { return nil, fs.ErrNotExist }
	if got, err := loadWatch(a); err != nil || len(got) != 0 {
		t.Fatalf("missing file: %v %v", got, err)
	}
	a.deps.readFile = func(p string) ([]byte, error) { return nil, errors.New("denied " + p) }
	_, err = loadWatch(a)
	if err == nil || strings.Contains(err.Error(), a.getenv("OT_HOME")) {
		t.Fatalf("err %v", err)
	}
}

func entryWith(extra string) string {
	return "[[underlying]]\nissuer = \"cik:0000000001\"\nmarket = \"US\"\nsymbol = \"X\"\n" + extra
}

func TestWatchRefusals(t *testing.T) {
	t.Parallel()
	const first = "[[underlying]]\nissuer = \"cik:0000000001\"\nmarket = \"US\"\n"
	for name, tc := range map[string]struct{ body, want string }{
		"unknown market":     {strings.Replace(entryWith(""), `"US"`, `"HK"`, 1), "cik:0000000001: market"},
		"duplicate issuer":   {entryWith("") + first, "cik:0000000001: listed twice"},
		"short cik":          {strings.Replace(entryWith(""), "0000000001", "123", 1), "cik:123"},
		"letters in cik":     {strings.Replace(entryWith(""), "0000000001", "00000000ab", 1), "cik:00000000ab"},
		"no issuer scheme":   {strings.Replace(entryWith(""), "cik:", "", 1), "0000000001"},
		"override format":    {entryWith("report_date_override = \"20-11-2026\"\n"), "cik:0000000001: report_date_override"},
		"override not a day": {entryWith("report_date_override = \"2026-02-30\"\n"), "cik:0000000001: report_date_override"},
		"override unquoted":  {entryWith("report_date_override = 2026-11-20\n"), "cik:0000000001: report_date_override"},
		"buy zero":           {entryWith("buy_price = \"0\"\n"), "cik:0000000001: buy_price"},
		"buy negative":       {entryWith("buy_price = \"-1\"\n"), "cik:0000000001: buy_price"},
		"buy five places":    {entryWith("buy_price = \"1.12345\"\n"), "cik:0000000001: buy_price"},
		"buy text":           {entryWith("buy_price = \"1e3\"\n"), "cik:0000000001: buy_price"},
		"buy number":         {entryWith("buy_price = 150.5\n"), "cik:0000000001: buy_price"},
		"sale zero":          {entryWith("sale_price = \"0.0000\"\n"), "cik:0000000001: sale_price"},
		"sale trailing dot":  {entryWith("sale_price = \"5.\"\n"), "cik:0000000001: sale_price"},
		"sale below buy":     {entryWith("buy_price = \"10.01\"\nsale_price = \"10.0099\"\n"), "cik:0000000001: sale_price is below buy_price"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := parseWatch([]byte(tc.body))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err %v, want it to contain %q", err, tc.want)
			}
		})
	}
	// Malformed TOML and an unknown key are refused by a fixed text.
	for _, body := range []string{"[[underlying", entryWith("colour = \"red\"\n")} {
		if _, err := parseWatch([]byte(body)); err == nil || !strings.Contains(err.Error(), "watch.toml is not valid") {
			t.Fatalf("err %v", err)
		}
	}
}

func TestWatchPricesLoaded(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ extra, buy, sale string }{
		"both unset":      {"", "", ""},
		"empty is unset":  {"buy_price = \"\"\nsale_price = \"\"\n", "", ""},
		"buy only":        {"buy_price = \"0.0001\"\n", "0.0001", ""},
		"sale only":       {"sale_price = \"12\"\n", "", "12"},
		"equal":           {"buy_price = \"10.5000\"\nsale_price = \"10.5\"\n", "10.5000", "10.5"},
		"sale above buy":  {"buy_price = \"9\"\nsale_price = \"10.25\"\n", "9", "10.25"},
		"numeric compare": {"buy_price = \"9\"\nsale_price = \"10\"\n", "9", "10"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := parseWatch([]byte(entryWith(tc.extra)))
			if err != nil || len(got) != 1 || got[0].BuyPrice != tc.buy || got[0].SalePrice != tc.sale {
				t.Fatalf("got %+v err %v", got, err)
			}
		})
	}
}
