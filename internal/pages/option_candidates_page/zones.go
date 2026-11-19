package option_candidates_page

import (
	"github.com/andrewrong/optdesk/internal/compute/buypriceapi"
	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// zonesOf copies the stored valuation, field by field, into the plain zones and tables the resolver reads.
// Tables.Present is true only when both the newest R version and the newest margin table exist.
func zonesOf(st valuation.State) ([]buypriceapi.Zone, buypriceapi.Tables) {
	zones := make([]buypriceapi.Zone, 0, len(st.Entries))
	for _, e := range st.Entries {
		z := buypriceapi.Zone{
			Symbol: e.Profile.Symbol, Currency: e.Profile.Currency, Pool: e.Profile.Pool, Market: e.Profile.RMarket,
		}
		if e.Current != nil {
			z.V, z.P, z.ReviewDay, z.Reviewed = e.Current.V, e.Current.P, string(e.Current.VersionDate), true
		}
		zones = append(zones, z)
	}
	var t buypriceapi.Tables
	if st.RVersion == nil || st.Margin == nil {
		return zones, t
	}
	t.Present = true
	t.R = valuelinesapi.RValues{US: st.RVersion.RUS, JP: st.RVersion.RJP, HKCN: st.RVersion.RHKCN}
	t.Margin = make([]valuelinesapi.MarginRow, 0, len(st.Margin.Rows))
	for _, r := range st.Margin.Rows {
		t.Margin = append(t.Margin, valuelinesapi.MarginRow{PFrom: r.PFrom, M: r.M, W: r.W, H: r.H, S: r.S})
	}
	return zones, t
}
