package option_boards

import "fmt"

const (
	ivMissing = "IV 缺"
	ivTail    = " · HV30 缺 · IVP 缺"
)

// fillTop writes the stored parts of the top bar; the day change, quote and refresh fields have no source and
// stay null.
func fillTop(t *topBar, rows []stockRow) error {
	first := rows[0].C
	if first.Spot != nil {
		t.Spot = strPtr("现价 " + *first.Spot)
	}
	t.IV = strPtr(ivMissing)
	for _, r := range rows {
		if r.C.Rank == nil {
			continue
		}
		if r.C.Iv != nil {
			pct, err := shiftPercent(*r.C.Iv)
			if err != nil {
				return fmt.Errorf("option_boards: iv of instrument %d: %w", r.C.InstrumentID, err)
			}
			t.IV = strPtr("IV " + pct + ivTail)
		}
		break
	}
	e, err := earningsWord(rows[0])
	if err != nil {
		return err
	}
	t.Earnings = strPtr("财报 " + e)
	return nil
}

// earningsWord is "MM-DD" for a listed date, "无" for absent and "缺" otherwise.
func earningsWord(r stockRow) (string, error) {
	switch {
	case r.C.EarningsState == "listed" && r.C.NextEarnings != nil:
		return monthDay(*r.C.NextEarnings)
	case r.C.EarningsState == "absent":
		return "无", nil
	}
	return gapText, nil
}
