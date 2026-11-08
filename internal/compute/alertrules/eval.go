package alertrules

import (
	"context"
	"fmt"
	"math/big"
	"slices"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	dayLayout  = "2006-01-02"
	lookBack   = 5
	dataDayKey = "data_day"
)

// opWords are the fixed words of Detail, one per comparison.
var opWords = map[string]string{"gt": "above", "ge": "at or above", "lt": "below", "le": "at or below"}

// Evaluate evaluates every rule over the readings read returns for today-5 to today and
// uses the newest row. It follows the order of the rules. A reader error is an error of the
// call, never a no_data result.
func Evaluate(ctx context.Context, rules compute.AlertRules, read compute.ReadingFunc, today contracts.Day) ([]compute.AlertResult, error) {
	to, err := parseDay(string(today))
	if err != nil {
		return nil, fmt.Errorf("alertrules: today is not a day")
	}
	from := contracts.Day(to.AddDate(0, 0, -lookBack).Format(dayLayout))
	out := make([]compute.AlertResult, 0, len(rules.Rules))
	for _, r := range rules.Rules {
		rows, err := read(ctx, r.Figure, r.SubjectKind, r.Subject, from, today)
		if err != nil {
			return nil, fmt.Errorf("alertrules: reading for rule %q: %w", r.ID, err)
		}
		res, err := evalRule(r, rows, from, today)
		if err != nil {
			return nil, err
		}
		res.Public = slices.Contains(rules.PublicSubjects, r.Subject)
		out = append(out, res)
	}
	return out, nil
}

func parseDay(s string) (time.Time, error) {
	t, err := time.Parse(dayLayout, s)
	if err != nil || t.Format(dayLayout) != s {
		return time.Time{}, fmt.Errorf("not a day")
	}
	return t, nil
}

func rowDay(r contracts.ReadingRecord) contracts.Day {
	return contracts.Day(r.Day.Format(dayLayout))
}

// newest picks the row of the latest day inside the window, the highest rev on a tie.
func newest(rows []contracts.ReadingRecord, from, to contracts.Day) (contracts.ReadingRecord, bool) {
	var best contracts.ReadingRecord
	found := false
	for _, r := range rows {
		d := rowDay(r)
		if d < from || d > to {
			continue
		}
		if !found || d > rowDay(best) || (d == rowDay(best) && r.Rev > best.Rev) {
			best, found = r, true
		}
	}
	return best, found
}

func evalRule(r compute.AlertRule, rows []contracts.ReadingRecord, from, to contracts.Day) (compute.AlertResult, error) {
	res := compute.AlertResult{RuleID: r.ID, Title: r.Title, Level: r.Level, Figure: r.Figure, State: "no_data"}
	row, ok := newest(rows, from, to)
	if !ok {
		res.Reason = "no_reading"
		return res, nil
	}
	res.Day = rowDay(row)
	if row.State != "ok" || row.Value == nil || !decimalRe.MatchString(*row.Value) {
		res.Reason = "missing_input"
		return res, nil
	}
	if string(row.Unit) != r.Unit {
		res.Reason = "unit_mismatch"
		return res, nil
	}
	dd, ok := dataDay(row)
	if !ok {
		res.Reason = "no_data_day"
		return res, nil
	}
	res.DataDay = dd
	rd, _ := parseDay(string(res.Day))
	dt, _ := parseDay(string(dd))
	if rd.Sub(dt) > time.Duration(r.MaxDataAgeDays)*24*time.Hour {
		res.Reason = "stale_data"
		return res, nil
	}
	val, vok := new(big.Rat).SetString(*row.Value)
	thr, tok := new(big.Rat).SetString(r.Threshold)
	if !vok || !tok || !decimalRe.MatchString(r.Threshold) {
		return res, fmt.Errorf("alertrules: rule %q: key %q is not a decimal", r.ID, "threshold")
	}
	words, wok := opWords[r.Op]
	if !wok {
		return res, fmt.Errorf("alertrules: rule %q: key %q is not one of gt, ge, lt, le", r.ID, "op")
	}
	c := val.Cmp(thr)
	hold := false
	switch r.Op {
	case "gt":
		hold = c > 0
	case "ge":
		hold = c >= 0
	case "lt":
		hold = c < 0
	case "le":
		hold = c <= 0
	}
	res.State = "clear"
	if hold {
		res.State = "fired"
	}
	res.Value = *row.Value
	res.Detail = fmt.Sprintf("%s %s %s %s %s", r.Figure, r.Subject, *row.Value, words, r.Threshold)
	return res, nil
}

func dataDay(row contracts.ReadingRecord) (contracts.Day, bool) {
	for _, p := range row.Parameters {
		if p.Param != dataDayKey || p.ValueTag == nil {
			continue
		}
		if _, err := parseDay(*p.ValueTag); err != nil {
			return "", false
		}
		return contracts.Day(*p.ValueTag), true
	}
	return "", false
}
