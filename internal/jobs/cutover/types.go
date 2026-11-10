package cutover

import "github.com/andrewrong/optdesk/internal/jobs"

// Types of the cutover job; each name is an alias of the one in package jobs.
type (
	VerifyResult       = jobs.VerifyResult
	ImportAccountFunc  = jobs.ImportAccountFunc
	VerifyAccountFunc  = jobs.VerifyAccountFunc
	FigureStatus       = jobs.FigureStatus
	FigureResult       = jobs.FigureResult
	FigureReport       = jobs.FigureReport
	CompareFiguresFunc = jobs.CompareFiguresFunc
)

// The five statuses of a compared figure.
const (
	FigureChecked     = jobs.FigureChecked
	FigureExplained   = jobs.FigureExplained
	FigureUnexplained = jobs.FigureUnexplained
	FigureNew         = jobs.FigureNew
	FigureUnchecked   = jobs.FigureUnchecked
)
