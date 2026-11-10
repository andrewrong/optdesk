package earningsrefresh

import "github.com/andrewrong/optdesk/internal/jobs"

// Types of the earningsrefresh job; each name is an alias of the one in package jobs.
type (
	NewEarningsRefreshFunc        = jobs.NewEarningsRefreshFunc
	NewEarningsRefreshSymbolsFunc = jobs.NewEarningsRefreshSymbolsFunc
)
