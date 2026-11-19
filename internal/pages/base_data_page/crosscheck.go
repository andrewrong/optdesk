// crosscheck.go builds the cross_check object of the bars row from the newest run of pair R6 (docs/design/base-data-page.md,
// entry 22). The level comes from the injected Verdict; the page makes none of its own beyond "no run" and "failed".
package base_data_page

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
	"github.com/andrewrong/optdesk/internal/compute/reconapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	reconstore "github.com/andrewrong/optdesk/internal/store/recon"
)

const (
	barsRowID     = "bars"
	pairR6        = "R6"
	sourceR6      = "yfinance" // the source of R6 (owner's word 2026-11-18) when no run says otherwise
	reasonNoRun   = "no_run"
	statusFailed  = "failed"
	itemsShownMax = 20
)

type crossItemVM struct {
	Key     string `json:"key"`
	Ours    string `json:"ours"`
	Other   string `json:"other"`
	Diff    string `json:"diff"`
	Kind    string `json:"kind"`
	HintZh  string `json:"hint_zh"`
	Company string `json:"company,omitempty"`
}

type crossCheckVM struct {
	PairID        string        `json:"pair_id"`
	Level         string        `json:"level"`
	N             int           `json:"n"`
	Matched       int           `json:"matched"`
	Diff          int           `json:"diff"`
	Missing       int           `json:"missing"`
	Skipped       int           `json:"skipped"`
	Reason        string        `json:"reason,omitempty"`
	ReasonZh      string        `json:"reason_zh,omitempty"`
	ToleranceText string        `json:"tolerance_text"`
	SourceID      string        `json:"source_id"`
	RunAt         *string       `json:"run_at"`
	Items         []crossItemVM `json:"items"`
}

// reasonNames are the Chinese names of the reason codes of a run.
var reasonNames = map[string]string{
	"rate_limited": "被限流", "network": "网络错误", "timeout": "超时", "source_changed": "数据源格式变了",
	"no_data": "数据源没有数据", "source_lacks": "数据源没有这一天",
}

func reasonName(code string) string {
	if n, ok := reasonNames[code]; ok {
		return n
	}
	return code
}

// readCrossCheck reads the runs once and returns the cross_check of the bars row and the pair levels for Eval. With no
// reader it returns nothing; with a failed reader it returns the error and nothing else.
func readCrossCheck(ctx context.Context, in Input, day contracts.Day) (*crossCheckVM, []basecheckapi.PairLevel, error) {
	if in.Recon == nil {
		return nil, nil, nil
	}
	runs, err := in.Recon.Latest(ctx)
	if err != nil {
		return nil, nil, err
	}
	cc := crossCheckOf(runs, day, in.Verdict)
	return &cc, []basecheckapi.PairLevel{pairLevelOf(barsRowID, cc)}, nil
}

// crossCheckOf is the cross_check of pair R6 for the session day: from the newest run of that pair and day.
// No run is unknown (no_run); a failed run is unknown with its code, without a call of verdict.
func crossCheckOf(runs []reconstore.Run, day contracts.Day, verdict reconapi.VerdictFunc) crossCheckVM {
	var run *reconstore.Run
	for i := range runs {
		r := &runs[i]
		if r.PairID == pairR6 && r.SessionDay == day && (run == nil || r.FinishedAt.After(run.FinishedAt)) {
			run = r
		}
	}
	cc := crossCheckVM{PairID: pairR6, SourceID: sourceR6, Items: []crossItemVM{}}
	if run == nil {
		cc.Level = string(basecheckapi.Unknown)
		cc.Reason = reasonNoRun
		cc.ReasonZh = reasonNoRunZh
		return cc
	}
	at := run.FinishedAt.UTC().Format(time.RFC3339)
	cc.N, cc.Matched, cc.Diff = run.N, run.Matched, run.Diff
	cc.Missing = run.MissingOurs + run.MissingOther
	cc.Skipped = run.Skipped
	cc.RunAt = &at
	if run.SourceID != "" {
		cc.SourceID = run.SourceID
	}
	cc.ToleranceText = run.ToleranceText
	if run.Reason != "" {
		cc.Reason = run.Reason
		cc.ReasonZh = reasonFailedZh + reasonName(run.Reason)
	}
	if run.Status == statusFailed {
		cc.Level = string(basecheckapi.Unknown)
		if run.Reason == "" {
			cc.ReasonZh = reasonFailedZh
		}
		return cc
	}
	cc.Level = string(verdict(pairR6, reconapi.Output{N: run.N, Matched: run.Matched, Diff: run.Diff,
		MissingOurs: run.MissingOurs, MissingOther: run.MissingOther, Skipped: run.Skipped},
		run.PairsAsked, run.PairsCompared))
	cc.ToleranceText = comparedText(run.ToleranceText, run.N, run.Skipped)
	for i, it := range run.Items {
		if i >= itemsShownMax {
			break
		}
		cc.Items = append(cc.Items, crossItemOf(it))
	}
	return cc
}

// comparedText appends "compared N of M" (M is N plus the skipped) to the tolerance text.
func comparedText(tol string, n, skipped int) string {
	t := fmt.Sprintf(comparedFormat, n, n+skipped)
	if tol == "" {
		return t
	}
	return tol + "；" + t
}

func crossItemOf(it reconstore.Item) crossItemVM {
	v := crossItemVM{Key: it.Key, Ours: it.Ours, Other: it.Other, Diff: it.Diff, Kind: it.Kind, Company: it.Company}
	if it.Kind == string(reconapi.Skipped) {
		v.HintZh = skippedPrefixZh + reasonName(it.Reason)
		return v
	}
	v.HintZh = hintR6
	return v
}
