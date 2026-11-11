package option_notices_page

import (
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

type gap struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

func mkGap(reason, text string) gap { return gap{Kind: "gap", Reason: reason, Text: text} }

type fact struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}

type noticeRow struct {
	ID       string   `json:"id"`
	Code     string   `json:"code"`
	Severity string   `json:"severity"`
	Title    string   `json:"title"`
	Contract string   `json:"contract"`
	Expiry   *string  `json:"expiry"`
	Facts    []fact   `json:"facts"`
	Reasons  []string `json:"reasons"`
}

type cashRow struct {
	Day        string `json:"day"`
	Currency   string `json:"currency"`
	AmountText string `json:"amount_text"`
}

type aliasBlock struct {
	Alias       string      `json:"alias"`
	Main        bool        `json:"main"`
	Status      string      `json:"status"`
	DataDay     *string     `json:"data_day"`
	Notices     []noticeRow `json:"notices"`
	CashByDate  []cashRow   `json:"cash_by_date"`
	CushionText *string     `json:"cushion_text"`
	Note        *string     `json:"note"`
}

// noCaptureBlock is the block of an alias with no stored capture.
func noCaptureBlock(alias string, main bool) aliasBlock {
	note := noteNoCapture
	return aliasBlock{Alias: clip(alias), Main: main, Status: "no_capture", Notices: []noticeRow{},
		CashByDate: []cashRow{}, Note: &note}
}

// okBlock writes the block of an alias from the rule's result, in the rule's order. The status is
// stale when the data day is older than the build day. note is empty for no note.
func okBlock(alias string, main bool, dataDay, buildDay contracts.Day, res compute.NoticeResult, note string) aliasBlock {
	b := aliasBlock{Alias: clip(alias), Main: main, Status: "ok", Notices: make([]noticeRow, 0, len(res.Notices)),
		CashByDate: make([]cashRow, 0, len(res.CashByDate))}
	if dataDay < buildDay {
		b.Status = "stale"
	}
	dd := string(dataDay)
	b.DataDay = &dd
	for i, n := range res.Notices {
		b.Notices = append(b.Notices, noticeRowOf(alias, i, n))
	}
	for _, c := range res.CashByDate {
		b.CashByDate = append(b.CashByDate, cashRow{Day: string(c.Day), Currency: clip(c.Currency),
			AmountText: clip(amountText(c.Amount, c.Currency))})
	}
	if res.Cushion != "" {
		t := clip(cushionHead + percentText(res.Cushion))
		b.CushionText = &t
	}
	if note != "" {
		b.Note = &note
	}
	return b
}

func noticeRowOf(alias string, i int, n compute.Notice) noticeRow {
	row := noticeRow{ID: noticeID(alias, i, n.Code), Code: n.Code, Severity: n.Severity,
		Title: clip(noticeTitle(n.Code)), Contract: clip(n.Contract), Facts: []fact{}, Reasons: []string{}}
	if n.Expiry != "" {
		e := string(n.Expiry)
		row.Expiry = &e
	}
	for _, f := range n.Facts {
		if f.Label == "reason" {
			row.Reasons = append(row.Reasons, clip(reasonText(f.Value)))
			continue
		}
		text := f.Value
		if f.Label == "cushion" {
			text = percentText(text)
		}
		row.Facts = append(row.Facts, fact{Label: clip(factLabel(f.Label)), Text: clip(text)})
	}
	return row
}
