// Package candidate_text formats stored option candidate rows as plain text for the command line.
// It sorts nothing and computes nothing: the text is the stored columns in the stored order.
package candidate_text

import (
	"encoding/json"
	"strings"

	"github.com/andrewrong/optdesk/internal/store"
)

const (
	noRows = "no candidates"
	dash   = "-"
)

// Format returns the plain text of the stored rows in the stored order, one line per row: for an ok
// row the company, kind, alias, expiry, strike, bid, annualized yield, break-even and marks; with all,
// the blocked and unknown rows follow in their place with the reasons of their gates. No rows to show
// gives "no candidates".
func Format(rows []store.Candidate, all bool) string {
	var lines []string
	for _, c := range rows {
		switch {
		case c.Status == "ok":
			lines = append(lines, line(c))
		case all:
			lines = append(lines, line(c)+"  ["+c.Status+"] "+reasons(c.Gates))
		}
	}
	if len(lines) == 0 {
		return noRows
	}
	return strings.Join(lines, "\n")
}

func line(c store.Candidate) string {
	yield := dash
	if c.AnnualizedYield != nil {
		yield = *c.AnnualizedYield
	}
	return strings.Join([]string{
		c.Issuer, c.Kind, orDash(c.Alias), string(c.Expiry),
		"strike " + c.Strike, "bid " + c.BidEod, "yield " + yield, "breakeven " + c.BreakEven,
		"marks " + marks(c.Marks),
	}, "  ")
}

func orDash(s string) string {
	if s == "" {
		return dash
	}
	return s
}

// marks is the stored JSON array of marks joined by commas; text that is not an array is shown as stored.
func marks(text string) string {
	var ms []string
	if err := json.Unmarshal([]byte(text), &ms); err != nil {
		return text
	}
	if len(ms) == 0 {
		return dash
	}
	return strings.Join(ms, ",")
}

// reasons lists gate=reason for every gate that is not a pass, in the stored order; text that is not
// an array of gate results is shown as stored.
func reasons(text string) string {
	var gates []struct {
		Gate   string `json:"gate"`
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(text), &gates); err != nil {
		return text
	}
	var out []string
	for _, g := range gates {
		if g.Status != "pass" {
			out = append(out, g.Gate+"="+g.Reason)
		}
	}
	if len(out) == 0 {
		return dash
	}
	return strings.Join(out, "; ")
}
