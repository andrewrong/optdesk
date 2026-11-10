package option_data_page

import "strconv"

// Units and decimals of the formatting rule of docs/design/option-page.md#interface.
const (
	unitPrice = "price"
	unitCount = "count"
	unitIV    = "iv_decimal"
)

type figure struct {
	Kind  string  `json:"kind"`
	Value float64 `json:"value"`
	Text  string  `json:"text"`
	Unit  string  `json:"unit"`
	// ApproxUSDText is "≈ US$ N" beside the price of a non-USD underlying; absent is omitted.
	ApproxUSDText string `json:"approx_usd_text,omitempty"`
}

type gap struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

func newGap(text string) gap { return gap{Kind: "gap", Reason: "no_data", Text: text} }

// noData is the cell of a null stored value.
func noData() gap { return newGap("no data") }

// cell formats a stored value: a nil value is a no_data gap, never a zero.
func cell(v *float64, unit string, decimals int) any {
	if v == nil {
		return noData()
	}
	return figure{Kind: "value", Value: *v, Text: strconv.FormatFloat(*v, 'f', decimals, 64), Unit: unit}
}

func priceCell(v *float64) any { return cell(v, unitPrice, 2) }

func countCell(v *float64) any { return cell(v, unitCount, 0) }

func greekCell(v *float64, unit string) any { return cell(v, unit, 4) }

// ivCell shows the stored fraction times 100 with one decimal; the value stays the stored one.
func ivCell(v *float64) any {
	if v == nil {
		return noData()
	}
	return figure{Kind: "value", Value: *v, Text: strconv.FormatFloat(*v*100, 'f', 1, 64) + "%", Unit: unitIV}
}

var statusText = map[int]string{1: "ok", 2: "no quote", 3: "not listed", 4: "stale", 5: "error"}

// limit keeps at most n leading items.
func limit[T any](s []T, n int) []T {
	if len(s) > n {
		return s[:n]
	}
	return s
}
