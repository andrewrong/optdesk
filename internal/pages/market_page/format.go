package market_page

import (
	"encoding/json"
	"strconv"
)

// pointText writes a stored series point with two decimals; the only float in the package.
func pointText(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

// number is a JSON number written from exact decimal text.
func number(text string) json.Number { return json.Number(text) }
