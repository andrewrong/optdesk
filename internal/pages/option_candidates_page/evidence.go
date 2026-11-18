package option_candidates_page

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/andrewrong/optdesk/internal/compute/optionevidenceapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// evState is what the evidence of one short-put row came to.
type evState int

const (
	evOK         evState = iota
	evNoReader           // nil Evaluator
	evNoSpot             // no stored spot
	evUnreadable         // Evaluate answered ErrBadInput
)

// rowEvidence is the evidence of one short-put row, as the Evaluator answered it with the readings
// and the ATM pair that went into it.
type rowEvidence struct {
	state       evState
	ev          optionevidenceapi.Evidence
	rd          nameReadings
	atmStrike   string
	atmNoReader bool // nil ATMFor
}

// evidenceSet is the evidence of the rows it was built for, by the row's position in them;
// the zero value (no rows) makes every evidence cell the gap no_reader.
type evidenceSet struct {
	rows     []rowEvidence
	chainDay contracts.Day
	rateDay  contracts.Day
}

// evidenceInputs are the readers the evidence needs; each nil gives the gap no_reader.
type evidenceInputs struct {
	eval     optionevidenceapi.Evaluator
	atm      ATMFunc
	readings store.ReadingViewReader
	names    store.InstrumentNameReader
}

func inputsOf(c Config) evidenceInputs {
	return evidenceInputs{eval: c.Evidence, atm: c.ATMFor, readings: c.Readings, names: c.Names}
}

// evidenceOf evaluates the given short-put rows, whatever their stored status, once per row, in row order. Any reader error
// and any Evaluate error but ErrBadInput is returned.
func evidenceOf(ctx context.Context, in evidenceInputs, tickers map[int]string, gd contracts.Day, rate rateRead, rows []store.Candidate) (evidenceSet, error) {
	set := evidenceSet{chainDay: gd, rateDay: rate.day}
	ratePct := ""
	if oldest, err := daysBack(gd, maxRateAgeDays); err == nil && rate.day != "" && rate.day >= oldest {
		ratePct = rate.text
	}
	var err error
	byName := map[int]nameReadings{}
	for _, c := range rows {
		r := rowEvidence{}
		switch spot := deref(c.Spot); {
		case spot == "":
			r.state = evNoSpot
		case in.eval == nil:
			r.state = evNoReader
		default:
			if r.rd, err = readingsOfRow(ctx, in, tickers, byName, c, gd); err != nil {
				return evidenceSet{}, err
			}
			row := optionevidenceapi.Row{
				ChainDay: gd, Expiry: c.Expiry, DTE: c.DTE, Spot: spot, Strike: c.Strike, Bid: c.BidEod,
				BuyPrice: deref(c.BuyPrice), IV: deref(c.Iv), Delta: deref(c.Delta),
				Multiplier: strconv.Itoa(c.Multiplier), RatePercent: ratePct,
				Forecast: r.rd.forecast.text, ForecastErr: r.rd.forecastErr.text, IVP: r.rd.ivp.text, VolEdge: r.rd.volEdge.text,
			}
			if c.NextExDate != nil {
				row.ExDiv = *c.NextExDate
			}
			if in.atm == nil {
				r.atmNoReader = true
			} else if row.ATMIV, row.ATMStrike, err = in.atm(ctx, c.UnderlyingID, gd, c.Expiry, spot); err != nil {
				return evidenceSet{}, fmt.Errorf("option_candidates_page: atm quote: %w", err)
			}
			r.atmStrike = row.ATMStrike
			switch ev, err := in.eval.Evaluate(row); {
			case err == nil:
				r.ev = ev
			case errors.Is(err, optionevidenceapi.ErrBadInput):
				r.state = evUnreadable
			default:
				return evidenceSet{}, fmt.Errorf("option_candidates_page: evidence: %w", err)
			}
		}
		set.rows = append(set.rows, r)
	}
	return set, nil
}

// readingsOfRow is the readings of the row's name, read once per name of the group. Without a reading
// reader or a ticker nothing is read and the gap text says which is missing.
func readingsOfRow(ctx context.Context, in evidenceInputs, tickers map[int]string, cache map[int]nameReadings,
	c store.Candidate, gd contracts.Day) (nameReadings, error) {
	switch {
	case in.readings == nil:
		return nameReadings{noReader: gapNoReadings}, nil
	case in.names == nil || tickers[c.UnderlyingID] == "":
		return nameReadings{noReader: gapNoTicker}, nil
	}
	if r, ok := cache[c.UnderlyingID]; ok {
		return r, nil
	}
	r, err := readingsFor(ctx, in.readings, tickers[c.UnderlyingID], gd)
	if err != nil {
		return nameReadings{}, err
	}
	cache[c.UnderlyingID] = r
	return r, nil
}
