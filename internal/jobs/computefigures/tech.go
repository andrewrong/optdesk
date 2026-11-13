package computefigures

import "github.com/andrewrong/optdesk/internal/contracts"

// WithTech adds the key market.tech_state|market|NDX_SPX and its step to a computefigures job. This is the
// declaration stub: it returns the job unchanged until the step is written.
func WithTech(j contracts.Job, _ TechDeps) contracts.Job {
	return j
}
