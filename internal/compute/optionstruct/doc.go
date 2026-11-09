// Package optionstruct implements compute.OptionStructures (built by
// compute.NewOptionStructuresFunc): it groups the open option positions of one alias into
// structures, names their kind, and gives the risk figures that need no market price.
// It is pure: exact math/big.Rat arithmetic on decimal text, no float, no clock, no I/O.
package optionstruct
