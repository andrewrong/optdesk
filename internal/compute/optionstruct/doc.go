// Package optionstruct implements compute.OptionStructures (built by
// compute.NewOptionStructuresFunc): it groups the open option positions of one alias into
// structures, names their kind, and gives the risk figures that need no market price.
// The kinds come from a definition table matched greedily by descending leg count, each match
// taking the largest whole multiple of lots (match_table.go, match.go).
// It is pure: exact math/big.Rat arithmetic on decimal text, no float, no clock, no I/O.
//
// Algorithm and definition table after QuantConnect Lean, OptionStrategyMatcher, Apache-2.0, https://github.com/QuantConnect/Lean
package optionstruct
