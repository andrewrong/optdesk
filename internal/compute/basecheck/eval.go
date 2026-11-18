package basecheck

import (
	"math/big"

	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// NotRead is the one check of a row whose view failed.
const NotRead = "not_read"

var _ basecheckapi.Func = Evaluate

// rank orders the levels for the worst: fail, unknown, warn, pass.
func rank(l basecheckapi.Level) int {
	switch l {
	case basecheckapi.Fail:
		return 3
	case basecheckapi.Unknown:
		return 2
	case basecheckapi.Warn:
		return 1
	case basecheckapi.Pass:
		return 0
	}
	return 0
}

func worse(a, b basecheckapi.Level) basecheckapi.Level {
	if rank(b) > rank(a) {
		return b
	}
	return a
}

type key struct{ dataset, rule string }

// Evaluate turns facts, ledger cells and sessions into the verdicts of the page, one row per
// entry of basecheckapi.Rows in order. It reads no clock and no store.
func Evaluate(in basecheckapi.Input) basecheckapi.Output {
	facts := map[key][]basecheckapi.Fact{}
	for _, f := range in.Facts {
		k := key{f.Dataset, f.Rule}
		facts[k] = append(facts[k], f)
	}
	cells := map[string][]basecheckapi.Cell{}
	for _, c := range in.Cells {
		cells[c.Dataset] = append(cells[c.Dataset], c)
	}
	notRead := map[string]bool{}
	for _, id := range in.NotRead {
		notRead[id] = true
	}

	out := basecheckapi.Output{
		Level:  basecheckapi.Pass,
		Counts: map[basecheckapi.Level]int{basecheckapi.Pass: 0, basecheckapi.Warn: 0, basecheckapi.Fail: 0, basecheckapi.Unknown: 0},
	}
	for _, info := range basecheckapi.Rows {
		var row basecheckapi.Row
		if notRead[info.ID] {
			row = basecheckapi.Row{
				Dataset: info.ID, Group: info.Group, Level: basecheckapi.Unknown,
				Checks: []basecheckapi.Check{{Rule: NotRead, Level: basecheckapi.Unknown}},
			}
		} else {
			row = evalRow(in, info, facts, cells[info.ID])
		}
		out.Rows = append(out.Rows, row)
		out.Counts[row.Level]++
		out.Level = worse(out.Level, row.Level)
	}
	return out
}

func evalRow(in basecheckapi.Input, info basecheckapi.RowInfo, facts map[key][]basecheckapi.Fact, cells []basecheckapi.Cell) basecheckapi.Row {
	row := basecheckapi.Row{Dataset: info.ID, Group: info.Group, Level: basecheckapi.Pass}
	newest := newestFacts(facts[key{info.ID, "newest"}])
	if len(newest) > 0 {
		top := newest[0]
		for _, f := range newest {
			if f.Day > top.Day {
				top = f
			}
		}
		row.Newest, row.NewestValue, row.NewestSubject = string(top.Day), top.Value, top.Subject
	}
	for _, r := range basecheckapi.Rules {
		if r.Row != info.ID || r.Rule == "newest" {
			continue
		}
		th := Thresholds[r.Row+"."+r.Rule]
		var c basecheckapi.Check
		switch r.Source {
		case "note":
			c = noteCheck(r.Rule, facts[key{info.ID, r.Rule}])
		case "ledger":
			c = ledgerCheck(in, info, r.Rule, th, cells)
		case "derived":
			c = ageCheck(in, info, r.Rule, th, newest)
			row.AgeDays, row.AgeSessions = ageNumbers(in, info, newest)
		default:
			fs := facts[key{info.ID, r.Rule}]
			if th.Fail.Kind == Empty {
				fs = declaredSubjects(fs, info.Calendars)
			}
			c = sqlCheck(r.Rule, th, fs)
		}
		row.Checks = append(row.Checks, c)
		if r.Source != "note" {
			row.Level = worse(row.Level, c.Level)
		}
	}
	return row
}

// declaredSubjects leaves out the facts of a subject the row does not declare (not a key of
// calendars), so a view that emits a bare subject the row never fetches does not fail the row. With
// no calendars, or for a fact with an empty subject, every fact is kept.
func declaredSubjects(fs []basecheckapi.Fact, calendars map[string]string) []basecheckapi.Fact {
	if len(calendars) == 0 {
		return fs
	}
	var out []basecheckapi.Fact
	for _, f := range fs {
		if _, ok := calendars[f.Subject]; ok || f.Subject == "" {
			out = append(out, f)
		}
	}
	return out
}

// newestFacts keeps the newest facts that carry a day.
func newestFacts(fs []basecheckapi.Fact) []basecheckapi.Fact {
	var out []basecheckapi.Fact
	for _, f := range fs {
		if f.Day != "" {
			out = append(out, f)
		}
	}
	return out
}

func baseCheck(rule string, th Threshold) basecheckapi.Check {
	return basecheckapi.Check{
		Rule: rule, Level: basecheckapi.Unknown,
		WarnAt: limitText(th.Warn), FailAt: limitText(th.Fail), ThresholdSource: th.Source,
	}
}

func limitText(l Limit) string {
	switch l.Kind {
	case None:
		return ""
	case Any:
		return "any"
	case Empty:
		return "empty"
	case Count, Share, Days, Sessions:
		return l.Value
	}
	return l.Value
}

// reached says whether a limit is met by n rows examined, bad of them bad, and an age.
func reached(l Limit, n, bad int64, age int) bool {
	switch l.Kind {
	case Any:
		return bad >= 1
	case Empty:
		return n == 0
	case Count:
		v, ok := new(big.Rat).SetString(l.Value)
		return ok && new(big.Rat).SetInt64(bad).Cmp(v) >= 0
	case Share:
		v, ok := new(big.Rat).SetString(l.Value)
		if !ok || n <= 0 {
			return false
		}
		return new(big.Rat).SetFrac64(bad, n).Cmp(v) >= 0
	case Days, Sessions:
		v, ok := new(big.Rat).SetString(l.Value)
		return ok && new(big.Rat).SetInt64(int64(age)).Cmp(v) >= 0
	case None:
		return false
	}
	return false
}

// level is fail when the fail limit is met, else warn when the warn limit is, else pass. A rule
// with no fail limit is at most warn.
func level(th Threshold, n, bad int64, age int) basecheckapi.Level {
	switch {
	case reached(th.Fail, n, bad, age):
		return basecheckapi.Fail
	case reached(th.Warn, n, bad, age):
		return basecheckapi.Warn
	}
	return basecheckapi.Pass
}

// shareText is bad / n as decimal text with four places, empty when n is 0.
func shareText(n, bad int64) string {
	if n <= 0 {
		return ""
	}
	return new(big.Rat).SetFrac64(bad, n).FloatString(4)
}

func example(f basecheckapi.Fact) string {
	if f.Day == "" {
		return f.Value
	}
	if f.Value == "" {
		return string(f.Day)
	}
	return f.Value + " " + string(f.Day)
}

// sqlCheck sums the facts of a rule over its subjects. No fact is unknown. For rows a subject with
// no usable rows is itself a fail.
func sqlCheck(rule string, th Threshold, fs []basecheckapi.Fact) basecheckapi.Check {
	c := baseCheck(rule, th)
	if len(fs) == 0 {
		return c
	}
	var n, bad int64
	for _, f := range fs {
		n += f.N
		bad += f.Bad
		if c.Example == "" && f.Bad > 0 {
			c.Example = example(f)
		}
	}
	c.N, c.Bad, c.Share = n, bad, shareText(n, bad)
	if th.Fail.Kind == Empty {
		// emptiness is judged per subject: a sum would hide a bare subject
		c.Level = basecheckapi.Pass
		for _, f := range fs {
			c.Level = worse(c.Level, level(th, f.N, f.Bad, 0))
		}
	} else {
		c.Level = level(th, n, bad, 0)
	}
	return c
}

// noteCheck is shown, never judged: its Level is empty.
func noteCheck(rule string, fs []basecheckapi.Fact) basecheckapi.Check {
	c := basecheckapi.Check{Rule: rule}
	for _, f := range fs {
		c.N += f.N
		c.Bad += f.Bad
		if c.Example == "" && f.Bad > 0 {
			c.Example = example(f)
		}
	}
	c.Share = shareText(c.N, c.Bad)
	return c
}

// ledgerCheck reads the cells of the row: coverage_gap counts days with missing keys,
// coverage_latest the missing and expired keys of the last closed session of its first calendar.
func ledgerCheck(in basecheckapi.Input, info basecheckapi.RowInfo, rule string, th Threshold, cells []basecheckapi.Cell) basecheckapi.Check {
	c := baseCheck(rule, th)
	switch rule {
	case "coverage_gap":
		missing := map[contracts.Day]int{}
		closed, hasClosed := in.Days[firstCalendar(info)]
		for _, cell := range cells {
			if cell.Day < HistoryStart || (hasClosed && cell.Day > closed) {
				continue
			}
			missing[cell.Day] += cell.Missing
		}
		if len(missing) == 0 {
			return c
		}
		c.N = int64(len(missing))
		for d, m := range missing {
			if m > 0 {
				c.Bad++
				if c.Example == "" || string(d) > c.Example {
					c.Example = string(d)
				}
			}
		}
	case "coverage_latest":
		last, ok := in.Days[firstCalendar(info)]
		if !ok {
			return c
		}
		found := false
		for _, cell := range cells {
			if cell.Day == last {
				found = true
				c.N += int64(cell.Keys)
				c.Bad += int64(cell.Missing + cell.Expired)
			}
		}
		if !found {
			return c
		}
		c.Example = string(last)
	default:
		return c
	}
	c.Share = shareText(c.N, c.Bad)
	c.Level = level(th, c.N, c.Bad, 0)
	return c
}

// ageCheck judges the worst age among the subjects that have a newest day; no newest fact is
// unknown. Bad is the worst age in the unit of the limit.
func ageCheck(in basecheckapi.Input, info basecheckapi.RowInfo, rule string, th Threshold, newest []basecheckapi.Fact) basecheckapi.Check {
	c := baseCheck(rule, th)
	if len(newest) == 0 {
		return c
	}
	kind := th.Fail.Kind
	if kind == None {
		kind = th.Warn.Kind
	}
	c.Level = basecheckapi.Pass
	for i, f := range newest {
		a, ok := subjectAge(in, info, f)
		l := basecheckapi.Unknown
		if ok {
			l = level(th, 0, 0, a.value(kind))
		}
		if i == 0 || rank(l) > rank(c.Level) || (l == c.Level && ok && int64(a.value(kind)) > c.Bad) {
			if ok {
				c.Bad = int64(a.value(kind))
			}
			c.Example = string(a.Newest)
			if a.Subject != "" {
				c.Example = a.Subject + " " + c.Example
			}
		}
		c.Level = worse(c.Level, l)
	}
	return c
}

// ageNumbers are the worst calendar days and worst sessions among the subjects of a row.
func ageNumbers(in basecheckapi.Input, info basecheckapi.RowInfo, newest []basecheckapi.Fact) (days, sessions int) {
	for _, f := range newest {
		a, ok := subjectAge(in, info, f)
		if !ok {
			continue
		}
		if a.Days > days {
			days = a.Days
		}
		if a.Sessions > sessions {
			sessions = a.Sessions
		}
	}
	return days, sessions
}
