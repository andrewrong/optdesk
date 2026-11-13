package strategyresultpg

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/strategyresult"
)

// shortPut is the strategy id, and the candidate kind, that the results answer.
const shortPut = "short_put"

// legJSON is one element of Row.Legs: the fields of strategy.Leg in snake case.
type legJSON struct {
	InstrumentID string `json:"instrument_id"`
	Side         string `json:"side"`
	Right        string `json:"right"`
	Strike       string `json:"strike"`
	Expiry       string `json:"expiry"`
	Multiplier   string `json:"multiplier"`
	Quantity     int    `json:"quantity"`
}

// checkJSON is one element of Row.Checks and, byte for byte, of the gates column of option_candidate.
type checkJSON struct {
	Gate   string `json:"gate"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type candidateReader struct {
	res strategyresult.Reader
	old store.CandidateReader
}

// NewCandidateReader answers Candidates for kind short_put from the rows of strategy short_put when the result store
// holds a run of that day, and hands every other kind and every day with no result run to old unchanged. A failure
// of the result reader is returned, not hidden behind old.
func NewCandidateReader(res strategyresult.Reader, old store.CandidateReader) store.CandidateReader {
	return &candidateReader{res: res, old: old}
}

func (c *candidateReader) Candidates(ctx context.Context, day contracts.Day, kind string) ([]store.Candidate, error) {
	if kind != shortPut {
		return c.old.Candidates(ctx, day, kind)
	}
	rows, err := c.res.Results(ctx, day, shortPut)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return c.old.Candidates(ctx, day, kind)
	}
	return candidatesOf(rows)
}

func candidatesOf(rows []strategyresult.Row) ([]store.Candidate, error) {
	out := make([]store.Candidate, 0, len(rows))
	for _, r := range rows {
		c, err := toCandidate(r)
		if err != nil {
			return nil, fmt.Errorf("strategyresultpg: row %q: %w", r.Key, err)
		}
		out = append(out, c)
	}
	return out, nil
}

// toCandidate rebuilds every column of a candidates.option_candidate row of kind short_put from a result row: the
// columns of the row, the single leg for the contract, and Facts for the rest (an absent key is a nil pointer, or
// empty text for a column that is not optional).
func toCandidate(r strategyresult.Row) (store.Candidate, error) {
	var legs []legJSON
	if err := json.Unmarshal([]byte(r.Legs), &legs); err != nil {
		return store.Candidate{}, fmt.Errorf("legs: %w", err)
	}
	if len(legs) != 1 {
		return store.Candidate{}, fmt.Errorf("legs: %d legs, want 1", len(legs))
	}
	leg := legs[0]
	var checks []checkJSON
	if err := json.Unmarshal([]byte(r.Checks), &checks); err != nil {
		return store.Candidate{}, fmt.Errorf("checks: %w", err)
	}
	if checks == nil {
		checks = []checkJSON{}
	}
	gates, _ := json.Marshal(checks)
	var facts map[string]string
	if err := json.Unmarshal([]byte(r.Facts), &facts); err != nil {
		return store.Candidate{}, fmt.Errorf("facts: %w", err)
	}
	id, err := strconv.Atoi(leg.InstrumentID)
	if err != nil {
		return store.Candidate{}, fmt.Errorf("leg instrument id is not a number")
	}
	mult, err := strconv.Atoi(strings.SplitN(leg.Multiplier, ".", 2)[0])
	if err != nil {
		return store.Candidate{}, fmt.Errorf("leg multiplier is not a number")
	}
	dte := 0
	if v, ok := facts["dte"]; ok {
		if dte, err = strconv.Atoi(v); err != nil {
			return store.Candidate{}, fmt.Errorf("facts dte is not a number")
		}
	}
	opt := func(k string) *string {
		if v, ok := facts[k]; ok {
			return &v
		}
		return nil
	}
	optDay := func(k string) *contracts.Day {
		if v, ok := facts[k]; ok {
			d := contracts.Day(v)
			return &d
		}
		return nil
	}
	return store.Candidate{
		SessionDay: r.SessionDay, RunID: r.RunID, Kind: shortPut, InstrumentID: id, Alias: "",
		RuleVersion: r.RuleVersion, Issuer: facts["issuer"], UnderlyingID: r.UnderlyingID,
		Expiry: contracts.Day(leg.Expiry), DTE: dte, Right: "put", Strike: leg.Strike, Multiplier: mult,
		BidEod: facts["bid"], Ask: opt("ask"), Spot: opt("spot"), BuyPrice: opt("buy_price"),
		SalePrice: opt("sale_price"), BreakEven: facts["break_even"], AnnualizedYield: opt("annualized_yield"),
		Delta: opt("delta"), Iv: opt("iv"), Status: r.Status, Gates: string(gates), Marks: facts["marks"],
		Fits: facts["fits"], NextEarnings: optDay("next_earnings"), EarningsState: facts["earnings_state"],
		NextExDate: optDay("next_ex_date"), ExAmount: opt("ex_amount"), Extrinsic: opt("extrinsic"),
		Rank: r.Rank, RankInUnderlying: r.RankInUnderlying, ComputedAt: r.ComputedAt,
		SettingsVersion: r.SettingsVersion,
	}, nil
}

// Compare lists the differences between the short put rows old holds for day and the rows new holds, rebuilt as
// candidates. The rows are compared as a sequence in the order of the reader (rank, instrument id, alias), column by
// column except RunID and ComputedAt; a line names the key and the column, and carries both values except for the
// fits column. A row present on one side only is a line of its own. An empty slice means the two agree.
func Compare(old store.CandidateReader, new strategyresult.Reader, day contracts.Day) ([]string, error) {
	ctx := context.Background()
	oldRows, err := old.Candidates(ctx, day, shortPut)
	if err != nil {
		return nil, fmt.Errorf("strategyresultpg: compare: old reader: %w", err)
	}
	rows, err := new.Results(ctx, day, shortPut)
	if err != nil {
		return nil, fmt.Errorf("strategyresultpg: compare: result reader: %w", err)
	}
	newRows, err := candidatesOf(rows)
	if err != nil {
		return nil, err
	}
	lines := []string{}
	for i := 0; i < len(oldRows) || i < len(newRows); i++ {
		switch {
		case i >= len(newRows):
			lines = append(lines, fmt.Sprintf("row %d key %s: only in the old rows", i, candidateKey(oldRows[i])))
		case i >= len(oldRows):
			lines = append(lines, fmt.Sprintf("row %d key %s: only in the result rows", i, candidateKey(newRows[i])))
		default:
			lines = append(lines, diffRow(i, oldRows[i], newRows[i])...)
		}
	}
	return lines, nil
}

func candidateKey(c store.Candidate) string {
	if c.Alias == "" {
		return strconv.Itoa(c.InstrumentID)
	}
	return strconv.Itoa(c.InstrumentID) + ":" + c.Alias
}

func diffRow(i int, a, b store.Candidate) []string {
	var lines []string
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	for f := 0; f < va.NumField(); f++ {
		name := va.Type().Field(f).Name
		if name == "RunID" || name == "ComputedAt" {
			continue
		}
		x, y := va.Field(f).Interface(), vb.Field(f).Interface()
		if reflect.DeepEqual(x, y) {
			continue
		}
		if name == "Fits" {
			lines = append(lines, fmt.Sprintf("row %d key %s: column %s differs", i, candidateKey(a), name))
			continue
		}
		lines = append(lines, fmt.Sprintf("row %d key %s: column %s differs: old %s, new %s", i, candidateKey(a), name,
			show(va.Field(f)), show(vb.Field(f))))
	}
	return lines
}

// show prints a column value, a nil pointer as nil and any other pointer as its target.
func show(v reflect.Value) string {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return "nil"
		}
		v = v.Elem()
	}
	return fmt.Sprintf("%v", v.Interface())
}
