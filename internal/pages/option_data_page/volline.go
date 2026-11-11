package option_data_page

import (
	"math"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const volLineMissing = "IV n/a · HV30 n/a · IVP n/a"

// applyVolLine writes underlying.vol_line (docs/design/option-pages-v22.md Decisions 1, 5, 6): the stored
// iv_decimal of one chain row, then HV30 and IVP as missing until compute gives them a producer.
// kept and rows are the same rows in the same order, rows already carrying the terms.
func applyVolLine(sessionDay string, price *float64, kept []contracts.OptionEod, rows []chainRow, u *underlying) {
	u.VolLine = volLineMissing
	if price == nil {
		return
	}
	best := -1
	for i, r := range rows {
		if kept[i].IvDecimal == nil || r.Expiry == nil || *r.Expiry < sessionDay || r.Right == nil {
			continue
		}
		if _, ok := strikeOf(r); !ok {
			continue
		}
		if best < 0 || nearer(*price, r, rows[best]) {
			best = i
		}
	}
	if best >= 0 {
		iv := ivCell(kept[best].IvDecimal).(figure)
		u.VolLine = "IV " + iv.Text + " · HV30 n/a · IVP n/a"
	}
}

func strikeOf(r chainRow) (float64, bool) {
	f, ok := r.Strike.(figure)
	return f.Value, ok
}

// nearer reports whether a is chosen over b: earlier expiry, nearer strike, lower strike, call before put.
func nearer(price float64, a, b chainRow) bool {
	if *a.Expiry != *b.Expiry {
		return *a.Expiry < *b.Expiry
	}
	sa, _ := strikeOf(a)
	sb, _ := strikeOf(b)
	if da, db := math.Abs(sa-price), math.Abs(sb-price); da != db {
		return da < db
	}
	if sa != sb {
		return sa < sb
	}
	return *a.Right == "call" && *b.Right != "call"
}
