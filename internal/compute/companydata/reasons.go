package companydata

import (
	"strings"

	"github.com/andrewrong/optdesk/internal/compute/companydataapi"
)

// reasonTexts holds the one sentence each reason prints (Decision 10 of docs/design/data-collection.md).
var reasonTexts = map[companydataapi.Reason]string{
	companydataapi.ReasonJP:              "Japanese names have no statements source, so their statements are left out on purpose.",
	companydataapi.ReasonNoCIK:           "No SEC number is set for this name, so type it on the underlyings page.",
	companydataapi.ReasonFundamentalsOff: "The fundamentals flag of this name is off, so no statements are fetched.",
	companydataapi.ReasonNoStatements:    "No US-GAAP statements are stored: either this is an IFRS filer, whose quarters are left out on purpose, or the daily run has not stored it yet.",
	companydataapi.ReasonAnnualOnly:      "The filer reports once a year on form 20-F or 40-F, so there are no quarters.",
	companydataapi.ReasonNoInstrument:    "No instrument is resolved yet, so no bars can be collected.",
	companydataapi.ReasonNoBars:          "The instrument is resolved but no daily bar is stored.",
	companydataapi.ReasonNoNextReport:    "No report date is listed for this name.",
}

func missingOf(r companydataapi.Reason) companydataapi.Missing {
	return companydataapi.Missing{Reason: r, Text: reasonTexts[r]}
}

// reasons lists what is missing and why, in a fixed order; it is never nil.
func reasons(in companydataapi.SummaryInput) []companydataapi.Missing {
	out := []companydataapi.Missing{}
	if r, ok := statementsReason(in); ok {
		out = append(out, missingOf(r))
	}
	if len(in.BarDays) == 0 {
		if in.HasInstrument {
			out = append(out, missingOf(companydataapi.ReasonNoBars))
		} else {
			out = append(out, missingOf(companydataapi.ReasonNoInstrument))
		}
	}
	if in.NextReport == nil {
		out = append(out, missingOf(companydataapi.ReasonNoNextReport))
	}
	return out
}

// statementsReason says why a name has no quarters, when it has none for a known reason. An IFRS
// filer leaves no stored row, so it reads no_statements (Decision 11).
func statementsReason(in companydataapi.SummaryInput) (companydataapi.Reason, bool) {
	switch {
	case in.Market == "JP":
		return companydataapi.ReasonJP, true
	case !strings.HasPrefix(in.Issuer, "cik:"):
		return companydataapi.ReasonNoCIK, true
	case !in.Fundamentals:
		return companydataapi.ReasonFundamentalsOff, true
	}
	gaap, annual, quarterly := false, false, false
	for _, f := range in.Facts {
		if f.Taxonomy == "us-gaap" {
			gaap = true
		}
		switch strings.TrimSuffix(f.Form, "/A") {
		case "20-F", "40-F":
			annual = true
		case "10-Q":
			quarterly = true
		}
	}
	switch {
	case !gaap && !annual:
		return companydataapi.ReasonNoStatements, true
	case annual && !quarterly:
		return companydataapi.ReasonAnnualOnly, true
	}
	return "", false
}
