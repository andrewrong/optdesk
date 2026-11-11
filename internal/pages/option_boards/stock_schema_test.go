package option_boards

import (
	"bytes"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestStockValidatesSchema(t *testing.T) {
	t.Parallel()
	schema := compileSchema(t, "page.stock_option_page.v2.json")
	stale := stockFixture()
	for i := range stale {
		stale[i].SessionDay = "2026-11-09"
	}
	// every case is built on stockDay; the rows of "stale" are of the day before, those of "no_rows" of none.
	cases := map[string]struct {
		cfg StockConfig
	}{
		"ok":      {stockCfg(stockDay, stockFixture(), "AAPL")},
		"bare":    {stockCfg(stockDay, stockFixture(), "")},
		"no_key":  {stockCfg(stockDay, stockFixture(), "NOPE")},
		"no_rows": {stockCfg("2026-10-01", stockFixture(), "AAPL")},
		"stale":   {stockCfg("2026-11-09", stale, "AAPL")},
	}
	noReader := stockCfg(stockDay, nil, "AAPL")
	noReader.Candidates, noReader.Eod, noReader.Names = nil, nil, nil
	cases["no_reader"] = struct{ cfg StockConfig }{noReader}
	for name, tc := range cases {
		_, b := buildStockPayload(t, tc.cfg, stockDay)
		v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(v); err != nil {
			t.Errorf("%s: %v\n%s", name, err, b)
		}
	}
}
