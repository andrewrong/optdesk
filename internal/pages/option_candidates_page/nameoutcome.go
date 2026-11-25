package option_candidates_page

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute/optioncandapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// nameChunk is the most instrument ids one InstrumentNames call accepts.
const nameChunk = 500

// nameOutcome is the outcome of one Japanese name with no candidate row (sellput-jp-outcomes.md Decisions 1, 3, 4).
type nameOutcome struct {
	Name    string `json:"name"`
	Outcome string `json:"outcome"`
	Text    string `json:"text"`
	Counts  string `json:"counts"`
}

// readGroupWithOutcomes is readGroup with the name outcomes of the yen group added; any other group has none.
func readGroupWithOutcomes(ctx context.Context, c Config, series store.SeriesReader, s groupSpec, day contracts.Day) (groupRead, error) {
	gr, err := readGroup(ctx, c, series, s, day)
	if err != nil || s.currency != currencyJPY {
		return gr, err
	}
	if gr.outcomes, err = nameOutcomesOf(ctx, c, day, gr.rows); err != nil {
		return gr, fmt.Errorf("option_candidates_page: read open rows: %w", err)
	}
	return gr, nil
}

// symbolIDs maps the upper-case symbol of each stock among ids to its instrument id.
func symbolIDs(ctx context.Context, r store.InstrumentNameReader, ids []int64) (map[string]int, error) {
	sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
	bySymbol := map[string]int{}
	for start := 0; start < len(ids); start += nameChunk {
		names, err := r.InstrumentNames(ctx, ids[start:min(start+nameChunk, len(ids))])
		if err != nil {
			return nil, err
		}
		for _, n := range names {
			if n.Type == "stock" {
				bySymbol[strings.ToUpper(n.Name)] = int(n.InstrumentID)
			}
		}
	}
	return bySymbol, nil
}

// openFactsOf are the plain facts of open rows.
func openFactsOf(rs []contracts.OptionOpen) []optioncandapi.OpenFact {
	facts := make([]optioncandapi.OpenFact, len(rs))
	for i, r := range rs {
		facts[i] = optioncandapi.OpenFact{Status: r.Status, OutsideBand: r.OutsideBand}
		if r.ErrorCode != nil {
			facts[i].ErrorCode = *r.ErrorCode
		}
	}
	return facts
}

// nameOutcomesOf classifies, for each name of c.JPNames that has no row among rows, the stored JPY open rows of
// day. Without the open-row reader, the rule, the name reader or any name it returns nil. A reader error is
// returned.
func nameOutcomesOf(ctx context.Context, c Config, day contracts.Day, rows []store.Candidate) ([]nameOutcome, error) {
	if c.OpenRows == nil || c.NameOpen == nil || c.Names == nil || len(c.JPNames) == 0 {
		return nil, nil
	}
	rule := c.NameOpen()
	if rule == nil {
		return nil, nil
	}
	all, err := c.OpenRows.OptionOpen(ctx, day)
	if err != nil {
		return nil, err
	}
	open := map[int][]contracts.OptionOpen{}
	var ids []int64
	for _, r := range all {
		if r.Currency != currencyJPY {
			continue
		}
		if _, seen := open[r.UnderlyingID]; !seen {
			ids = append(ids, int64(r.UnderlyingID))
		}
		open[r.UnderlyingID] = append(open[r.UnderlyingID], r)
	}
	// A name with candidate rows is told apart by the underlying id of its rows, which need not have open rows.
	withRows := map[int]bool{}
	for _, r := range rows {
		if _, seen := open[r.UnderlyingID]; !seen && !withRows[r.UnderlyingID] {
			ids = append(ids, int64(r.UnderlyingID))
		}
		withRows[r.UnderlyingID] = true
	}
	bySymbol, err := symbolIDs(ctx, c.Names, ids)
	if err != nil {
		return nil, err
	}
	var out []nameOutcome
	for _, name := range c.JPNames {
		id, found := bySymbol[strings.ToUpper(name)]
		if found && withRows[id] {
			continue
		}
		var rs []contracts.OptionOpen
		if found {
			rs = open[id]
		}
		facts := openFactsOf(rs)
		res := rule.Classify(optioncandapi.NameOpenFacts{Name: name, Facts: facts})
		out = append(out, nameOutcome{Name: name, Outcome: res.Outcome, Text: outcomeText(res.Outcome, facts), Counts: countsText(res)})
	}
	return out, nil
}

// outcomeText is the Chinese words of an outcome; an outcome this page does not know reads as "other".
func outcomeText(outcome string, facts []optioncandapi.OpenFact) string {
	switch outcome {
	case optioncandapi.OutcomeNoQuote:
		return nameNoQuoteZh
	case optioncandapi.OutcomeFailed:
		return nameFailedZh
	case optioncandapi.OutcomeNoSpot:
		return nameNoSpotHeadZh + codesOf(facts, func(f optioncandapi.OpenFact) bool {
			return f.Status == 7 && f.ErrorCode != "" && f.ErrorCode != "BUDGET"
		}) + nameNoSpotTailZh
	case optioncandapi.OutcomeDeferred:
		return nameDeferredZh
	case optioncandapi.OutcomeNoCandidate:
		return nameNoCandidateZh
	case optioncandapi.OutcomeNoRows:
		return nameNoRowsZh
	}
	return nameOtherHeadZh + codesOf(facts, func(f optioncandapi.OpenFact) bool {
		switch f.Status {
		case 1, 2, 6, 7, 8, 9:
			return false
		case 3:
			return f.ErrorCode != "NO_QUOTE"
		}
		return true
	}) + nameOtherTailZh
}

// codesOf lists the distinct codes (the status number when a row has no code) of the facts that pick selects.
func codesOf(facts []optioncandapi.OpenFact, pick func(optioncandapi.OpenFact) bool) string {
	seen := map[string]bool{}
	var codes []string
	for _, f := range facts {
		if !pick(f) {
			continue
		}
		code := f.ErrorCode
		if code == "" {
			code = strconv.Itoa(f.Status)
		}
		if !seen[code] {
			seen[code] = true
			codes = append(codes, code)
		}
	}
	sort.Strings(codes)
	return strings.Join(codes, nameCodeSepZh)
}

// countsText prints the counts of every kind that is not zero, as text.
func countsText(n optioncandapi.NameOpen) string {
	var parts []string
	for _, p := range []struct {
		head string
		n    int
	}{
		{nameCountNoQuoteZh, n.NoQuote}, {nameCountFailedZh, n.Failed}, {nameCountNoSpotZh, n.NoSpot},
		{nameCountDeferredZh, n.Deferred}, {nameCountOkZh, n.Ok}, {nameCountOtherZh, n.Other},
	} {
		if p.n > 0 {
			parts = append(parts, p.head+strconv.Itoa(p.n)+nameCountUnitZh)
		}
	}
	if len(parts) == 0 {
		return nameCountNoneZh
	}
	return strings.Join(parts, nameCountSepZh)
}
