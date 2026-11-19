package recon

import "github.com/andrewrong/optdesk/internal/compute/reconapi"

// Field is the tolerance of one compared amount of a pair.
type Field struct {
	Name string
	Tol  reconapi.Tolerance
}

// Rule is one pair of the version 2 table of docs/design/base-data-page.md, as data. Tolerances
// are data, not code: Compare serves every pair. Abs is an absolute room and Rel a share of the
// larger amount (0.001 is 0.1 percent); an empty text means none, so the amounts must be equal.
// R7 and R8's own bounds are not here: R7 is judged inside the stored chain.
//
// WarnShare and FailShare are the shares of bad items over N above which the pair is warn and fail;
// MinComparedShare is the least share of the asked pairs that must be compared, else the level is at
// best warn. They are set for R6 only; the pairs R1 to R5 fail on any finding.
type Rule struct {
	ID                  string
	Fields              []Field
	OtherMissingSkipped bool
	WarnShare           string
	FailShare           string
	MinComparedShare    string
}

// Rules holds R1 to R9 in order.
var Rules = []Rule{
	{ID: "R1", Fields: []Field{{"quantity", reconapi.Tolerance{}}, {"avg_cost", reconapi.Tolerance{Rel: "0.001"}}}},
	{ID: "R2", Fields: []Field{{"quantity", reconapi.Tolerance{}}, {"price", reconapi.Tolerance{Abs: "0.0001"}}, {"commission", reconapi.Tolerance{Abs: "0.01"}}}},
	{ID: "R3", Fields: []Field{{"amount", reconapi.Tolerance{Abs: "1", Rel: "0.0005"}}}},
	{ID: "R4", Fields: []Field{{"rate", reconapi.Tolerance{Rel: "0.003"}}}},
	{ID: "R5", Fields: []Field{{"quantity", reconapi.Tolerance{}}}},
	{ID: "R6", Fields: []Field{{"close", reconapi.Tolerance{Rel: "0.001"}}}, OtherMissingSkipped: true,
		WarnShare: "0.001", FailShare: "0.01", MinComparedShare: "0.5"},
	{ID: "R7", Fields: []Field{{"gap", reconapi.Tolerance{}}}},
	{ID: "R8", Fields: []Field{{"close", reconapi.Tolerance{Rel: "0.005"}}}},
	{ID: "R9", Fields: []Field{{"value", reconapi.Tolerance{}}}},
}

// RuleOf returns the rule with the id.
func RuleOf(id string) (Rule, bool) {
	for _, r := range Rules {
		if r.ID == id {
			return r, true
		}
	}
	return Rule{}, false
}
