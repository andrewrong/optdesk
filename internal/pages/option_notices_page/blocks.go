package option_notices_page

import (
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
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
	Day        string   `json:"day"`
	Currency   string   `json:"currency"`
	AmountText string   `json:"amount_text"`
	Puts       []putRow `json:"puts"`
}

// putRow is one stored short put behind a cash day, as stored text; no per-put figure.
type putRow struct {
	Symbol       string `json:"symbol"`
	Expiry       string `json:"expiry"`
	StrikeText   string `json:"strike_text"`
	Right        string `json:"right"`
	QuantityText string `json:"quantity_text"`
}

// putsOf lists the short puts of rows that expire on day in currency, in stored order.
func putsOf(rows []store.Position, day contracts.Day, currency string) []putRow {
	out := []putRow{}
	for _, p := range rows {
		if (p.SecType != storedOption && p.SecType != "OPT") || (p.Right != "P" && !strings.EqualFold(p.Right, "put")) ||
			p.Expiry != string(day) || p.Currency != currency {
			continue
		}
		qty, short := strings.CutPrefix(p.Quantity, "-")
		if !short || strings.Trim(qty, "0.") == "" {
			continue
		}
		out = append(out, putRow{Symbol: clip(p.Symbol), Expiry: p.Expiry, StrikeText: clip(p.Strike), Right: "P",
			QuantityText: clip(qty)})
	}
	return out
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
	CushionHint *string     `json:"cushion_hint"`
	NoteHint    *string     `json:"note_hint"`
}

// noCaptureBlock is the block of an alias with no stored capture.
func noCaptureBlock(alias string, main bool) aliasBlock {
	note := noteNoCapture
	return aliasBlock{Alias: clip(alias), Main: main, Status: "no_capture", Notices: []noticeRow{},
		CashByDate: []cashRow{}, Note: &note}
}

// okBlock writes the block of an alias from the rule's result, in the rule's order. The status is
// stale when the data day is older than the build day. note is empty for no note.
func okBlock(alias string, main bool, dataDay, buildDay contracts.Day, res compute.NoticeResult, note string,
	positions []store.Position, params compute.NoticeParams) aliasBlock {
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
			AmountText: clip(amountText(c.Amount, c.Currency)), Puts: putsOf(positions, c.Day, c.Currency)})
	}
	if res.Cushion != "" {
		t := clip(cushionHead + percentText(res.Cushion))
		b.CushionText = &t
		h := cushionHintText(params)
		b.CushionHint = &h
	}
	if note != "" {
		b.Note = &note
		if note == noteNoGuard {
			h := noteHint
			b.NoteHint = &h
		}
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
