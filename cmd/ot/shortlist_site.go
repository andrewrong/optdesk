package main

import (
	"github.com/andrewrong/optdesk/internal/compute/optionshortlist"
	"github.com/andrewrong/optdesk/internal/compute/optionshortlistapi"
)

// newShortlister returns the short-put shortlist rule of option_candidates_page, as the interface, so the
// value handed to the page has the static type optionshortlistapi.Shortlister
// (docs/design/candidates-page-v2.md#Decisions).
func newShortlister() optionshortlistapi.Shortlister { return optionshortlist.New() }
