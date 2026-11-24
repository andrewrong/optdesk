package main

import (
	"context"
	"strings"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// Design: owner ask optdesk-5950 (issue #3769): a name whose latest row of the stored list has options off is not collected
// by the option_open families, whether the owner holds its shares or an option on it. The filter is applied at the wiring of
// these families only (plan, resolve, gap, option list, and the Japan ones that share the wiring).

// optionsOffKeys reads the stored list once and returns the row keys of the names with options off.
// A read error is retryable and no name is answered from a partial list.
func optionsOffKeys(ctx context.Context, list underlying.Reader) (map[string]bool, error) {
	rows, err := list.Current(ctx)
	if err != nil {
		return nil, &listedError{kind: contracts.Retryable, msg: "read underlying list failed", err: err}
	}
	off := map[string]bool{}
	for _, r := range rows {
		if !r.Options {
			off[r.Key] = true
		}
	}
	return off, nil
}

// optionsOn is f with the held names of the options-off rows left out, so that followedBy gives such a name no options
// group even when the owner holds an option on it. The other families keep the f they have.
func (f jobFactory) optionsOn() jobFactory {
	if f.list == nil || f.held == nil {
		return f
	}
	inner, list := f.held, f.list
	f.held = func(ctx context.Context) (map[string]bool, error) {
		held, err := inner(ctx)
		if err != nil {
			return nil, err
		}
		off, err := optionsOffKeys(ctx, list)
		if err != nil {
			return nil, err
		}
		out := make(map[string]bool, len(held))
		for k := range held {
			if !off[strings.TrimPrefix(k, "option:")] {
				out[k] = true
			}
		}
		return out, nil
	}
	return f
}

// optionsOnAccounts is rd with the positions of a name with options off left out, stock rows and option rows alike.
// A name on no row of the list passes. Without a list or a reader it is rd.
func optionsOnAccounts(rd store.AccountReader, list underlying.Reader) store.AccountReader {
	if rd == nil || list == nil {
		return rd
	}
	return optionsOnReader{AccountReader: rd, list: list}
}

type optionsOnReader struct {
	store.AccountReader
	list underlying.Reader
}

func (r optionsOnReader) Positions(ctx context.Context, alias, day string) ([]store.Position, error) {
	rows, err := r.AccountReader.Positions(ctx, alias, day)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	off, err := optionsOffKeys(ctx, r.list)
	if err != nil {
		return nil, err
	}
	out := make([]store.Position, 0, len(rows))
	for _, p := range rows {
		// A position carries no market: the currency tells the Japan names from the rest.
		market := "US"
		if p.Currency == "JPY" {
			market = "JP"
		}
		if !off[nameKey(market, p.Symbol)] {
			out = append(out, p)
		}
	}
	return out, nil
}
