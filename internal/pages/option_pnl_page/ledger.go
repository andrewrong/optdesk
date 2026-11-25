package option_pnl_page

import (
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	// ledgerMonths is the number of months of the page day the ledger counts, the page-day month included.
	// Trial value; the plan appendix A replay (step 12) says whether another window is better. Used by
	// buildLedger and by selectLots, which reads the lots of that window.
	ledgerMonths = 6
	// ledgerMinLots is the number of split lots a ledger book needs before it stops saying "no conclusion".
	// Trial value, plan appendix A; used by buildLedger.
	ledgerMinLots = 20
)

// ledgerView is the ledger block of one account: two views over the same ended lots.
type ledgerView struct {
	Title string      `json:"title"`
	Note  string      `json:"note"`
	Views []ledgerTab `json:"views"`
}

// ledgerTab is one grouping of the ledger: by month or by company.
type ledgerTab struct {
	ID     string      `json:"id"`
	Label  string      `json:"label"`
	Groups []groupView `json:"groups"`
}

// groupView is one month or one company with one book per currency.
type groupView struct {
	ID      string     `json:"id"`
	Label   string     `json:"label"`
	Summary string     `json:"summary"`
	Books   []bookView `json:"books"`
}

// monthKey is the text YYYY-MM of the month back months before year and month, by month number.
func monthKey(year, month, back int) string {
	n := year*12 + month - 1 - back
	y, m := n/12, n%12+1
	text := strconv.Itoa(y) + "-"
	if m < 10 {
		text += "0"
	}
	return text + strconv.Itoa(m)
}

// ledgerFirst is the first day of the oldest ledger month of the page day d.
func ledgerFirst(d time.Time) contracts.Day {
	return contracts.Day(monthKey(d.Year(), int(d.Month()), ledgerMonths-1) + "-01")
}

// buildLedger groups the ended outcomes by the month of ClosedOn and by Underlying and gives one Booker
// call per group. It adds no figure and no count; open outcomes are only counted for the note.
// Design: docs/design/options-pnl-ledger.md#Decisions (2, 3, 5, 6, 8).
func buildLedger(c Config, idx int, d time.Time, outs []outcome) *ledgerView {
	months := make([]string, ledgerMonths)
	inWindow := map[string]bool{}
	for i := range months {
		months[i] = monthKey(d.Year(), int(d.Month()), i)
		inWindow[months[i]] = true
	}
	byMonth := map[string][]outcome{}
	byCompany := map[string][]outcome{}
	var companies []string
	var open []outcome
	for _, o := range outs {
		p := o.lot.pos
		if p.Status == statusOpen {
			open = append(open, o)
			continue
		}
		m := monthOfDay(p.ClosedOn)
		if !inWindow[m] {
			continue
		}
		byMonth[m] = append(byMonth[m], o)
		if _, ok := byCompany[p.Underlying]; !ok {
			companies = append(companies, p.Underlying)
		}
		byCompany[p.Underlying] = append(byCompany[p.Underlying], o)
	}
	byM := ledgerTab{ID: "by_month", Label: "By month", Groups: []groupView{}}
	var folded []outcome
	var foldedMonths []string
	for i, m := range months {
		if len(byMonth[m]) == 0 && i != 0 {
			continue
		}
		if noSplit(byMonth[m]) {
			folded = append(folded, byMonth[m]...)
			foldedMonths = append(foldedMonths, m)
			continue
		}
		byM.Groups = append(byM.Groups, ledgerGroup(c, "a"+strconv.Itoa(idx)+"m", len(byM.Groups), m, byMonth[m], false))
	}
	if len(folded) > 0 {
		g := ledgerGroup(c, "a"+strconv.Itoa(idx)+"m", len(byM.Groups), foldedLabel(foldedMonths, len(folded)), folded, false)
		g.Summary = fit(g.Summary + " · " + foldedNoteZh)
		byM.Groups = append(byM.Groups, g)
	}
	byC := ledgerTab{ID: "by_company", Label: "By company", Groups: []groupView{}}
	for _, name := range companies {
		byC.Groups = append(byC.Groups, ledgerGroup(c, "a"+strconv.Itoa(idx)+"c", len(byC.Groups), name, byCompany[name], true))
	}
	return &ledgerView{Title: "Ledger", Note: ledgerNoteZh + " · " + plural(len(open), "open lot", "open lots"),
		Views: []ledgerTab{byM, byC}}
}

// noSplit is true for a bucket with lots of which none could be split.
func noSplit(outs []outcome) bool {
	for _, o := range outs {
		if o.split() {
			return false
		}
	}
	return len(outs) > 0
}

// foldedLabel names the folded months by the first and the last of them and the lot count.
func foldedLabel(months []string, lots int) string {
	oldest, newest := months[len(months)-1], months[0] // months run newest first
	span := oldest
	if newest != oldest {
		span += " ~ " + newest
	}
	return foldedMonthsZh + " " + span + " · " + plural(lots, "lot", "lots")
}

func monthOfDay(day contracts.Day) string {
	if len(day) < 7 {
		return ""
	}
	return string(day[:7])
}

// ledgerGroup is one group: one Combine of the results of the bucket, one book per currency in order of
// first appearance. span adds the real span of ClosedOn of the bucket to the summary.
func ledgerGroup(c Config, tok string, gi int, label string, outs []outcome, span bool) groupView {
	g := groupView{ID: "grp-" + tok + strconv.Itoa(gi), Label: fit(label), Books: []bookView{}}
	if len(outs) == 0 {
		g.Summary = noEndedMonthZh
		return g
	}
	var results []optionattrapi.Result
	var order []string
	seen := map[string]bool{}
	first, last := outs[0].lot.pos.ClosedOn, outs[0].lot.pos.ClosedOn
	for _, o := range outs {
		if !o.isGap() {
			results = append(results, o.res)
		}
		if cur := o.lot.pos.Currency; !seen[cur] {
			seen[cur] = true
			order = append(order, cur)
		}
		if day := o.lot.pos.ClosedOn; day < first {
			first = day
		} else if day > last {
			last = day
		}
	}
	byCur := map[string]optionattrapi.Book{}
	if len(results) > 0 {
		for _, b := range c.Booker.Combine(results) {
			byCur[b.Currency] = b
		}
	}
	summary := ""
	for ci, cur := range order {
		b, ok := byCur[cur]
		if !ok {
			// every lot of the currency is a gap lot: no lot was added
			b = optionattrapi.Book{Currency: cur, RentMinusCost: "0", LedgerAllPnL: "0"}
		}
		var curOuts []outcome
		for _, o := range outs {
			if o.lot.pos.Currency == cur {
				curOuts = append(curOuts, o)
			}
		}
		g.Books = append(g.Books, ledgerBook(tok, gi, ci, cur, curOuts, b))
		if ci > 0 {
			summary += "; "
		}
		summary += cur + ", " + plural(b.Lots, "split lot", "split lots")
		if !span {
			summary += ", " + plural(len(curOuts), "ended lot", "ended lots")
		}
	}
	if span {
		summary += ", lots ended " + string(first) + " to " + string(last)
	}
	g.Summary = fit(summary)
	return g
}

// lotsTail is " · n lots" (" · 1 lot").
func lotsTail(n int) string { return " · " + plural(n, "lot", "lots") }

// ledgerHints maps the id suffix of a ledger line to its Chinese hint.
var ledgerHints = map[string]string{
	"-head":     ledgerHintRentZh,
	"-all":      ledgerHintLedgerAllZh,
	"-time":     ledgerHintTimeZh,
	"-move":     ledgerHintMoveCostZh,
	"-dir":      ledgerHintDirectionZh,
	"-iv":       ledgerHintIVRemarkZh,
	"-entry":    ledgerHintEntryZh,
	"-exit":     ledgerHintExitZh,
	"-total":    ledgerHintTotalZh,
	"-assigned": ledgerHintAssignedZh,
	"-ledger":   ledgerHintLedgerSaysZh,
}

// ledgerBook is the book of one currency of one group: headline, the Sample line under 20 split lots,
// the ledger-all line, the split lines and the "Ledger says" line, each with its lot count. It does not
// use bookCards: its empty-book test reads Total, which Combine gives as "0".
func ledgerBook(tok string, gi, ci int, cur string, outs []outcome, b optionattrapi.Book) bookView {
	gs, cs := strconv.Itoa(gi), strconv.Itoa(ci)
	prefix := "lg" + tok + gs + "-" + cs
	v := bookView{ID: "book-lg-" + tok + "-" + gs + "-" + cs, Currency: cur, Lines: []line{}}
	v.Counts = cur + ", " + plural(b.Lots, "split lot", "split lots") + " of " + strconv.Itoa(len(outs)) + " ended"
	v.LeftOut = leftOut(outs, b, cur)
	few := b.Lots < ledgerMinLots
	f := figuresOfBook(b)
	split := lotsTail(b.Lots)
	tone := func(l line, base string) line {
		if few {
			return l.tone("muted")
		}
		return l.tone(base)
	}
	v.Headline = tone(mkLine(prefix+"-head", lblRentZh, money(f.RentMinusCost, cur)+split, ""), toneOf(f.RentMinusCost))
	lines := []line{}
	if few {
		text := noConclusionZh + " (" + plural(b.Lots, "split lot", "split lots") + ", fewer than " + strconv.Itoa(ledgerMinLots) + ")"
		lines = append(lines, mkLine(prefix+"-sample", lblSampleZh, text, "").tone("muted"))
	}
	lines = append(lines, mkLine(prefix+"-all", lblAllZh, money(b.LedgerAllPnL, cur)+lotsTail(b.AllLots), ""))
	if b.Lots > 0 {
		for _, l := range splitLines(prefix, f, cur, true) {
			l.Text += split
			lines = append(lines, tone(l, l.Tone))
		}
		ledger := ledgerLine(prefix, f, cur, true, true)
		ledger.Text += split
		lines = append(lines, tone(ledger, ledger.Tone))
	}
	for i := range lines {
		lines[i].Hint = hintOf(prefix, lines[i].ID)
	}
	withEntryHint(lines, prefix+"-entry", lateEntryHint(outs, len(outs)))
	v.Headline.Hint = hintOf(prefix, v.Headline.ID)
	v.Lines = lines
	return v
}

// hintOf is the Chinese hint of the line with id, or nil for a line with no jargon label.
func hintOf(prefix, id string) *string {
	if h, ok := ledgerHints[id[len(prefix):]]; ok {
		return &h
	}
	return nil
}
