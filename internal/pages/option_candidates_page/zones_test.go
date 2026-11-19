package option_candidates_page

import (
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/buypriceapi"
	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

func TestZonesCopiesState(t *testing.T) {
	t.Parallel()
	st := valuation.State{
		Entries: []valuation.Entry{
			{
				Profile: valuation.Profile{Symbol: "AAA", Currency: "USD", Pool: "in_pool", RMarket: "US"},
				Current: &valuation.Review{V: "100", P: "3", VersionDate: contracts.Day("2026-11-10")},
			},
			{Profile: valuation.Profile{Symbol: "BBB", Currency: "JPY", Pool: "removed", RMarket: "JP"}},
		},
		RVersion: &valuation.RVersion{RUS: "0.9", RJP: "0.8", RHKCN: "0.7"},
		Margin:   &valuation.MarginTableVersion{Rows: []valuation.MarginTableRow{{PFrom: "1", M: "0.1", W: "0.2", H: "0.3", S: "0.4"}}},
	}
	zones, tables := zonesOf(st)
	wantZones := []buypriceapi.Zone{
		{Symbol: "AAA", Currency: "USD", Pool: "in_pool", Market: "US", V: "100", P: "3", ReviewDay: "2026-11-10", Reviewed: true},
		{Symbol: "BBB", Currency: "JPY", Pool: "removed", Market: "JP"},
	}
	if !reflect.DeepEqual(zones, wantZones) {
		t.Errorf("zones = %+v", zones)
	}
	wantTables := buypriceapi.Tables{
		Present: true, R: valuelinesapi.RValues{US: "0.9", JP: "0.8", HKCN: "0.7"},
		Margin: []valuelinesapi.MarginRow{{PFrom: "1", M: "0.1", W: "0.2", H: "0.3", S: "0.4"}},
	}
	if !reflect.DeepEqual(tables, wantTables) {
		t.Errorf("tables = %+v", tables)
	}
	st.Margin = nil
	if _, tb := zonesOf(st); tb.Present {
		t.Error("tables present without a margin table")
	}
}
