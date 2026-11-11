package structures_page

import "encoding/json"

type sharePercent struct {
	Value json.Number `json:"value"`
	Text  string      `json:"text"`
	Unit  string      `json:"unit"`
}

type stage struct {
	Name     string       `json:"name"`
	DateText string       `json:"date_text"`
	Elapsed  sharePercent `json:"elapsed"`
	DaysText string       `json:"days_text"`
}

// The two ends of the timeline are fixed text: compute gives no day count and no elapsed share, and a page
// computes nothing (Decision 12), so no stage for today and no count of days is written.
var (
	startShare = sharePercent{Value: "0", Text: "0%", Unit: "share_percent"}
	endShare   = sharePercent{Value: "100", Text: "100%", Unit: "share_percent"}
)

// timeline is the two stages of the structure, the day it was opened and its first expiry, both dates copied
// from the ledger piece and the legs. Without an opening day there are no stages and a named gap.
func (b *builder) timeline() []stage {
	opened := ""
	for _, l := range b.s.Legs {
		if d := string(b.pos[l.PositionID].OpenedOn); d != "" && (opened == "" || d < opened) {
			opened = d
		}
	}
	expiry := earliestExpiry(b.s)
	if opened == "" || expiry == "-" {
		b.addGap(mkGap(reasonMissing))
		return []stage{}
	}
	return []stage{
		{Name: "Open", DateText: opened, Elapsed: startShare, DaysText: "Open day"},
		{Name: "Expiry", DateText: expiry, Elapsed: endShare, DaysText: "Expiry day"},
	}
}
