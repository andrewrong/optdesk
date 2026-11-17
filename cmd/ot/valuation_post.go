package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute/valuelines"
	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// Design: docs/design/valuation-structure.md#7 valuation-wire-posts (Decisions 5, 6, 9, 11).

// valuationStore is the reader and writer of one post, opened with the OT_PG_DSN_SETTINGS line.
type valuationStore struct {
	r     valuation.Reader
	w     valuation.Writer
	close func()
}

// openValuationStore opens the store of a post. An error carries no value of the form or of the line.
func openValuationStore(a *app) (valuationStore, error) {
	_, dsn, reason, err := a.ownerEditSecrets()
	if err != nil || reason != "" || a.deps.openValuation == nil {
		return valuationStore{}, errors.New("valuation: no writer")
	}
	r, w, c, err := a.deps.openValuation(dsn)
	if err != nil {
		return valuationStore{}, errors.New("valuation: cannot open the store")
	}
	return valuationStore{r: r, w: w, close: func() { _ = c.Close() }}, nil
}

// valuationPostStart is the common start of both posts: the method, the guard of the owner posts and the store.
// ok false means the answer is written and nothing was written to the store.
func valuationPostStart(a *app, rd siteReaders, w http.ResponseWriter, r *http.Request) (st valuationStore, ok bool) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return st, false
	}
	if !guardOwnerPost(w, r, rd.edit) {
		return st, false
	}
	st, err := openValuationStore(a)
	if err != nil {
		valuationFail(a, w, "valuation post: no store")
		return st, false
	}
	return st, true
}

func valuationFail(a *app, w http.ResponseWriter, msg string) {
	a.log.Error(msg, "component", "ot", "status", "failed")
	http.Error(w, "internal error", http.StatusInternalServerError)
}

// valuationSeeOther answers 303 to path with the query q.
func valuationSeeOther(w http.ResponseWriter, r *http.Request, path string, q url.Values) {
	http.Redirect(w, r, "/"+path+"?"+q.Encode(), http.StatusSeeOther)
}

// postOwnerValuationParams serves POST /valuation-params-post: the guard, then one of the actions r_table,
// margin_table, accept_r and r_version, each one insert. A refusal re-renders the page with 422 and the text in
// error; a write answers 303 to the page with saved=1 (warned=1 when the saved R moves a market by more than
// RStepWarn). Nothing is written before the guard passes.
func postOwnerValuationParams(a *app, clk clock.Clock, rd siteReaders, newHandler pages.NewHandlerFunc, w http.ResponseWriter, r *http.Request) {
	st, ok := valuationPostStart(a, rd, w, r)
	if !ok {
		return
	}
	defer st.close()
	ctx := r.Context()
	refuse := func(text string) {
		vm, err := valuationParamsDoc(ctx, a, clk, rd, st.r, text)
		if err != nil {
			valuationFail(a, w, "valuation params build failed")
			return
		}
		newHandler(vm).ServeHTTP(&statusWriter{ResponseWriter: w, code: http.StatusUnprocessableEntity}, r)
	}
	state, err := st.r.Current(ctx)
	if err != nil {
		valuationFail(a, w, "valuation params post: read failed")
		return
	}
	v := r.PostForm
	date, text := postedDay(v.Get("effective_date"), "effective_date")
	if text != "" {
		refuse(text)
		return
	}
	q, text, err := paramsAction(ctx, st.w, v, date, state)
	if text != "" {
		refuse(text)
		return
	}
	if err != nil {
		valuationFail(a, w, "valuation params write failed")
		return
	}
	valuationSeeOther(w, r, valuationParamsRoute, q)
}

// paramsAction runs one action of the params form: the refusal text (nothing written) or the error of the one insert,
// and the query of the 303 answer.
func paramsAction(ctx context.Context, w valuation.Writer, v url.Values, date contracts.Day, state valuation.State) (url.Values, string, error) {
	q := url.Values{"saved": {"1"}}
	switch v.Get("action") {
	case "r_table":
		rows, text := parseRTable(v.Get("table"))
		if text != "" {
			return q, text, nil
		}
		return q, "", w.PutRTable(ctx, valuation.RTableVersion{EffectiveDate: date, Rows: rows})
	case "margin_table":
		rows, text := parseMarginTable(v.Get("table"))
		if text != "" {
			return q, text, nil
		}
		return q, "", w.PutMarginTable(ctx, valuation.MarginTableVersion{EffectiveDate: date, Rows: rows})
	case "accept_r":
		ver, warned, text := acceptR(v, date, state)
		if text != "" {
			return q, text, nil
		}
		if warned {
			q.Set("warned", "1")
		}
		return q, "", w.PutRVersion(ctx, ver)
	case "r_version":
		ver, text := typedRVersion(v, date)
		if text != "" {
			return q, text, nil
		}
		return q, "", w.PutRVersion(ctx, ver)
	}
	return q, "action: unknown", nil
}

// postedDay reads a required day field.
func postedDay(s, field string) (contracts.Day, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", field + ": required"
	}
	if _, err := time.Parse(dayLayout, s); err != nil {
		return "", field + ": not a date, write YYYY-MM-DD"
	}
	return contracts.Day(s), ""
}

// tableLines are the non-empty lines of a posted table, each as its fields (commas and blanks separate them), with
// the number of the line in the text.
type tableLine struct {
	n      int
	fields []string
}

func tableLines(text string) []tableLine {
	var out []tableLine
	for i, l := range strings.Split(text, "\n") {
		f := strings.FieldsFunc(l, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\r' })
		if len(f) > 0 {
			out = append(out, tableLine{n: i + 1, fields: f})
		}
	}
	return out
}

// decimalAt checks that s is a decimal text; the text is stored as typed.
func decimalAt(s string) (decimal.Decimal, bool) {
	d, err := decimal.NewFromString(s)
	return d, err == nil
}

// parseRTable reads the R table: one bucket per line, `yield_up_to us jp hkcn`; the top bucket, which must be the
// last, has no yield_up_to and gives three fields. The yield_up_to of the lines must ascend.
func parseRTable(text string) ([]valuation.RTableRow, string) {
	lines := tableLines(text)
	if len(lines) == 0 {
		return nil, "table: required"
	}
	var rows []valuation.RTableRow
	var prev decimal.Decimal
	for i, l := range lines {
		var row valuation.RTableRow
		switch len(l.fields) {
		case 4:
			row = valuation.RTableRow{YieldUpTo: l.fields[0], US: l.fields[1], JP: l.fields[2], HKCN: l.fields[3]}
		case 3:
			if i != len(lines)-1 {
				return nil, fmt.Sprintf("line %d: a bucket with no yield_up_to must be the last", l.n)
			}
			row = valuation.RTableRow{US: l.fields[0], JP: l.fields[1], HKCN: l.fields[2]}
		default:
			return nil, fmt.Sprintf("line %d: want yield_up_to, us, jp, hkcn", l.n)
		}
		for _, f := range []string{row.YieldUpTo, row.US, row.JP, row.HKCN} {
			if _, ok := decimalAt(f); !ok && f != "" {
				return nil, fmt.Sprintf("line %d: %q is not a number", l.n, f)
			}
		}
		if row.YieldUpTo != "" {
			y, _ := decimalAt(row.YieldUpTo)
			if i > 0 && !y.GreaterThan(prev) {
				return nil, fmt.Sprintf("line %d: yield_up_to must be above that of the line before", l.n)
			}
			prev = y
		}
		rows = append(rows, row)
	}
	return rows, ""
}

// parseMarginTable reads the margin table: one bucket per line, `p_from m w h s`, p_from ascending.
func parseMarginTable(text string) ([]valuation.MarginTableRow, string) {
	lines := tableLines(text)
	if len(lines) == 0 {
		return nil, "table: required"
	}
	var rows []valuation.MarginTableRow
	var prev decimal.Decimal
	for i, l := range lines {
		if len(l.fields) != 5 {
			return nil, fmt.Sprintf("line %d: want p_from, m, w, h, s", l.n)
		}
		for _, f := range l.fields {
			if _, ok := decimalAt(f); !ok {
				return nil, fmt.Sprintf("line %d: %q is not a number", l.n, f)
			}
		}
		p, _ := decimalAt(l.fields[0])
		if i > 0 && !p.GreaterThan(prev) {
			return nil, fmt.Sprintf("line %d: p_from must be above that of the line before", l.n)
		}
		prev = p
		rows = append(rows, valuation.MarginTableRow{PFrom: l.fields[0], M: l.fields[1], W: l.fields[2], H: l.fields[3], S: l.fields[4]})
	}
	return rows, ""
}

// acceptR makes the R version of the click on "Update R": the proposal is recomputed here with ProposeR from the
// posted yield and the stored R table, and the posted R values are never read. The AI switch of the newest version
// is carried over. warned says that a market moves by more than RStepWarn; the row is saved either way.
func acceptR(v url.Values, date contracts.Day, st valuation.State) (valuation.RVersion, bool, string) {
	yield := strings.TrimSpace(v.Get("yield"))
	if _, ok := decimalAt(yield); !ok {
		return valuation.RVersion{}, false, "yield: required, a number"
	}
	if st.RTable == nil {
		return valuation.RVersion{}, false, "r_table: no R table is stored; enter one first"
	}
	rows := make([]valuelinesapi.RRow, 0, len(st.RTable.Rows))
	for _, r := range st.RTable.Rows {
		rows = append(rows, valuelinesapi.RRow{YieldUpTo: r.YieldUpTo, US: r.US, JP: r.JP, HKCN: r.HKCN})
	}
	calc := valuelines.New()
	next, err := calc.ProposeR(rows, yield)
	if err != nil {
		return valuation.RVersion{}, false, "yield: no bucket of the R table fits it"
	}
	ver := valuation.RVersion{EffectiveDate: date, YieldUsed: yield, RUS: next.US, RJP: next.JP, RHKCN: next.HKCN}
	warned := false
	if p := st.RVersion; p != nil {
		ver.AISwitch, ver.AIExpiry = p.AISwitch, p.AIExpiry
		warned = len(calc.RStepWarnings(valuelinesapi.RValues{US: p.RUS, JP: p.RJP, HKCN: p.RHKCN}, next)) > 0
	}
	return ver, warned, ""
}

// typedRVersion reads an R version typed on the form: the three R, the AI switch and its expiry text, required
// when the switch is on (Decision 11).
func typedRVersion(v url.Values, date contracts.Day) (valuation.RVersion, string) {
	t := func(k string) string { return strings.TrimSpace(v.Get(k)) }
	ver := valuation.RVersion{
		EffectiveDate: date, YieldUsed: t("yield_used"), RUS: t("r_us"), RJP: t("r_jp"), RHKCN: t("r_hkcn"),
		AISwitch: switchOn(v, "ai_switch"), AIExpiry: t("ai_expiry"), Note: t("note"),
	}
	for _, f := range []struct{ name, val string }{{"r_us", ver.RUS}, {"r_jp", ver.RJP}, {"r_hkcn", ver.RHKCN}} {
		if _, ok := decimalAt(f.val); !ok {
			return ver, f.name + ": required, a number"
		}
	}
	if _, ok := decimalAt(ver.YieldUsed); !ok && ver.YieldUsed != "" {
		return ver, "yield_used: not a number"
	}
	if ver.AISwitch && ver.AIExpiry == "" {
		return ver, "ai_expiry: required when the AI switch is on"
	}
	return ver, ""
}
