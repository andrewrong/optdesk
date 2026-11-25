package option_pnl_page

import (
	"errors"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
)

// lotCard is the card of one lot: a gap card with its reason, a no-quotes card with the ledger figures,
// an unreconciled card with its split and the reason, or the full split (Combine of one result).
func lotCard(c Config, prefix, alias string, o outcome, isRoll bool) (lotView, error) {
	p := o.lot.pos
	cur := p.Currency
	card := lotView{ID: "lot-" + safeID(alias+"-"+p.PositionID, 92), Title: lotTitle(o), Summary: lotSummary(o),
		Lines: []line{}, Recon: []line{}, Badges: []string{}}
	if isRoll {
		card.Badges = append(card.Badges, badgeRoll)
	}
	if o.isGap() {
		card.Summary += " · Ledger says " + ledgerText(ledgerPnL(p), cur)
		if p.Commissions != "" {
			card.Summary += ", commission " + money(p.Commissions, cur)
		}
		card.DaysLine = mkLine(prefix+"-days", lblDaysZh, notComputed, "")
		card.Gap = &gap{Kind: "gap", Reason: classReasons[o.class], Text: fit(o.reason)}
		card.Summary = fit(card.Summary)
		return card, nil
	}
	res := o.res
	assigned := p.Status == "assigned" || p.Status == "exercised"
	card.Badges = append(badgesOf(res), card.Badges...)
	card.DaysLine = daysLine(prefix, res)
	switch {
	case res.NoQuotes:
		card.Lines = []line{
			mkLine(prefix+"-ledger", lblLedgerZh, money(res.LedgerPnL, cur), "ledger"),
			mkLine(prefix+"-comm", lblCommZh, money(res.Commission, cur), ""),
			mkLine(prefix+"-reason", lblReasonZh, noQuotesZh, ""),
		}
		return card, nil
	case res.Difference == "":
		f := figuresOfResult(res)
		card.Lines = append(splitLines(prefix, f, cur, assigned), ledgerLine(prefix, f, cur, assigned, false))
		card.Recon = []line{mkLine(prefix+"-diff", lblOursZh, reasonText(res), "difference").tone("muted")}
	default:
		books := c.Booker.Combine([]optionattrapi.Result{res})
		if len(books) == 0 {
			return card, errors.New("option_pnl_page: booker returned no book for one result")
		}
		f := figuresOfBook(books[0])
		card.Lines = append(splitLines(prefix, f, cur, assigned), ledgerLine(prefix, f, cur, assigned, false))
		card.Recon = []line{mkLine(prefix+"-diff", lblOursZh, "difference "+money(f.Difference, cur), "difference")}
	}
	if res.LateEntry {
		withEntryHint(card.Lines, prefix+"-entry", lateEntryLotZh)
	}
	if p.Status == statusOpen {
		card.Recon = append(card.Recon, mkLine(prefix+"-circ", lblOpenCheckZh, circularNote, "").tone("muted"))
	}
	if r := brokerLine(prefix, p.RealizedPnL, p.BrokerRealized, cur, p.Status); r != nil {
		card.Recon = append(card.Recon, *r)
	}
	if iv := ivLine(prefix, res); iv != nil {
		card.IVLine = iv
	}
	if assigned {
		l := mkLine(prefix+"-end", lblAssignZh, "shares bought at "+p.Strike+"; the close that day was "+o.lot.endClose+
			", so intrinsic value "+money(res.AssignedIntrinsic, cur)+" moved to the shares", "assigned")
		card.EndLine = &l
	}
	return card, nil
}

func reasonText(r optionattrapi.Result) string {
	if r.Reason == "" {
		return "not reconciled"
	}
	return r.Reason
}

func ledgerText(text, cur string) string {
	if text == "" {
		return ledgerMissing
	}
	return money(text, cur)
}

func lotTitle(o outcome) string {
	p := o.lot.pos
	return p.Underlying + " put " + p.Strike + " exp " + p.Expiry
}

func lotSummary(o outcome) string {
	p := o.lot.pos
	s := "short " + strings.TrimPrefix(p.Quantity, "-") + " opened " + string(p.OpenedOn) + " at " + money(p.OpenPrice, p.Currency) + "   "
	if p.Status != statusOpen {
		return s + p.Status + " " + string(p.ClosedOn)
	}
	day := p.MarkDay
	if o.res.MarkDay != "" {
		day = o.res.MarkDay
	}
	if o.isGap() || day == "" {
		return s + statusOpen
	}
	return s + "open (" + markedAtMid + string(day) + ")"
}

// brokerLine shows the ledger realised and the broker realised side by side; "differs" only when
// the two stored texts differ. An open lot has no realised figure.
func brokerLine(prefix, ledger, broker, cur, status string) *line {
	if status == statusOpen {
		return nil
	}
	l := mkLine(prefix+"-broker", lblBrokerZh, "broker realised not stored", "").tone("muted")
	if broker != "" {
		l.Text = "Ledger " + money(ledger, cur) + ", broker " + money(broker, cur)
		if ledger != broker {
			l.Text += ", " + brokerDiffers
		}
	}
	return &l
}

// ivLine is our IV of the first and last quote day and the stored IBKR IV of the last, comparison only.
func ivLine(prefix string, r optionattrapi.Result) *line {
	if len(r.Steps) == 0 {
		return nil
	}
	first, last := r.Steps[0], r.Steps[len(r.Steps)-1]
	to := last.IVTo
	if to == "" {
		to = last.IVFrom
	}
	text := "IV from mid " + percent(first.IVFrom) + " to " + percent(to) + " (ours, solved from the stored mid)   stored IBKR IV " +
		percent(last.RefIVTo) + " (comparison only)"
	l := mkLine(prefix+"-ivline", lblIVZh, text, "").tone("muted")
	return &l
}

func plural(n int, one, many string) string {
	if n == 1 {
		return strconv.Itoa(n) + " " + one
	}
	return strconv.Itoa(n) + " " + many
}

// daysLine counts the quote days and names the gap days and the days with no rate, from the steps.
func daysLine(prefix string, r optionattrapi.Result) line {
	if r.NoQuotes {
		return mkLine(prefix+"-days", lblDaysZh, noQuoteDays, "").tone("muted")
	}
	seen := map[string]bool{}
	multi := 0
	for _, s := range r.Steps {
		seen[string(s.From)] = true
		if !s.Terminal {
			seen[string(s.To)] = true
		}
		if s.MultiDay {
			multi++
		}
	}
	text := strconv.Itoa(len(seen)) + " with a quote"
	if n := len(r.GapDays); n > 0 {
		days := make([]string, 0, n)
		for _, d := range r.GapDays {
			days = append(days, string(d))
		}
		text += ", " + plural(n, "gap day", "gap days") + " " + strings.Join(days, ", ") + " (no two-sided quote stored"
		if multi > 0 {
			text += "; " + plural(multi, "two-day step", "two-day steps")
		}
		text += ")"
	}
	if n := len(r.NoRate); n > 0 {
		days := make([]string, 0, n)
		for _, d := range r.NoRate {
			days = append(days, string(d))
		}
		text += "; no rate stored for " + strings.Join(days, ", ")
	}
	return mkLine(prefix+"-days", lblDaysZh, fit(text), "").tone("muted")
}

// badgesOf are the badges of a result, from its fields only.
func badgesOf(r optionattrapi.Result) []string {
	out := []string{}
	if r.NoQuotes {
		return append(out, noQuotesBadge)
	}
	multi, wide := false, false
	for _, s := range r.Steps {
		multi = multi || s.MultiDay
		wide = wide || s.Wide
	}
	if multi {
		out = append(out, badgeTwoDay)
	}
	if wide {
		out = append(out, badgeWide)
	}
	if r.LateEntry {
		out = append(out, badgeLate)
	}
	if r.UnsplitSteps > 0 {
		out = append(out, badgeUnsplit+plural(r.UnsplitSteps, "step", "steps"))
	}
	return out
}

var statusOrder = []string{"open", "closed", "expired", "assigned", "exercised"}
var classOrder = []string{classNoMark, classNoClose, classNoLedger, classRefused}

// bookCards is one book per currency in the order the lots appear: the Booker's book when it has one,
// otherwise a placeholder that says nothing could be split.
func bookCards(alias string, outs []outcome, books []optionattrapi.Book) []bookView {
	out := []bookView{}
	var order []string
	byCur := map[string]optionattrapi.Book{}
	for _, b := range books {
		if _, ok := byCur[b.Currency]; !ok {
			order = append(order, b.Currency)
		}
		byCur[b.Currency] = b
	}
	for _, o := range outs {
		cur := o.lot.pos.Currency
		if _, ok := byCur[cur]; !ok {
			byCur[cur] = optionattrapi.Book{Currency: cur}
			order = append(order, cur)
		}
	}
	for i, cur := range order {
		b := byCur[cur]
		v := bookView{ID: "book-" + safeID(alias+"-"+cur, 90), Currency: cur, Lines: []line{}}
		prefix := "b" + strconv.Itoa(i) + "-" + safeID(alias, 16)
		v.Counts = cur + " book, " + plural(b.Lots, "lot", "lots") + " counted" + statusCounts(outs, cur)
		v.LeftOut = leftOut(outs, b, cur)
		if b.Lots == 0 && b.Total == "" {
			v.Headline = mkLine(prefix+"-head", lblTotalZh, noBookLine, "").tone("muted")
			out = append(out, v)
			continue
		}
		f := figuresOfBook(b)
		v.Headline = mkLine(prefix+"-head", lblRentZh, money(f.RentMinusCost, cur), "rent").tone(toneOf(f.RentMinusCost))
		v.Lines = append(splitLines(prefix, f, cur, true), ledgerLine(prefix, f, cur, true, true))
		withEntryHint(v.Lines, prefix+"-entry", lateEntryHint(curOutcomes(outs, cur), b.Lots))
		out = append(out, v)
	}
	return out
}

// statusCounts is " (2 open, 1 assigned)" over the lots of the currency that were attributed and have
// a split and a difference (the lots the Booker adds); empty when there are none.
func statusCounts(outs []outcome, cur string) string {
	n := map[string]int{}
	for _, o := range outs {
		if o.isGap() || o.res.NoQuotes || o.res.Difference == "" || o.lot.pos.Currency != cur {
			continue
		}
		n[o.lot.pos.Status]++
	}
	var parts []string
	for _, s := range statusOrder {
		if n[s] > 0 {
			parts = append(parts, strconv.Itoa(n[s])+" "+s)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

// leftOut is the line naming every class left out of the sums of a currency with its lot count and
// its ledger P&L; nil when none was left out.
func leftOut(outs []outcome, b optionattrapi.Book, cur string) *string {
	var parts []string
	gapLots := 0
	for _, class := range classOrder {
		var names []string
		for _, o := range outs {
			if o.isGap() && o.class == class && o.lot.pos.Currency == cur {
				names = append(names, o.lot.pos.Underlying+ledgerSepZh+ledgerText(ledgerPnL(o.lot.pos), cur))
			}
		}
		if len(names) > 0 {
			gapLots += len(names)
			parts = append(parts, strconv.Itoa(len(names))+countUnitZh+classTexts[class]+lotsOpenZh+strings.Join(names, semiZh)+closeZh)
		}
	}
	if b.NoQuoteLots > 0 {
		parts = append(parts, strconv.Itoa(b.NoQuoteLots)+noQuoteLotsZh+ledgerText(b.NoQuoteLedgerPnL, cur)+semiZh+leftOutNoQuotesZh+closeZh)
	}
	if b.UnreconciledLots > 0 {
		parts = append(parts, strconv.Itoa(b.UnreconciledLots)+unreconLotsZh+ledgerText(b.UnreconciledLedgerPnL, cur)+closeZh)
	}
	if len(parts) == 0 {
		return nil
	}
	n := gapLots + b.NoQuoteLots + b.UnreconciledLots
	s := fit(leftOutOfZh + strconv.Itoa(b.Lots+n) + leftOutLotsZh + strconv.Itoa(n) + leftOutHead + strings.Join(parts, semiZh))
	return &s
}

// curOutcomes are the outcomes of one currency.
func curOutcomes(outs []outcome, cur string) []outcome {
	var r []outcome
	for _, o := range outs {
		if o.lot.pos.Currency == cur {
			r = append(r, o)
		}
	}
	return r
}
