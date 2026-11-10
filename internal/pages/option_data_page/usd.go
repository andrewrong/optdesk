package option_data_page

import (
	"context"
	"fmt"
	"strconv"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

// applyUSD writes approx_usd_text ("≈ US$ N", docs/design/option-pages-v22.md Decision 5) on the
// price of the underlying and on the strike, bid, ask and last of every chain row, N being the
// value divided by the latest USD<currency> rate on or before sessionDay. With no reader or a USD
// page nothing is read and nothing changes. With no rate no figure gets the field and one gap is
// listed. A gap cell stays as it is.
func applyUSD(ctx context.Context, fx bar.FxRateReader, sessionDay string, vm *viewModel) error {
	ccy := vm.Underlying.Currency
	if fx == nil || ccy == defaultCurrency {
		return nil
	}
	pair := defaultCurrency + ccy
	r, found, err := fx.LatestFxRate(ctx, pair, contracts.Day(sessionDay))
	if err != nil {
		return fmt.Errorf("option_data_page: read fx rate %s: %w", pair, err)
	}
	if !found {
		vm.Gaps = limit(append(vm.Gaps, newGap(fmt.Sprintf("no %s rate on or before %s", pair, sessionDay))), maxGaps)
		return nil
	}
	rate, err := strconv.ParseFloat(r.Rate, 64)
	if err != nil || rate <= 0 {
		return fmt.Errorf("option_data_page: fx rate %s has rate %q", pair, r.Rate)
	}
	vm.Underlying.Price = withUSD(vm.Underlying.Price, rate)
	for i := range vm.Chain.Rows {
		row := &vm.Chain.Rows[i]
		row.Strike = withUSD(row.Strike, rate)
		row.Bid = withUSD(row.Bid, rate)
		row.Ask = withUSD(row.Ask, rate)
		row.Last = withUSD(row.Last, rate)
	}
	return nil
}

// withUSD returns a value figure with approx_usd_text set; any other cell is returned as it is.
func withUSD(cell any, rate float64) any {
	f, ok := cell.(figure)
	if !ok {
		return cell
	}
	f.ApproxUSDText = "≈ US$ " + strconv.FormatFloat(f.Value/rate, 'f', 2, 64)
	return f
}
