package optionattr

import (
	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
)

var _ optionattrapi.Booker = booker{}

// NewBooker returns the Booker: the only place the results of lots are added.
func NewBooker() optionattrapi.Booker { return booker{} }

type booker struct{}

// sums holds the running decimal sums of one currency.
type sums struct {
	book                                         optionattrapi.Book
	entry, exit, commission                      decimal.Decimal
	direction, moveCost, ivRemark, time, unsplit decimal.Decimal
	assigned, ledger, diff                       decimal.Decimal
	noQuoteLedger, unreconciledLedger            decimal.Decimal
}

// amount reads a decimal text; an empty text (an absent Exit) and a text that is no number count as zero.
func amount(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero
	}
	return d
}

// Combine groups the results by currency in order of first appearance and adds them exactly.
func (booker) Combine(rs []optionattrapi.Result) []optionattrapi.Book {
	var order []*sums
	byCurrency := map[string]*sums{}
	for _, r := range rs {
		s, ok := byCurrency[r.Currency]
		if !ok {
			s = &sums{}
			s.book.Currency = r.Currency
			byCurrency[r.Currency] = s
			order = append(order, s)
		}
		switch {
		case r.NoQuotes:
			s.book.NoQuoteLots++
			s.noQuoteLedger = s.noQuoteLedger.Add(amount(r.LedgerPnL))
		case r.Difference == "":
			s.book.UnreconciledLots++
			s.unreconciledLedger = s.unreconciledLedger.Add(amount(r.LedgerPnL))
		default:
			s.book.Lots++
			s.entry = s.entry.Add(amount(r.Entry))
			s.exit = s.exit.Add(amount(r.Exit))
			s.commission = s.commission.Add(amount(r.Commission))
			s.direction = s.direction.Add(amount(r.Direction))
			s.moveCost = s.moveCost.Add(amount(r.MoveCost))
			s.ivRemark = s.ivRemark.Add(amount(r.IVRemark))
			s.time = s.time.Add(amount(r.Time))
			s.unsplit = s.unsplit.Add(amount(r.Unsplit))
			s.assigned = s.assigned.Add(amount(r.AssignedIntrinsic))
			s.ledger = s.ledger.Add(amount(r.LedgerPnL))
			s.diff = s.diff.Add(amount(r.Difference))
		}
	}
	out := make([]optionattrapi.Book, 0, len(order))
	for _, s := range order {
		b := s.book
		b.Entry, b.Exit, b.Commission = s.entry.String(), s.exit.String(), s.commission.String()
		b.Direction, b.MoveCost, b.IVRemark = s.direction.String(), s.moveCost.String(), s.ivRemark.String()
		b.Time, b.Unsplit = s.time.String(), s.unsplit.String()
		b.RentMinusCost = s.time.Add(s.moveCost).String()
		b.Total = s.entry.Add(s.exit).Add(s.direction).Add(s.moveCost).Add(s.ivRemark).
			Add(s.time).Add(s.unsplit).Add(s.commission).String()
		b.AssignedIntrinsic, b.LedgerPnL, b.Difference = s.assigned.String(), s.ledger.String(), s.diff.String()
		b.NoQuoteLedgerPnL, b.UnreconciledLedgerPnL = s.noQuoteLedger.String(), s.unreconciledLedger.String()
		b.AllLots = b.Lots + b.NoQuoteLots + b.UnreconciledLots
		b.LedgerAllPnL = s.ledger.Add(s.noQuoteLedger).Add(s.unreconciledLedger).String()
		out = append(out, b)
	}
	return out
}
