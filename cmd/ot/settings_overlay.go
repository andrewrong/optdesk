package main

import (
	"context"
	"slices"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
)

// overlayReader is the owner reader the preview of POST /owner-settings builds its page over: the stored rows
// with the parsed change applied, so the page shows the posted values and the confirm form posts them again.
// History and Version are the store's. It writes nothing.
type overlayReader struct {
	ownerset.Reader
	ch settingsapi.FormChange
}

// Params returns the stored rows with the text of every changed key put in place of the stored value, a key with
// no stored row gaining one, sorted by key.
func (o overlayReader) Params(ctx context.Context, at time.Time) ([]ownerset.Param, error) {
	rows, err := o.Reader.Params(ctx, at)
	if err != nil {
		return nil, err
	}
	rows = slices.Clone(rows)
	for _, f := range o.ch.Params {
		i := slices.IndexFunc(rows, func(p ownerset.Param) bool { return p.Key == f.Key })
		if i >= 0 {
			rows[i].Value = f.Text
			continue
		}
		rows = append(rows, ownerset.Param{Key: f.Key, Value: f.Text})
	}
	slices.SortFunc(rows, func(a, b ownerset.Param) int {
		switch {
		case a.Key < b.Key:
			return -1
		case a.Key > b.Key:
			return 1
		}
		return 0
	})
	return rows, nil
}

// Exclusions returns the stored rows with the flag of every changed instrument set, an instrument with no stored
// row gaining one, sorted by id.
func (o overlayReader) Exclusions(ctx context.Context, at time.Time) ([]ownerset.Exclusion, error) {
	rows, err := o.Reader.Exclusions(ctx, at)
	if err != nil {
		return nil, err
	}
	rows = slices.Clone(rows)
	for id, excluded := range o.ch.Exclusions {
		i := slices.IndexFunc(rows, func(e ownerset.Exclusion) bool { return e.InstrumentID == id })
		if i >= 0 {
			rows[i].Excluded = excluded
			continue
		}
		rows = append(rows, ownerset.Exclusion{InstrumentID: id, Excluded: excluded})
	}
	slices.SortFunc(rows, func(a, b ownerset.Exclusion) int {
		switch {
		case a.InstrumentID < b.InstrumentID:
			return -1
		case a.InstrumentID > b.InstrumentID:
			return 1
		}
		return 0
	})
	return rows, nil
}
