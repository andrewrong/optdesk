package serieshistory

import "github.com/andrewrong/optdesk/internal/jobs"

// Types of the serieshistory job; each name is an alias of the one in package jobs.
type (
	SeriesLoaded         = jobs.SeriesLoaded
	SeriesHistoryLoader  = jobs.SeriesHistoryLoader
	NewSeriesHistoryFunc = jobs.NewSeriesHistoryFunc
)
