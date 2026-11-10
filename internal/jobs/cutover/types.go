package cutover

import (
	"context"
	"io"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

// VerifyResult counts the lines read, the lines with no stored row of their key (Missing) and the lines whose
// key has a stored row with another hash (Different).
type VerifyResult struct {
	Lines, Missing, Different int
}

// ImportAccountFunc imports the account.jsonl of the cut-over export (internal/jobs/cutover.ImportAccount):
// one store.Capture per line, insert-only, the capture id <alias>:<day>:import. aliasMap maps the alias of the
// file to the alias of the store (nil keeps every alias; a non-nil map refuses an alias it does not name
// before any write). A line after through is refused. Errors name the line number, never its content.
type ImportAccountFunc func(
	ctx context.Context, r io.Reader, st store.AccountStore, aliasMap map[string]string, through contracts.Day,
) (jobs.ImportResult, error)

// VerifyAccountFunc checks the same file against the store and writes nothing
// (internal/jobs/cutover.VerifyAccount); aliasMap and through are as in ImportAccountFunc.
type VerifyAccountFunc func(ctx context.Context, r io.Reader, rd store.AccountReader, aliasMap map[string]string, through contracts.Day) (VerifyResult, error)

// FigureStatus is the outcome of the comparison of one figure.
type FigureStatus string

// The five statuses of a compared figure.
const (
	FigureChecked     FigureStatus = "checked"
	FigureExplained   FigureStatus = "explained"
	FigureUnexplained FigureStatus = "unexplained"
	FigureNew         FigureStatus = "new"
	FigureUnchecked   FigureStatus = "unchecked"
)

// FigureResult is the status of one figure of the register and, for unchecked, the reason.
type FigureResult struct {
	ID      string
	Version int
	Status  FigureStatus
	Reason  string
}

// FigureReport is the result of a comparison; OK is true only when no figure is unexplained.
type FigureReport struct {
	Results []FigureResult
	OK      bool
}

// CompareFiguresFunc compares the old figure file with optdesk's readings (internal/jobs/cutover.CompareFigures):
// old is figure.jsonl, register the figure register, classes the class file and explained the owner's
// explained entries.
type CompareFiguresFunc func(ctx context.Context, old, register, classes, explained io.Reader, rd store.ReadingReader) (FigureReport, error)
