package shortput

import (
	"strconv"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/strategy"
	"github.com/andrewrong/optdesk/internal/strategy/params"
)

// paramsOf names every key of compute.CandidateParams (the [params] keys of job.toml) and the three management keys.
func paramsOf(p compute.CandidateParams, v Values) strategy.Params {
	values := map[string]string{
		"min_dte":                strconv.Itoa(p.MinDTE),
		"max_dte":                strconv.Itoa(p.MaxDTE),
		"min_bid_size":           strconv.Itoa(p.MinBidSize),
		"max_spread":             p.MaxSpread,
		"min_open_interest":      strconv.Itoa(p.MinOpenInterest),
		"div_guard_factor":       p.DivGuardFactor,
		"max_rate_age_days":      strconv.Itoa(p.MaxRateAgeDays),
		"max_event_age_sessions": strconv.Itoa(p.MaxEventAgeSessions),
		"min_report_gap_days":    strconv.Itoa(p.MinReportGapDays),
		"close_mode":             v.CloseMode,
		"early_keep":             v.EarlyKeep,
		"early_dte":              v.EarlyDTE,
	}
	return strategy.Params{Values: values, Hash: params.Hash(values)}
}
