package optionnotice

import "github.com/andrewrong/optdesk/internal/compute"

// ruleVersion is reported in every result; bump it when a code, severity or threshold rule changes.
const ruleVersion = 1

// DefaultParams implements compute.DefaultNoticeParamsFunc with the thresholds the owner
// confirmed on 2026-11-09: expiry near within 5 calendar days, cushion warn 0.15, act 0.05.
func DefaultParams() compute.NoticeParams {
	return compute.NoticeParams{ExpiryDays: 5, CushionWarn: "0.15", CushionAct: "0.05"}
}

var _ compute.DefaultNoticeParamsFunc = DefaultParams
var _ compute.NewOptionNoticesFunc = New
