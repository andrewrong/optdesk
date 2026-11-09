package optionledger

import (
	"errors"
	"strings"
)

// errRowKey means a section row has no value for a required key attribute, or is a lot row that carries no
// lot attribute; the row is refused and counted, and a key is never guessed.
var errRowKey = errors.New("optionledger: section row without its key attributes")

// sectionSpec is one row of the registry of docs/design/account-flex-fx.md Decision 5: the row element name
// of a section and its key attributes in order, required ones and then optional ones.
type sectionSpec struct {
	Element  string
	Required []string
	Optional []string
	// ByLot marks a section whose rows of level LOT are told apart by the optional attributes alone: such a
	// row with all of them empty is ambiguous and refused.
	ByLot bool
}

// registry lists the expected sections in the order their status rows are written. A later section is one
// more row here and no schema change.
var registry = []sectionSpec{
	{Element: "CashTransaction", Required: []string{"transactionID"}},
	{
		Element:  "OpenPosition",
		Required: []string{"reportDate", "levelOfDetail", "conid"},
		Optional: []string{"openDateTime", "originatingTransactionID"},
		ByLot:    true,
	},
	{Element: "ChangeInNAV", Required: []string{"fromDate", "toDate"}},
	{Element: "EquitySummaryByReportDateInBase", Required: []string{"reportDate"}},
	{Element: "ConversionRate", Required: []string{"reportDate", "fromCurrency", "toCurrency"}},
}

// sectionByElement returns the registry row of a row element name.
func sectionByElement(element string) (sectionSpec, bool) {
	for _, s := range registry {
		if s.Element == element {
			return s, true
		}
	}
	return sectionSpec{}, false
}

// rowKey joins the key attributes of a row with "|": the required ones, then the optional ones, an absent
// optional attribute as empty text. It returns errRowKey when a required attribute is missing or empty, and
// for a ByLot row (OpenPosition) of level LOT with neither openDateTime nor originatingTransactionID (ambiguous).
func rowKey(spec sectionSpec, attrs map[string]string) (string, error) {
	parts := make([]string, 0, len(spec.Required)+len(spec.Optional))
	for _, a := range spec.Required {
		v := attrs[a]
		if v == "" {
			return "", errRowKey
		}
		parts = append(parts, v)
	}
	empty := true
	for _, a := range spec.Optional {
		v := attrs[a]
		if v != "" {
			empty = false
		}
		parts = append(parts, v)
	}
	if spec.ByLot && attrs["levelOfDetail"] == "LOT" && empty {
		return "", errRowKey
	}
	return strings.Join(parts, "|"), nil
}
