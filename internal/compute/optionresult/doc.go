// Package optionresult holds the implementation of compute.OptionResults (built by
// compute.NewOptionResultsFunc): the realized result of the closed option positions of one
// alias by side and currency (option-positions.md, Decision 9). It is pure: exact
// math/big.Rat arithmetic, no float, no clock, no I/O, nothing stored.
package optionresult
