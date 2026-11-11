package main

import (
	"context"

	"github.com/andrewrong/optdesk/internal/jobs/computefigures"
	"github.com/andrewrong/optdesk/internal/store/instrument"
)

// openIndexIDsDefault binds instrument.OpenIndexIDs to computefigures.OpenIndexIDsFunc. On error it returns a nil
// interface, not a *instrument.IndexIDs nil held in one, so a caller's nil check holds.
func openIndexIDsDefault(ctx context.Context, dsn string) (computefigures.IndexIDStore, error) {
	s, err := instrument.OpenIndexIDs(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return s, nil
}
