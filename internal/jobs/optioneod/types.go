package optioneod

import "github.com/andrewrong/optdesk/internal/jobs"

// Types of the optioneod job; each name is an alias of the one in package jobs.
type (
	NewOptionEodFunc             = jobs.NewOptionEodFunc
	NewOptionEodSymbolsFunc      = jobs.NewOptionEodSymbolsFunc
	NewOptionEodWatchFunc        = jobs.NewOptionEodWatchFunc
	NewOptionEodWatchSymbolsFunc = jobs.NewOptionEodWatchSymbolsFunc
	NewOptionEodHeldFunc         = jobs.NewOptionEodHeldFunc
	OptionEodWindow              = jobs.OptionEodWindow
	NewOptionEodWatchWindowFunc  = jobs.NewOptionEodWatchWindowFunc
)
