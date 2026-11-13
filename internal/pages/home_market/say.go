package home_market

import (
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// The four sentences of Decision 9 of docs/design/market-state-tech.md, the only Chinese text of this package.
// They are fixed text keyed by the state, not stored and not composed in the browser.
const (
	sayLeading    = "科技近 1 个月、3 个月都跑赢大盘（试行，不是预测）。"
	sayInLine     = "科技与大盘大体同步（试行，不是预测）。"
	sayLagging    = "科技近 1 个月、3 个月都跑输大盘。科技股的新 put：先等一等，或减量、选更远的行权价（试行，不是预测）。"
	sayLaggingHrd = "科技近 3 个月明显跑输大盘。科技股的新 put：先等一等，或减量、选更远的行权价（试行，不是预测）。"
)

// The English line of the numbers: the stored gap tags are put in as they are.
const (
	numbersHead = "NDX minus SPX: 1 month "
	numbersMid  = " pt, 3 months "
	numbersTail = " pt; trial thresholds, not a forecast"
)

var sentences = map[string]string{
	"LEADING":      sayLeading,
	"IN LINE":      sayInLine,
	"LAGGING":      sayLagging,
	"LAGGING HARD": sayLaggingHrd,
}

// View is what a page shows of one stored reading: State empty and Gap set when there is nothing to show.
type View struct {
	State, Gap, Say, Numbers string
}

// FromReading reads a stored market.tech_state reading for the page day: the state is its `state` tag. A data day
// more than 5 calendar days before day, or a stored reason code 3, is stale_input; a reading that is not ok, has no
// state tag or an unknown one is missing_input; no reading is no_data. Say is the Chinese sentence of Decision 9;
// Numbers is the English line of Decision 12.
func FromReading(r *contracts.ReadingRecord, day contracts.Day) View {
	if r == nil {
		return View{Gap: "no_data"}
	}
	if string(r.State) == "missing_input" && r.ReasonCode != nil && *r.ReasonCode == codeStaleInput {
		return View{Gap: "stale_input"}
	}
	state, g1, g3 := tag(r, "state"), tag(r, "gap_1m"), tag(r, "gap_3m")
	sentence, known := sentences[state]
	data, hasDay := dataDayOf(r)
	page, err := time.Parse(dayLayout, string(day))
	if string(r.State) != "ok" || !known || g1 == "" || g3 == "" || !hasDay || err != nil {
		return View{Gap: "missing_input"}
	}
	if daysBetween(data, page) > readingDays {
		return View{Gap: "stale_input"}
	}
	return View{State: state, Say: sentence, Numbers: numbersHead + g1 + numbersMid + g3 + numbersTail}
}

// tag is the value_tag of the named parameter, empty when absent.
func tag(r *contracts.ReadingRecord, name string) string {
	for _, p := range r.Parameters {
		if p.Param == name && p.ValueTag != nil {
			return *p.ValueTag
		}
	}
	return ""
}
