package option_candidates_page

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// The trial values of the tier rule that the page passes with the owner's delta bound
// (candidates-tiers.md Decisions 5, 6 and 16); the rule echoes them in Result.Params for the legend.
const (
	deltaMaxDefault  = "0.3"
	deltaMaxKey      = "put_delta_max"
	thinDeltaDefault = "0.15"
	cashAgeDays      = 3
	daysInYear       = 365
	cashTag          = "TotalCashValue"
	tierKeyHead      = "tier-short_put-"
)

// tierKey is the id of a short-put row in the tier view: unique inside the read rows.
func tierKey(c store.Candidate, i int) string {
	return tierKeyHead + strconv.Itoa(c.InstrumentID) + "-" + strconv.Itoa(i)
}

// tierFacts is the input of Judge and what the page keeps of it for the legend and the cash lines.
type tierFacts struct {
	in       optiontierapi.Input
	deltaSet bool // the bound was read through the owner settings, not the default
}

// tierInputOf builds the Judge input from the stored rows (all statuses, in stored order), the account
// readers and the owner setting. The page passes stored text unchanged and selects, sums and compares nothing.
func tierInputOf(ctx context.Context, c Config, gd contracts.Day, rows []store.Candidate, at time.Time) (tierFacts, error) {
	f := tierFacts{in: optiontierapi.Input{Day: gd, Rows: make([]optiontierapi.Row, len(rows))}}
	for i, r := range rows {
		f.in.Rows[i] = tierRowOf(r, tierKey(r, i), gd)
	}
	delta, set, err := deltaBound(ctx, c, at)
	if err != nil {
		return tierFacts{}, err
	}
	f.deltaSet = set
	f.in.Params = optiontierapi.Params{DeltaMax: delta, ThinDelta: thinDeltaDefault, MaxCashAgeDays: cashAgeDays, DaysInYear: daysInYear}
	if f.in.Accounts, err = accountsOf(ctx, c, gd); err != nil {
		return tierFacts{}, err
	}
	return f, nil
}

// tierRowOf is one stored row as the rule's row. A row whose gates or marks cannot be read is passed
// with empty reasons and the mark unreadable_gates, which the rule turns into the grey bad_input.
func tierRowOf(c store.Candidate, key string, gd contracts.Day) optiontierapi.Row {
	row := optiontierapi.Row{
		Key: key, Company: strconv.Itoa(c.UnderlyingID), ChainDay: gd, Expiry: c.Expiry, DTE: c.DTE,
		Spot: deref(c.Spot), Strike: c.Strike, Bid: c.BidEod, BuyPrice: deref(c.BuyPrice),
		IV: deref(c.Iv), Delta: deref(c.Delta), Multiplier: strconv.Itoa(c.Multiplier),
	}
	gates, gErr := parseGates(c.Gates)
	marks, mErr := parseMarks(c.Marks)
	if gErr != nil || mErr != nil {
		row.Marks = []string{unreadableGates}
		return row
	}
	row.Marks = marks
	row.QuoteReason, row.EventsReason = firstReason(gates, "quote"), firstReason(gates, "events")
	return row
}

// firstReason is the reason code of the first gate of that name that is not a pass, "" when none.
func firstReason(gates []gateResult, name string) string {
	for _, g := range gates {
		if g.Gate == name && g.Status != "pass" {
			return g.Reason
		}
	}
	return ""
}

// deltaBound is the delta bound text and whether it came through the owner settings: the stored
// put_delta_max resolved by RuleParams, or the default when either reader is nil. A reader error is returned.
func deltaBound(ctx context.Context, c Config, at time.Time) (string, bool, error) {
	if c.Owner == nil || c.RuleParams == nil {
		return deltaMaxDefault, false, nil
	}
	params, err := c.Owner.Params(ctx, at)
	if err != nil {
		return "", false, fmt.Errorf("option_candidates_page: read owner settings: %w", err)
	}
	stored := make(map[string]string, len(params))
	for _, p := range params {
		stored[p.Key] = p.Value
	}
	if v := c.RuleParams.Resolve(stored)[deltaMaxKey]; v != "" {
		return v, true, nil
	}
	return deltaMaxDefault, false, nil
}

// accountsOf reads, per alias of c.Aliases, the newest day with a TotalCashValue among the group day
// and the cashAgeDays days before it; the page counts its own probe steps and does no date arithmetic
// beyond daysBack. A nil Account or no alias gives no account.
func accountsOf(ctx context.Context, c Config, gd contracts.Day) ([]optiontierapi.AccountCash, error) {
	if c.Account == nil {
		return nil, nil
	}
	out := make([]optiontierapi.AccountCash, 0, len(c.Aliases))
	for _, alias := range c.Aliases {
		a := optiontierapi.AccountCash{Alias: alias}
		for age := 0; age <= cashAgeDays; age++ {
			day, err := daysBack(gd, age)
			if err != nil {
				return nil, fmt.Errorf("option_candidates_page: bad day %q: %w", gd, err)
			}
			vals, err := c.Account.Values(ctx, alias, string(day))
			if err != nil {
				return nil, fmt.Errorf("option_candidates_page: read account values of %s: %w", alias, err)
			}
			v, ok := cashValue(vals)
			if !ok {
				continue
			}
			pos, err := c.Account.Positions(ctx, alias, string(day))
			if err != nil {
				return nil, fmt.Errorf("option_candidates_page: read account positions of %s: %w", alias, err)
			}
			a.Cash, a.Currency, a.CaptureDay, a.AgeDays = v.Value, v.Currency, day, age
			a.Positions = positionTexts(pos)
			break
		}
		out = append(out, a)
	}
	return out, nil
}

// cashValue is the first stored TotalCashValue row with a value.
func cashValue(vals []store.AccountValue) (store.AccountValue, bool) {
	for _, v := range vals {
		if v.Tag == cashTag && v.Value != "" {
			return v, true
		}
	}
	return store.AccountValue{}, false
}

// positionTexts passes every stored position as text; the rule picks the short puts.
func positionTexts(pos []store.Position) []optiontierapi.Position {
	out := make([]optiontierapi.Position, len(pos))
	for i, p := range pos {
		out[i] = optiontierapi.Position{SecType: p.SecType, Right: p.Right, Strike: p.Strike,
			Multiplier: p.Multiplier, Quantity: p.Quantity, Currency: p.Currency}
	}
	return out
}
