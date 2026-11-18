// Package reconapi declares the compare function of the version 2 pairs of the base data page, its
// data and the function that turns a result into a pair level. Declarations only: no function
// body. The implementation is internal/compute/recon.
// Design: docs/design/base-data-page.md#14 (Interface 5).
package reconapi

import "github.com/andrewrong/optdesk/internal/compute/basecheckapi"

// Kind is the result of comparing one key.
type Kind string

// The five kinds. Skipped is never counted in N.
const (
	Match        Kind = "match"
	Diff         Kind = "diff"
	MissingOurs  Kind = "missing_ours"
	MissingOther Kind = "missing_other"
	Skipped      Kind = "skipped" // not compared; Item.Reason says why
)

// Value is one keyed amount, as decimal text.
type Value struct {
	Key, Amount string
}

// Tolerance is the room inside which two amounts match, as decimal text: Abs absolute, Rel a
// share of the larger amount. Empty means none.
type Tolerance struct {
	Abs, Rel string
}

// Item is the result for one key. Diff is decimal text; Reason is set for kind skipped.
type Item struct {
	Key, Ours, Other, Diff, Reason string
	Kind                           Kind
}

// Input is two keyed lists and a tolerance. Skip maps a key to the reason it is not compared.
// OtherMissingSkipped turns a key only on our side into kind skipped (used by R6).
type Input struct {
	Ours, Other         []Value
	Skip                map[string]string
	OtherMissingSkipped bool
	Tol                 Tolerance
}

// Output is the items and one count per kind; N is matched + diff + missing_ours + missing_other.
type Output struct {
	Items                                                []Item
	N, Matched, Diff, MissingOurs, MissingOther, Skipped int
}

// Func compares two keyed lists. The one implementation is internal/compute/recon.
type Func func(Input) Output

// VerdictFunc turns the counts of a run into the level of a pair. pairsAsked is the pairs the run
// asked the source for, pairsCompared those with at least one answer.
type VerdictFunc func(pair string, o Output, pairsAsked, pairsCompared int) basecheckapi.Level
