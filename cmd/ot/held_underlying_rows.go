package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/barsrefresh"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// Design: docs/design/held-underlying-add.md#decisions (7, 8 and 11).

// The texts of an index option on the page (Decision 8): JPY is covered by the job option_eod_jp, any other is not
// supported.
const (
	heldIndexTextJP    = "index option, covered by option_eod_jp"
	heldIndexTextOther = "not supported: index option"
)

// heldIndexText is the text of the page for an index option of currency.
func heldIndexText(currency string) string {
	if currency == "JPY" {
		return heldIndexTextJP
	}
	return heldIndexTextOther
}

// The words the store keeps in position.sec_type (migrations/0019_account.sql), not the broker words STK and OPT.
const (
	heldSecStock  = "stock"
	heldSecOption = "option"
)

// heldPair is one held name that is planned: no row yet, not an index option, and it passes Check.
type heldPair struct{ symbol, currency, key string }

// heldRowsJob is the wrapper of the underlying resolve job that adds a pending row for each held name that has none:
// Plan only reads, Run writes (Decisions 7 and 11 of docs/design/held-underlying-add.md).
type heldRowsJob struct {
	inner     contracts.Job
	list      underlying.Reader
	adder     underlying.HeldAdder
	account   store.AccountReader
	aliases   []string
	clk       clock.Clock
	log       *slog.Logger
	datasetID string
	maxNames  int
	zone      *time.Location
	indexes   map[string]bool
}

// heldRows wraps inner. datasetID is the dataset of the inner keys, maxNames the cap of names in one run and zone the
// zone of the schedule, whose today is the only day that plans held names. The index set comes from the bars manifest.
func heldRows(inner contracts.Job, list underlying.Reader, adder underlying.HeldAdder, account store.AccountReader, aliases []string,
	clk clock.Clock, log *slog.Logger, datasetID string, maxNames int, zone *time.Location) (contracts.Job, error) {
	if inner == nil || list == nil || adder == nil || account == nil || clk == nil || log == nil || zone == nil || maxNames <= 0 {
		return nil, errors.New("held rows: job, list, adder, account reader, clock, log, zone and max names are required")
	}
	m, err := barsrefresh.DefaultManifest()
	if err != nil {
		return nil, fmt.Errorf("held rows: bars manifest: %w", err)
	}
	idx := map[string]bool{}
	for _, i := range m.Indexes {
		idx[strings.ToUpper(i.Symbol)] = true
	}
	return &heldRowsJob{inner: inner, list: list, adder: adder, account: account, aliases: aliases, clk: clk, log: log,
		datasetID: datasetID, maxNames: maxNames, zone: zone, indexes: idx}, nil
}

// ID is the id of the inner job.
func (h *heldRowsJob) ID() string { return h.inner.ID() }

// Schedule is the schedule of the inner job.
func (h *heldRowsJob) Schedule() (calendarID, anchor, zone string, offset time.Duration) {
	if s, ok := h.inner.(interface {
		Schedule() (string, string, string, time.Duration)
	}); ok {
		return s.Schedule()
	}
	return "", "", "", 0
}

// RunBound is the bound of the inner job.
func (h *heldRowsJob) RunBound() time.Duration {
	if s, ok := h.inner.(interface{ RunBound() time.Duration }); ok {
		return s.RunBound()
	}
	return 0
}

// RestartSensitive is the flag of the inner job.
func (h *heldRowsJob) RestartSensitive() bool {
	if s, ok := h.inner.(interface{ RestartSensitive() bool }); ok {
		return s.RestartSensitive()
	}
	return false
}

// Plan is the plan of the inner job and, for today only, one key for each held name with no row. It writes nothing.
// A failed read of the positions or of the list leaves the inner plan as it is, with one Warn and no error: an error
// would make the reconciler skip the whole job.
func (h *heldRowsJob) Plan(ctx context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	keys, hash, err := h.inner.Plan(ctx, day)
	if err != nil {
		return keys, hash, err
	}
	if string(day) != h.clk.Now().In(h.zone).Format(time.DateOnly) {
		return keys, hash, nil
	}
	pairs, err := h.candidates(ctx)
	if err != nil {
		h.log.Warn("held names not read", "component", "ot", "job", h.inner.ID())
		return keys, hash, nil
	}
	if len(pairs) == 0 {
		return keys, hash, nil
	}
	held := make([]contracts.ExpectedKey, 0, len(pairs)+len(keys))
	ids := []string{hash}
	for _, p := range pairs {
		held = append(held, contracts.ExpectedKey{DatasetID: h.datasetID, SubjectKey: p.key})
		ids = append(ids, p.key)
	}
	sum := sha256.Sum256([]byte(strings.Join(ids, "\n")))
	return append(held, keys...), hex.EncodeToString(sum[:]), nil
}

// Run adds the row of the first key that has none and is still held, then runs the inner job, whose read of the due
// rows holds the new row. A name sold since the plan adds nothing and returns nothing. A store error is a Retryable
// result, kept beside what the inner job did.
func (h *heldRowsJob) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	var addErr error
	pairs, err := h.candidates(ctx)
	if err != nil {
		addErr = err
	}
	for _, k := range keys {
		i := sort.Search(len(pairs), func(i int) bool { return pairs[i].key >= k.SubjectKey })
		if i == len(pairs) || pairs[i].key != k.SubjectKey {
			continue
		}
		if _, err := h.adder.Add(ctx, pairs[i].symbol, pairs[i].currency); err != nil {
			addErr = err
		}
		break
	}
	res := h.inner.Run(ctx, day, keys)
	if addErr != nil && res.Err == nil {
		res.Err = &listedError{kind: contracts.Retryable, msg: "add held names failed", err: addErr}
	}
	return res
}

// candidates reads the newest positions and the list and returns the held names to add, sorted by key, at most
// maxNames.
func (h *heldRowsJob) candidates(ctx context.Context) ([]heldPair, error) {
	rows, err := h.list.Current(ctx)
	if err != nil {
		return nil, err
	}
	have := map[string]bool{}
	for _, r := range rows {
		have[strings.ToUpper(r.Symbol)] = true
	}
	positions, err := h.positions(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []heldPair
	for _, p := range positions {
		if (p.SecType != heldSecStock && p.SecType != heldSecOption) || isZeroQuantity(p.Quantity) {
			continue
		}
		if p.SecType == heldSecOption && h.indexes[strings.ToUpper(p.Symbol)] {
			continue
		}
		if have[strings.ToUpper(p.Symbol)] {
			continue
		}
		key, err := h.adder.Check(p.Symbol, p.Currency)
		if err != nil || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, heldPair{symbol: p.Symbol, currency: p.Currency, key: key})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key < out[j].key })
	if len(out) > h.maxNames {
		out = out[:h.maxNames]
	}
	return out, nil
}

// positions is the newest positions of each alias of the held aliases, searched back heldLookbackDays from today.
func (h *heldRowsJob) positions(ctx context.Context) ([]store.Position, error) {
	now := h.clk.Now().In(h.zone)
	var all []store.Position
	for _, alias := range h.aliases {
		for back := 0; back <= heldLookbackDays; back++ {
			rows, err := h.account.Positions(ctx, alias, now.AddDate(0, 0, -back).Format(time.DateOnly))
			if err != nil {
				return nil, err
			}
			if len(rows) > 0 {
				all = append(all, rows...)
				break
			}
		}
	}
	return all, nil
}
