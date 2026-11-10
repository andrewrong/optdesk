package valuation

import (
	"sort"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// The band of Decision 3: the value per share at three required yields of the free cash
// flow. The primary yield (8%) gives the maximum assignment price, the high yield (10%)
// the lower edge and the low yield (6%) the reference high.
const (
	yieldLow     = "0.06"
	yieldPrimary = "0.08"
	yieldHigh    = "0.10"
	bandYears    = 5 // fiscal years whose median free cash flow is taken
)

// Reasons the inputs of the band and of the reverse DCF are refused.
const (
	reasonInsufficientYears = "insufficient_years"
	reasonMissingShares     = "missing_shares"
	reasonNonPositiveShares = "non_positive_shares"
	reasonSharesScale       = "shares_scale_suspect"
	reasonSharesMismatch    = "shares_scale_mismatch"
	reasonSharesChanged     = "shares_changed_since_base"
	reasonNonPositiveFCF    = "non_positive_fcf"
	reasonMissingInputFmt   = "missing_input:%s:%s"
)

var (
	minShares        = decimal.NewFromInt(1_000_000)
	maxSharesRatio   = decimal.NewFromInt(100)
	maxCoverToShares = decimal.RequireFromString("1.25")
	decOne           = decimal.NewFromInt(1)
)

// fcfRow is one fiscal year of free cash flow: operating cash flow minus capital spending
// minus stock compensation, as filed.
type fcfRow struct {
	ocf, capex, fcf decimal.Decimal
}

// fcfInput is what the band and the reverse DCF read from Filings.
type fcfInput struct {
	rows   []fcfRow // newest first, bandYears of them
	shares decimal.Decimal
	cover  *decimal.Decimal // the cover-page count, nil when absent
	debt   *decimal.Decimal // long-term debt of the latest year, nil when absent
}

// readFcfInputs reads the free cash flow rows, the shares and the debt of f. It returns the
// reasons the inputs are refused: too few years or a missing figure, then the share-count
// checks, in the order of the system this ports.
func readFcfInputs(f valuationapi.Filings) (fcfInput, []string) {
	var in fcfInput
	if len(f.Years) < bandYears {
		return in, []string{reasonInsufficientYears}
	}
	var reasons []string
	complete := true
	for _, y := range f.Years[:bandYears] {
		var v [3]decimal.Decimal
		ok := true
		for i, role := range [3]string{roleOperatingCashFlow, roleCapitalSpending, roleStockCompensation} {
			d, found := parseValue(y.Values[role])
			if !found {
				ok = false
				reasons = append(reasons, sprintf(reasonMissingInputFmt, role, string(y.FYEnd)))
				continue
			}
			v[i] = d
		}
		if !ok {
			complete = false
			continue
		}
		in.rows = append(in.rows, fcfRow{ocf: v[0], capex: v[1], fcf: v[0].Sub(v[1]).Sub(v[2])})
	}
	if !complete {
		return in, append(reasons, reasonInsufficientYears)
	}
	shares, haveShares := parseValue(f.SharesOutstanding)
	switch {
	case !haveShares:
		reasons = append(reasons, reasonMissingShares)
	case shares.Sign() <= 0:
		reasons = append(reasons, reasonNonPositiveShares)
	case shares.LessThan(minShares):
		reasons = append(reasons, reasonSharesScale)
	}
	in.shares = shares
	changed := false
	if c, ok := parseValue(f.CoverShares); ok {
		in.cover = &c
		if haveShares && shares.Sign() > 0 {
			big, small := decimal.Max(shares, c), decimal.Min(shares, c)
			switch {
			case c.Sign() <= 0 || big.GreaterThanOrEqual(small.Mul(maxSharesRatio)):
				reasons = append(reasons, reasonSharesMismatch)
			default:
				changed = c.GreaterThanOrEqual(shares.Mul(maxCoverToShares))
			}
		}
	}
	if d, ok := parseValue(f.Years[0].Values[roleLongTermDebt]); ok {
		in.debt = &d
	}
	if changed {
		reasons = append(reasons, reasonSharesChanged)
	}
	return in, reasons
}

func parseValue(text string) (decimal.Decimal, bool) {
	if text == "" {
		return decimal.Zero, false
	}
	d, err := decimal.NewFromString(text)
	return d, err == nil
}

// median of the values; the mean of the two middle ones for an even count.
func median(vs []decimal.Decimal) decimal.Decimal {
	s := append([]decimal.Decimal(nil), vs...)
	sort.Slice(s, func(i, j int) bool { return s[i].LessThan(s[j]) })
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	q, _ := div(s[n/2-1].Add(s[n/2]), decimal.NewFromInt(2))
	return q
}

// Band forms the buy band of one company from its filings: the median free cash flow of
// the latest five years over the latest diluted shares, over each required yield. It
// implements valuationapi.BandFunc. It takes no price, holds no expiry and reads no clock;
// the approval day sets validity (Decision 3). A financial or cyclical company is
// unvalued; refused inputs give missing_input with their reasons.
func Band(id string, f valuationapi.Filings, _ contracts.Day) valuationapi.BandFigure {
	fig := valuationapi.BandFigure{ID: id, State: valuationapi.StateMissingInput, Currency: f.Currency}
	p, ok := ParamsFor(f.Standard)
	if !ok {
		fig.Reasons = []string{reasonUnsupportedStd}
		return fig
	}
	if reason := unvaluedSIC(f.SIC, p); reason != "" {
		fig.State, fig.Reasons = valuationapi.StateUnvalued, []string{reason}
		return fig
	}
	in, reasons := readFcfInputs(f)
	fcfs := make([]decimal.Decimal, len(in.rows))
	ocfs := make([]decimal.Decimal, len(in.rows))
	for i, r := range in.rows {
		fcfs[i], ocfs[i] = r.fcf, r.ocf
	}
	if len(reasons) == 0 || len(in.rows) == bandYears {
		if median(fcfs).Sign() <= 0 {
			reasons = append(reasons, reasonNonPositiveFCF)
		}
	}
	if len(reasons) > 0 {
		fig.Reasons = reasons
		return fig
	}
	medFCF := median(fcfs)
	at := func(y string) string {
		q, _ := div(medFCF, in.shares)
		q, _ = div(q, decimal.RequireFromString(y))
		return q.RoundBank(4).StringFixed(4)
	}
	fig.State = valuationapi.StateOK
	fig.Value, fig.Low, fig.RefHigh = at(yieldPrimary), at(yieldHigh), at(yieldLow)
	fig.BaseFYEnd = f.Years[0].FYEnd
	fig.Flags = bandFlags(in.rows, medFCF, median(ocfs), in.debt, in.cover != nil)
	return fig
}

var _ valuationapi.BandFunc = Band
