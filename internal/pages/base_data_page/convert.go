package base_data_page

import (
	"fmt"

	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/basefact"
	"github.com/andrewrong/optdesk/internal/store/coverage"
)

// convertFact copies a store fact into the compute type, field by field. A field added to basefact.Fact is added here.
func convertFact(f basefact.Fact) basecheckapi.Fact {
	return basecheckapi.Fact{Dataset: f.Dataset, Rule: f.Rule, Subject: f.Subject, N: f.N, Bad: f.Bad, Day: f.Day, Value: f.Value}
}

func convertFacts(fs []basefact.Fact) []basecheckapi.Fact {
	out := make([]basecheckapi.Fact, 0, len(fs))
	for _, f := range fs {
		out = append(out, convertFact(f))
	}
	return out
}

// sumCells adds the ledger cells of the datasets of each row (from basecheckapi.Rows) into one cell per row and day,
// named by the row id. Cells come ascending by day; the result is ascending by day, then in the order of Rows.
func sumCells(cells []coverage.Cell) []basecheckapi.Cell {
	rowOf := map[string]string{}
	for _, r := range basecheckapi.Rows {
		for _, d := range r.LedgerDatasets {
			rowOf[d] = r.ID
		}
	}
	type key struct {
		day contracts.Day
		row string
	}
	sums := map[key]*basecheckapi.Cell{}
	var days []contracts.Day
	seenDay := map[contracts.Day]bool{}
	for _, c := range cells {
		row, ok := rowOf[c.Dataset]
		if !ok {
			continue
		}
		k := key{c.Day, row}
		s := sums[k]
		if s == nil {
			s = &basecheckapi.Cell{Day: c.Day, Dataset: row}
			sums[k] = s
		}
		if !seenDay[c.Day] {
			seenDay[c.Day] = true
			days = append(days, c.Day)
		}
		s.Keys += c.Keys
		s.Missing += c.Missing
		s.Expired += c.Expired
	}
	sortDays(days)
	out := []basecheckapi.Cell{}
	for _, d := range days {
		for _, r := range basecheckapi.Rows {
			if s := sums[key{d, r.ID}]; s != nil {
				out = append(out, *s)
			}
		}
	}
	return out
}

// pairLevelOf is the level of the pair of a cross_check, as Eval takes it for the row the pair is shown on.
func pairLevelOf(row string, cc crossCheckVM) basecheckapi.PairLevel {
	return basecheckapi.PairLevel{Dataset: row, Pair: cc.PairID, Level: basecheckapi.Level(cc.Level)}
}

// attachCrossCheck carries the cross_check object to the payload of the row with the given id. An unknown
// cross-check is also named in the failed_summary of the row, with its reason.
func attachCrossCheck(groups []groupVM, row string, cc *crossCheckVM) {
	if cc == nil {
		return
	}
	for gi := range groups {
		for ri := range groups[gi].Rows {
			if groups[gi].Rows[ri].Dataset == row {
				groups[gi].Rows[ri].CrossCheck = cc
				if cc.Level == string(basecheckapi.Unknown) {
					part := fmt.Sprintf(crossUnknownFormat, cc.PairID, cc.ReasonZh)
					if s := groups[gi].Rows[ri].FailedSummary; s != "" {
						part = s + "; " + part
					}
					groups[gi].Rows[ri].FailedSummary = part
				}
			}
		}
	}
}
