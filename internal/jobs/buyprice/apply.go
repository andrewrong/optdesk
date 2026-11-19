// Package buyprice sets the resolved buy price and its source on watch entries, from one
// valuation read per call (docs/design/buy-price-source.md, Decisions 4 and 5;
// docs/contracts/jobs/buyprice.md).
package buyprice

import (
	"context"
	"fmt"

	"github.com/andrewrong/optdesk/internal/compute/buypriceapi"
	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// Apply returns a copy of entries with BuyPrice and BuySource set and no other field changed;
// the input slice is not changed. It calls rd.Current once, then r.Resolve once per entry with
// the entry's Issuer, its Contract.Currency and its hand BuyPrice. A Current error is returned
// wrapped and no entry is returned. A nil rd resolves with no zones and makes no read. A nil r
// leaves each price as it is and gives BuySource hand where a price exists.
func Apply(ctx context.Context, rd valuation.Reader, r buypriceapi.Resolver, entries []jobs.WatchEntry) ([]jobs.WatchEntry, error) {
	var zones []buypriceapi.Zone
	var tables buypriceapi.Tables
	if rd != nil && r != nil {
		st, err := rd.Current(ctx)
		if err != nil {
			return nil, fmt.Errorf("read valuation: %w", err)
		}
		zones, tables = copyState(st)
	}
	out := make([]jobs.WatchEntry, len(entries))
	for i, e := range entries {
		hand := e.BuyPrice
		if r == nil {
			e.BuySource = ""
			if hand != "" {
				e.BuySource = buypriceapi.SourceHand
			}
			out[i] = e
			continue
		}
		res := r.Resolve(buypriceapi.Input{
			Symbol: e.Issuer, Currency: e.Contract.Currency, Hand: hand, Zones: zones, Tables: tables,
		})
		e.BuyPrice, e.BuySource = res.Price, res.Source
		if res.Source == buypriceapi.SourceNone {
			e.BuyPrice, e.BuySource = "", ""
		}
		out[i] = e
	}
	return out, nil
}

// copyState copies the valuation state field by field into the resolver's plain types.
func copyState(st valuation.State) ([]buypriceapi.Zone, buypriceapi.Tables) {
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
	if st.RVersion != nil && st.Margin != nil {
		t.Present = true
		t.R = valuelinesapi.RValues{US: st.RVersion.RUS, JP: st.RVersion.RJP, HKCN: st.RVersion.RHKCN}
		t.Margin = make([]valuelinesapi.MarginRow, 0, len(st.Margin.Rows))
		for _, m := range st.Margin.Rows {
			t.Margin = append(t.Margin, valuelinesapi.MarginRow{PFrom: m.PFrom, M: m.M, W: m.W, H: m.H, S: m.S})
		}
	}
	return zones, t
}
