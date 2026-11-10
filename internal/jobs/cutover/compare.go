package cutover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sort"
	"strings"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// Reasons of an unchecked figure.
const (
	reasonPathNotFound   = "path_not_found"
	reasonNoOldValue     = "no_old_value"
	reasonNotComputedYet = "not_computed_yet"
)

// oldRow is one line of the old figure.jsonl. A nil Value is a figure the exporter could not read; Reason
// then says path_not_found when the page path was missing.
type oldRow struct {
	Name        string  `json:"name"`
	Version     int     `json:"version"`
	SubjectKind string  `json:"subject_kind"`
	Subject     string  `json:"subject"`
	Day         string  `json:"day"`
	Value       *string `json:"value"`
	Places      int     `json:"places"`
	Reason      string  `json:"reason"`
}

type subjectKey struct{ name, kind, subject string }

// CompareFigures implements CompareFiguresFunc (Decisions 8, 9 and 13 of docs/design/cut-over.md). It
// sets, for every [[figure]] of the register in file order, the old rows beside optdesk's readings and gives
// one status. It reads the readings once for each distinct (name, subject kind, subject) of the old file over
// the range of its days. An id that matches no entry of classes is an error naming the ids, with no report.
// It writes nothing.
func CompareFigures(ctx context.Context, old, register, classes, explained io.Reader, rd store.ReadingReader) (FigureReport, error) {
	figs, err := readRegister(register)
	if err != nil {
		return FigureReport{}, err
	}
	table, err := readClasses(classes)
	if err != nil {
		return FigureReport{}, err
	}
	expl, err := readExplained(explained)
	if err != nil {
		return FigureReport{}, err
	}
	classOf := make([]string, len(figs))
	var missing []string
	for i, f := range figs {
		c, ok := table.classify(f.ID, f.Version)
		if !ok {
			missing = append(missing, f.ID)
		}
		classOf[i] = c
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return FigureReport{}, fmt.Errorf("figures with no class: %s", strings.Join(missing, ", "))
	}
	rows, err := readOld(old)
	if err != nil {
		return FigureReport{}, err
	}
	have, err := readReadings(ctx, rd, rows)
	if err != nil {
		return FigureReport{}, err
	}
	var results []FigureResult
	for i, f := range figs {
		res := FigureResult{ID: f.ID, Version: f.Version}
		res.Status, res.Reason = judge(f, classOf[i], rows, have, expl)
		results = append(results, res)
	}
	return report(results), nil
}

// report is the one place OK is set.
func report(results []FigureResult) FigureReport {
	ok := true
	for _, r := range results {
		if r.Status == FigureUnexplained {
			ok = false
		}
	}
	return FigureReport{Results: results, OK: ok}
}

func readOld(r io.Reader) ([]oldRow, error) {
	var rows []oldRow
	err := ReadLines(r, func(_ int, raw []byte) error {
		var row oldRow
		if err := json.Unmarshal(raw, &row); err != nil {
			return errors.New("not a figure line")
		}
		if row.Name == "" || row.Day == "" || row.Places < 0 {
			return errors.New("figure line lacks name or day, or has negative places")
		}
		if row.Value != nil {
			if _, err := roundHalfEven(*row.Value, row.Places); err != nil {
				return err
			}
		}
		rows = append(rows, row)
		return nil
	})
	return rows, err
}

// readReadings reads the readings of each distinct subject of the old file over the range of its days and
// indexes them by (name, version, subject kind, subject, day).
func readReadings(ctx context.Context, rd store.ReadingReader, rows []oldRow) (map[readKey]contracts.ReadingRecord, error) {
	have := map[readKey]contracts.ReadingRecord{}
	if len(rows) == 0 {
		return have, nil
	}
	from, to := rows[0].Day, rows[0].Day
	seen := map[subjectKey]bool{}
	var keys []subjectKey
	for _, r := range rows {
		from, to = min(from, r.Day), max(to, r.Day)
		k := subjectKey{r.Name, r.SubjectKind, r.Subject}
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	for _, k := range keys {
		recs, err := rd.LatestReadings(ctx, k.name, k.kind, k.subject, contracts.Day(from), contracts.Day(to))
		if err != nil {
			return nil, fmt.Errorf("read readings of %s: %w", k.name, err)
		}
		for _, rec := range recs {
			have[readKey{rec.Name, rec.Version, rec.SubjectKind, rec.Subject, rec.Day.Format(dayLayout)}] = rec
		}
	}
	return have, nil
}

type readKey struct {
	name    string
	version int
	kind    string
	subject string
	day     string
}

// judge gives the status of one figure.
func judge(f figureEntry, class string, rows []oldRow, have map[readKey]contracts.ReadingRecord, expl map[string]bool) (FigureStatus, string) {
	if class == classNew {
		return FigureNew, ""
	}
	var mine []oldRow
	for _, r := range rows {
		if r.Name == f.ID && r.Version == f.Version {
			mine = append(mine, r)
		}
	}
	var valued []oldRow
	reason := reasonNoOldValue
	for _, r := range mine {
		if r.Value != nil {
			valued = append(valued, r)
		} else if r.Reason == reasonPathNotFound {
			reason = reasonPathNotFound
		}
	}
	if len(valued) == 0 {
		return FigureUnchecked, reason
	}
	computed, differs := false, false
	for _, r := range valued {
		rec, ok := have[readKey{r.Name, r.Version, r.SubjectKind, r.Subject, r.Day}]
		if !ok || rec.Value == nil {
			differs = true
			continue
		}
		computed = true
		a, _ := roundHalfEven(*r.Value, r.Places)
		b, err := roundHalfEven(*rec.Value, r.Places)
		if err != nil || a != b {
			differs = true
		}
	}
	switch {
	case !computed && !anyReading(have, f):
		return FigureUnchecked, reasonNotComputedYet
	case !differs:
		return FigureChecked, ""
	case expl[f.ID]:
		return FigureExplained, ""
	}
	return FigureUnexplained, ""
}

// anyReading is true when optdesk holds a reading of the figure at all.
func anyReading(have map[readKey]contracts.ReadingRecord, f figureEntry) bool {
	for k := range have {
		if k.name == f.ID && k.version == f.Version {
			return true
		}
	}
	return false
}

// roundHalfEven rounds decimal text to places decimals, ties to even, and returns it with exactly that many
// places. Only plain decimal text is accepted (sign, digits, one point).
func roundHalfEven(text string, places int) (string, error) {
	if !plainDecimal(text) {
		return "", errors.New("value is not decimal text")
	}
	r, ok := new(big.Rat).SetString(text)
	if !ok {
		return "", errors.New("value is not decimal text")
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(places)), nil)
	r.Mul(r, new(big.Rat).SetInt(scale))
	num, den := r.Num(), r.Denom()
	q, rem := new(big.Int).QuoRem(new(big.Int).Abs(num), den, new(big.Int))
	switch new(big.Int).Mul(rem, big.NewInt(2)).Cmp(den) {
	case 1:
		q.Add(q, big.NewInt(1))
	case 0:
		if q.Bit(0) == 1 {
			q.Add(q, big.NewInt(1))
		}
	}
	neg := num.Sign() < 0 && q.Sign() != 0
	digits := q.String()
	if places > 0 {
		if len(digits) <= places {
			digits = strings.Repeat("0", places-len(digits)+1) + digits
		}
		digits = digits[:len(digits)-places] + "." + digits[len(digits)-places:]
	}
	if neg {
		digits = "-" + digits
	}
	return digits, nil
}

func plainDecimal(s string) bool {
	s = strings.TrimPrefix(strings.TrimPrefix(s, "-"), "+")
	point, digits := false, 0
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
			digits++
		case c == '.' && !point:
			point = true
		default:
			return false
		}
	}
	return digits > 0
}
