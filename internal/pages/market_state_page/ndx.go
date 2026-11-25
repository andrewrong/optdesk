package market_state_page

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	seriesNDX = "NDX"
	// ndxBackDays is how far back the NDX bars are read.
	ndxBackDays = 1500
)

// ndxRead is the NDX closes read from the bar store. A reason other than "" is the Gap reason of the
// Nasdaq-100 row: no_reader, no_data or missing_input.
type ndxRead struct {
	series compute.Series
	rows   []contracts.PriceBar // every bar row read, for shared.BarSourceOf
	reason string
}

// readNDX reads the trades closes of the index NDX, ascending, for ndxBackDays calendar days before d.
// Without an index-id reader or a bar reader nothing is read; no instrument named NDX is no_data and two
// are missing_input.
func readNDX(ctx context.Context, c Config, d time.Time) (ndxRead, error) {
	if c.Market == nil || c.IndexIDs == nil || c.Bars == nil {
		return ndxRead{reason: reasonNoReader}, nil
	}
	rows, err := c.IndexIDs.IndexIDs(ctx, []string{seriesNDX})
	if err != nil {
		return ndxRead{}, fmt.Errorf("market_state_page: read index ids: %w", err)
	}
	var ids []int64
	for _, r := range rows {
		if r.Name == seriesNDX {
			ids = append(ids, r.InstrumentID)
		}
	}
	switch len(ids) {
	case 0:
		return ndxRead{reason: reasonNoData}, nil
	case 1:
	default:
		return ndxRead{reason: reasonMissingInput}, nil
	}
	from := contracts.Day(d.AddDate(0, 0, -ndxBackDays).Format(dayLayout))
	bars, err := c.Bars.PriceBars(ctx, ids[0], basisTrades, from, contracts.Day(d.Format(dayLayout)))
	if err != nil {
		return ndxRead{}, fmt.Errorf("market_state_page: read bars of %s: %w", seriesNDX, err)
	}
	var s compute.Series
	for _, b := range bars {
		v, err := strconv.ParseFloat(b.Close, 64)
		if err != nil {
			continue
		}
		s = append(s, compute.Point{Day: contracts.Day(b.SessionDay.Format(dayLayout)), Value: v})
	}
	return ndxRead{series: s, rows: bars}, nil
}

// nasdaqRow is the Nasdaq-100 drawdown row; a read reason is its Gap, else it is the S&P row's method on
// the compute result.
func nasdaqRow(haveMarket bool, nd ndxRead, n liquidityapi.Num) marketRow {
	if haveMarket && nd.reason != "" {
		row := marketRow{ID: "nasdaq_drawdown", Label: labelNasdaq, Window: windowDrawdown}
		row.Figure = mkGap(nd.reason, reasonText(nd.reason, labelNasdaq))
		row.Day = dayOrGap("", row.Figure, labelNasdaq)
		row.History = mkGap(nd.reason, reasonText(nd.reason, labelNasdaq+" history"))
		return row
	}
	return drawdownCard("nasdaq_drawdown", labelNasdaq, windowDrawdown, "Nasdaq-100", haveMarket, n)
}

// readBarInputs reads the closes of the funds and of NDX.
func readBarInputs(ctx context.Context, c Config, d time.Time) (barsIn, ndxRead, error) {
	bars, err := readBars(ctx, c, d)
	if err != nil {
		return barsIn{}, ndxRead{}, err
	}
	nd, err := readNDX(ctx, c, d)
	return bars, nd, err
}
