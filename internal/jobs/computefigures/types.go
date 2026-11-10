package computefigures

import "github.com/andrewrong/optdesk/internal/jobs"

// Types of the computefigures job; each name is an alias of the one in package jobs.
type (
	ComputeStores             = jobs.ComputeStores
	NewComputeFiguresFunc     = jobs.NewComputeFiguresFunc
	NewComputeFiguresEaseFunc = jobs.NewComputeFiguresEaseFunc
)
