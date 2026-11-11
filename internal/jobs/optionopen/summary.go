package optionopen

import (
	"fmt"
	"strings"
	"time"
)

// StockProgress is the progress of one stock in the run summary.
type StockProgress struct {
	Symbol   string
	Done     int
	Total    int
	LastRowT time.Time
}

// Summary is the run summary of Decision 8, derived from the rows.
type Summary struct {
	Items, OK, Partial int
	Seconds            float64
	Stocks             []StockProgress
}

// String is the one line of the summary.
func (s Summary) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "option_open: items=%d ok=%d partial=%d seconds=%.1f", s.Items, s.OK, s.Partial, s.Seconds)
	for _, p := range s.Stocks {
		fmt.Fprintf(&b, " %s=%d/%d@%s", p.Symbol, p.Done, p.Total, p.LastRowT.UTC().Format("15:04:05"))
	}
	return b.String()
}

// summary derives the run summary from the rows: the seconds run from the first requested_at to the last
// received_at of the rows that were asked.
func (c *Core) summary(st *runState) Summary {
	s := Summary{Items: len(st.rows)}
	var first, last time.Time
	for _, r := range st.rows {
		switch r.Status {
		case stOK:
			s.OK++
		case stPartial:
			s.Partial++
		}
		if r.SentAt == nil {
			continue
		}
		if first.IsZero() || r.RequestedAt.Before(first) {
			first = r.RequestedAt
		}
		if r.ReceivedAt.After(last) {
			last = r.ReceivedAt
		}
	}
	if !first.IsZero() && last.After(first) {
		s.Seconds = last.Sub(first).Seconds()
	}
	for _, u := range st.order {
		s.Stocks = append(s.Stocks, *st.progress[u])
	}
	return s
}
