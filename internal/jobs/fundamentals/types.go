package fundamentals

import "github.com/andrewrong/optdesk/internal/jobs"

// Types of the fundamentals job; each name is an alias of the one in package jobs.
type (
	NewFundamentalsFunc = jobs.NewFundamentalsFunc
	FundamentalsSkip    = jobs.FundamentalsSkip
	FundamentalsSkips   = jobs.FundamentalsSkips
)
