package main

import (
	"github.com/andrewrong/optdesk/internal/compute/optionevidence"
	"github.com/andrewrong/optdesk/internal/compute/optionevidenceapi"
	"github.com/andrewrong/optdesk/internal/compute/optiontier"
	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
)

// newEvaluator returns the evidence evaluator over the bsm pricer with the default parameters, as the
// interface; the page Config and the tier rule share it.
func newEvaluator() optionevidenceapi.Evaluator {
	return optionevidence.New(newBSMValuer(), optionevidence.DefaultParams())
}

// newTierer returns the tier rule of option_candidates_page over the evaluator, as the interface, so the
// value handed to the page has the static type optiontierapi.Tierer.
func newTierer() optiontierapi.Tierer { return optiontier.New(newEvaluator()) }
