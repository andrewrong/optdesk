package optioneod

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/runview"
)

// Codes and scopes of the reason rows (docs/design/option-field-reasons.md, Decisions 2 to 5).
const (
	codeFieldMissing   = "field_missing"
	codeFieldsComplete = "fields_complete"
	scopeQuotes        = "quotes"
	scopeFields        = "fields"
	causeUnreported    = "unreported"
)

// tallied is one counted field: the Quote field name that Quote.Missing uses, the pointer of the quote and the column name.
type tallied struct {
	field  string
	column string
	value  func(q broker.Quote) *float64
}

var talliedFields = []tallied{
	{"bid", "bid", func(q broker.Quote) *float64 { return q.Bid }},
	{"ask", "ask", func(q broker.Quote) *float64 { return q.Ask }},
	{"last", "last", func(q broker.Quote) *float64 { return q.Last }},
	{"volume", "volume", func(q broker.Quote) *float64 { return q.Volume }},
	{"open_interest", "open_interest", func(q broker.Quote) *float64 { return q.OpenInterest }},
	{"implied_vol", "iv", func(q broker.Quote) *float64 { return q.ImpliedVol }},
	{"delta", "delta", func(q broker.Quote) *float64 { return q.Delta }},
	{"gamma", "gamma", func(q broker.Quote) *float64 { return q.Gamma }},
	{"theta", "theta", func(q broker.Quote) *float64 { return q.Theta }},
	{"vega", "vega", func(q broker.Quote) *float64 { return q.Vega }},
}

// tally counts, for one key, the stored quotes: the nil fields of the usable ones by cause and the others once.
type tally struct {
	total   int                       // every quote added
	usable  int                       // row status 1
	fields  map[string]map[string]int // column name -> cause -> count
	noQuote map[string]int            // cause -> count of quotes of row status 2 or 5
}

// add counts q, whose stored row has status rowStatus. A field cause is the entry of q.Missing, else unreported;
// the status of the quote is read only to name why a quote without content has none.
func (t *tally) add(q broker.Quote, rowStatus int) {
	t.total++
	if rowStatus != statusOK {
		if t.noQuote == nil {
			t.noQuote = map[string]int{}
		}
		t.noQuote[quoteCause(q)]++
		return
	}
	t.usable++
	for _, f := range talliedFields {
		if f.value(q) != nil {
			continue
		}
		if t.fields == nil {
			t.fields = map[string]map[string]int{}
		}
		if t.fields[f.column] == nil {
			t.fields[f.column] = map[string]int{}
		}
		t.fields[f.column][fieldCause(q, f.field)]++
	}
}

func fieldCause(q broker.Quote, field string) string {
	for _, m := range q.Missing {
		if m.Field != field {
			continue
		}
		if m.Reason == broker.MissingError {
			return "error_" + strconv.Itoa(m.ErrorCode)
		}
		return m.Reason.String()
	}
	return causeUnreported
}

// quoteCause is why a quote of row status 2 or 5 has no content.
func quoteCause(q broker.Quote) string {
	switch q.Status {
	case broker.StatusNoData:
		return "no_data"
	case broker.StatusNoSubscription:
		return "no_subscription"
	case broker.StatusTimeout:
		return "timeout"
	case broker.StatusDisconnected:
		return "disconnected"
	case broker.StatusError:
		if q.ErrorCode != 0 {
			return "error_" + strconv.Itoa(q.ErrorCode)
		}
		return "error"
	}
	return "empty"
}

// causes renders "cause n, cause n" by count descending, then name.
func causes(m map[string]int) string {
	names := make([]string, 0, len(m))
	for c := range m {
		names = append(names, c)
	}
	sort.Slice(names, func(i, j int) bool {
		if m[names[i]] != m[names[j]] {
			return m[names[i]] > m[names[j]]
		}
		return names[i] < names[j]
	})
	parts := make([]string, len(names))
	for i, c := range names {
		parts[i] = fmt.Sprintf("%s %d", c, m[c])
	}
	return strings.Join(parts, ", ")
}

func sum(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

// reasons returns the rows of the key: one field_missing row per field with a missing quote, one for quotes without
// content, and one fields_complete row when a usable quote was stored and no field row exists.
func (t *tally) reasons(day contracts.Day, dataset, subject string, at time.Time) []runview.Reason {
	rows := []runview.Reason{}
	row := func(scope, code, detail string) {
		rows = append(rows, runview.Reason{Day: day, Dataset: dataset, Subject: subject, Scope: scope, Code: code, Detail: detail, At: at})
	}
	for _, f := range talliedFields {
		if c := t.fields[f.column]; len(c) > 0 {
			row("field:"+f.column, codeFieldMissing,
				fmt.Sprintf("%s missing %d/%d usable quotes: %s", f.column, sum(c), t.usable, causes(c)))
		}
	}
	if n := sum(t.noQuote); n > 0 {
		row(scopeQuotes, codeFieldMissing, fmt.Sprintf("no quote %d/%d quotes: %s", n, t.total, causes(t.noQuote)))
	}
	if t.usable > 0 && len(t.fields) == 0 {
		row(scopeFields, codeFieldsComplete, fmt.Sprintf("no missing field in %d usable quotes", t.usable))
	}
	return rows
}
