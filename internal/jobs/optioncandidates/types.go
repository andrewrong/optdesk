package optioncandidates

import "github.com/andrewrong/optdesk/internal/jobs"

// Types of the optioncandidates job; each name is an alias of the one in package jobs.
type (
	CandidateStores              = jobs.CandidateStores
	NewOptionCandidatesJobFunc   = jobs.NewOptionCandidatesJobFunc
	NewOptionCandidatesCallsFunc = jobs.NewOptionCandidatesCallsFunc
	SettingsVersionFunc          = jobs.SettingsVersionFunc
)
