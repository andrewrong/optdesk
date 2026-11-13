// Package shortput is the short put behind strategy.Strategy: an adapter over the put rule of compute
// (compute.OptionCandidates, KindShortPut). The rule is not rewritten; this package maps the Snapshot to the
// rule's input and each Candidate to an Evaluation, ranks as the job does, answers Manage and hashes its parameters.
package shortput

import (
	"fmt"
	"math/big"
	"strconv"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/strategy"
)

const (
	// ID is the strategy column of a result row.
	ID = "short_put"
	// RuleVersion is the rule_version column, as the job stores it today.
	RuleVersion = "put_v1"

	// CloseExpiry holds to expiry; CloseEarly closes by early_keep or early_dte.
	CloseExpiry = "expiry"
	CloseEarly  = "early"
)

// Values are the management parameters, as text: CloseMode is "expiry" or "early", EarlyKeep a decimal (the share
// of the credit kept at which an early close happens), EarlyDTE an integer (days to expiry).
type Values struct {
	CloseMode string
	EarlyKeep string
	EarlyDTE  string
}

type shortPut struct {
	rule      compute.OptionCandidates
	cand      compute.CandidateParams
	params    strategy.Params
	closeMode string
	earlyKeep *big.Rat
	earlyDTE  int
}

// New builds the short put over a rule. A bad close_mode, early_keep or early_dte is an error that names the key
// and never the value.
func New(rule compute.OptionCandidates, p compute.CandidateParams, v Values) (strategy.Strategy, error) {
	if v.CloseMode != CloseExpiry && v.CloseMode != CloseEarly {
		return nil, fmt.Errorf("close_mode is not %q or %q", CloseExpiry, CloseEarly)
	}
	keep, ok := parseDecimal(v.EarlyKeep)
	if !ok {
		return nil, fmt.Errorf("early_keep is not a decimal")
	}
	dte, err := strconv.Atoi(v.EarlyDTE)
	if err != nil {
		return nil, fmt.Errorf("early_dte is not an integer")
	}
	return &shortPut{
		rule: rule, cand: p, params: paramsOf(p, v),
		closeMode: v.CloseMode, earlyKeep: keep, earlyDTE: dte,
	}, nil
}

func (s *shortPut) ID() string              { return ID }
func (s *shortPut) Version() string         { return RuleVersion }
func (s *shortPut) Params() strategy.Params { return s.params }

// Evaluate calls the put rule for the snapshot and maps every row.
func (s *shortPut) Evaluate(snap strategy.Snapshot) []strategy.Evaluation {
	rows := s.rule.Evaluate(inputOf(snap), s.cand)
	out := make([]strategy.Evaluation, 0, len(rows))
	for _, c := range rows {
		out = append(out, evaluationOf(c, snap))
	}
	return out
}
