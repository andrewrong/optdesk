// Package optiontier is the tier rule of the short-put candidates table: for each stored short-put row
// a tier (fits, check or grey), a place, badges, figures and the cash verdict per account alias. Decimal
// text only: no float, no clock, no I/O. The margin and the moves come from the injected evaluator.
package optiontier

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/optionevidenceapi"
	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
)

// Tier words.
const (
	tierFits  = "fits"
	tierCheck = "check"
	tierGrey  = "grey"
)

type tierer struct{ ev optionevidenceapi.Evaluator }

var _ optiontierapi.Tierer = (*tierer)(nil)

// DefaultParams has the trial values of the rule: the delta bound 0.3, the thin-premium line 0.15, a cash
// figure at most 3 days old and 365 days in a year. All four are trial values; the owner sets them after
// seeing the page.
func DefaultParams() optiontierapi.Params {
	return optiontierapi.Params{
		DeltaMax:       "0.3",  // trial
		ThinDelta:      "0.15", // trial
		MaxCashAgeDays: 3,      // trial
		DaysInYear:     365,    // trial
	}
}

// New returns the Tierer; ev is the one evaluator of margin and moves.
func New(ev optionevidenceapi.Evaluator) optiontierapi.Tierer { return &tierer{ev: ev} }

func parse(s string) (decimal.Decimal, bool) {
	if s == "" {
		return decimal.Zero, false
	}
	d, err := decimal.NewFromString(s)
	return d, err == nil
}

// fillParams replaces an empty or unreadable number of p by its default.
func fillParams(p optiontierapi.Params) optiontierapi.Params {
	def := DefaultParams()
	if _, ok := parse(p.DeltaMax); !ok {
		p.DeltaMax = def.DeltaMax
	}
	if _, ok := parse(p.ThinDelta); !ok {
		p.ThinDelta = def.ThinDelta
	}
	if p.MaxCashAgeDays <= 0 {
		p.MaxCashAgeDays = def.MaxCashAgeDays
	}
	if p.DaysInYear <= 0 {
		p.DaysInYear = def.DaysInYear
	}
	return p
}

// rowState is one row with its verdict and the badge sets before they are folded.
type rowState struct {
	row   optiontierapi.Row
	v     optiontierapi.Verdict
	look  map[string]bool
	infos map[string]bool
}

// Judge judges every row; no row is dropped.
func (t *tierer) Judge(in optiontierapi.Input) (optiontierapi.Result, error) {
	p := fillParams(in.Params)
	accts := buildAccounts(in.Accounts, p)
	built := cashBuilt(accts)
	states := make([]*rowState, 0, len(in.Rows))
	for _, r := range in.Rows {
		s, err := t.judgeRow(r, p, accts, built)
		if err != nil {
			return optiontierapi.Result{}, fmt.Errorf("judge %s: %w", r.Key, err)
		}
		states = append(states, s)
	}
	return assemble(states, p, built), nil
}

func (t *tierer) judgeRow(r optiontierapi.Row, p optiontierapi.Params, accts []aliasCash, built bool) (*rowState, error) {
	s := &rowState{row: r, look: storedLookAt(r), infos: storedInfos(r)}
	s.v = optiontierapi.Verdict{Key: r.Key, Why: map[string]string{}}
	n := readNumbers(r)
	grey := greyReason(r, n)
	var ev optionevidenceapi.Evidence
	evaluated := false
	if needsEvaluate(grey) {
		var err error
		ev, err = t.ev.Evaluate(evidenceRow(r))
		switch {
		case err == nil:
			evaluated = true
		case errors.Is(err, optionevidenceapi.ErrBadInput):
			if grey == "" {
				grey = greyBadInput
			}
		default:
			return nil, err
		}
	}
	writeFigures(&s.v, r, n, p, ev, evaluated, grey)
	// A yen row is never compared with USD funds: its cash cells are unknown and the row has no cash figure.
	built = built && isUSD(r)
	writeCash(s, n, accts, built)
	if grey != "" {
		s.v.Tier, s.v.Grey = tierGrey, grey
		return s, nil
	}
	s.rule(n, p, built)
	return s, nil
}

// rule decides fits or check of a row whose first three conditions hold, and adds the derived badges.
func (s *rowState) rule(n numbers, p optiontierapi.Params, built bool) {
	if built && !s.cashFits(n) {
		s.look[badgeCashShort] = true
	}
	max, _ := parse(p.DeltaMax)
	thin, _ := parse(p.ThinDelta)
	if d, ok := parse(s.row.Delta); !ok {
		s.look[badgeDeltaUnknown] = true
	} else {
		abs := d.Abs()
		if abs.GreaterThan(max) {
			s.look[badgeDeltaAboveBound] = true
		}
		if abs.LessThan(thin) {
			s.v.Notes = append(s.v.Notes, noteThinPremium)
		}
	}
	s.v.Tier = tierFits
	if len(s.look) > 0 {
		s.v.Tier = tierCheck
	}
}

func evidenceRow(r optiontierapi.Row) optionevidenceapi.Row {
	return optionevidenceapi.Row{
		ChainDay: r.ChainDay, Expiry: r.Expiry, DTE: r.DTE,
		Spot: r.Spot, Strike: r.Strike, Bid: r.Bid, BuyPrice: r.BuyPrice,
		IV: r.IV, Delta: r.Delta, Multiplier: r.Multiplier,
		EarningsBeforeExpiry: r.EventsReason == eventsEarningsBeforeExpiry,
	}
}
