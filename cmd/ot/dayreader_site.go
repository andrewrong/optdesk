package main

import (
	"context"
	"io"

	"github.com/andrewrong/optdesk/internal/store/optionday"
)

// openDayReader opens the obs.option_day reader with the pages line when deps has an opener, else the reader is nil
// (the coverage column of option_tracking_page is the gap no_reader). ok is false when the open failed.
func openDayReader(ctx context.Context, a *app, dsn string, track func(io.Closer, error) bool) (optionday.DayReader, bool) {
	if a.deps.openOptionDay == nil {
		return nil, true
	}
	d, err := a.deps.openOptionDay(ctx, dsn)
	if !track(d, err) {
		return nil, false
	}
	return d, true
}
