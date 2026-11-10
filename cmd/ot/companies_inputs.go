package main

import (
	"context"
	"math/big"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/valuation.md#valuation-wire-us (Decision 13).

// valuationFuncs are the seven compute functions of valuationapi, the real ones from internal/compute/valuation in
// cmd/ot and fakes in a test. One StandardOf serves the figures job and the page.
type valuationFuncs struct {
	StandardOf    valuationapi.StandardOfFunc
	BuildFilings  valuationapi.BuildFilingsFunc
	Band          valuationapi.BandFunc
	ImpliedGrowth valuationapi.ImpliedGrowthFunc
	ReturnAtPrice valuationapi.ReturnAtPriceFunc
	Questions     valuationapi.QuestionsFunc
	Diagnosis     valuationapi.DiagnosisFunc
}

// companiesInputs are the readers of companies_page that entry 24 leaves nil. A nil reader leaves its field of
// the page Config unset, which the page shows as a gap.
type companiesInputs struct {
	Facts   store.FactReader
	Series  store.SeriesReader
	Catalog store.OptionEodCatalog
	Names   store.InstrumentNameReader
	Eod     store.OptionEodReader
	Terms   store.OptionTermsReader
	Quotes  store.QuoteSnapshotReader
}

// quoteLookbackDays is how far before the page day a quote or an option_eod row is looked for.
const quoteLookbackDays = 7

// nameBatch is the most ids one InstrumentNames call takes.
const nameBatch = 500

// symbolIDs finds the instrument id of an underlying symbol: the catalog lists the ids with option_eod rows, the
// name reader names them, the id whose name equals the symbol is kept (a stock before any other type). The
// answer is read once per resolver.
type symbolIDs struct {
	catalog store.OptionEodCatalog
	names   store.InstrumentNameReader
	once    sync.Once
	ids     map[string]int64
	err     error
}

func (r *symbolIDs) load(ctx context.Context) {
	r.ids = map[string]int64{}
	list, err := r.catalog.OptionEodLatest(ctx, "")
	if err != nil {
		r.err = err
		return
	}
	isStock := map[string]bool{}
	underlying := make([]int64, 0, len(list))
	for _, e := range list {
		underlying = append(underlying, e.UnderlyingID)
	}
	r.err = inBatches(underlying, func(ids []int64) error {
		got, err := r.names.InstrumentNames(ctx, ids)
		for _, n := range got {
			stock := n.Type == "stock"
			if _, seen := r.ids[n.Name]; seen && (isStock[n.Name] || !stock) {
				continue
			}
			r.ids[n.Name], isStock[n.Name] = n.InstrumentID, stock
		}
		return err
	})
}

// inBatches calls fn with consecutive slices of items of at most nameBatch, and stops at the first error.
func inBatches[T any](items []T, fn func([]T) error) error {
	for i := 0; i < len(items); i += nameBatch {
		if err := fn(items[i:min(i+nameBatch, len(items))]); err != nil {
			return err
		}
	}
	return nil
}

// ID returns the instrument id of symbol; ok is false for a symbol with no match.
func (r *symbolIDs) ID(ctx context.Context, symbol string) (id int64, ok bool, err error) {
	r.once.Do(func() { r.load(ctx) })
	if r.err != nil {
		return 0, false, r.err
	}
	id, ok = r.ids[symbol]
	return id, ok, nil
}

func dayBefore(day contracts.Day, n int) (contracts.Day, error) {
	t, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return "", err
	}
	return contracts.Day(t.AddDate(0, 0, -n).Format(dayLayout)), nil
}

// spotOf returns the Spot function: the newest last price in the week up to the day, as decimal text. A symbol
// with no instrument or no price gives ok false, a gap.
func spotOf(res *symbolIDs, quotes store.QuoteSnapshotReader) func(context.Context, string, contracts.Day) (string, bool, error) {
	return func(ctx context.Context, symbol string, day contracts.Day) (string, bool, error) {
		id, ok, err := res.ID(ctx, symbol)
		if err != nil || !ok {
			return "", false, err
		}
		from, err := dayBefore(day, quoteLookbackDays)
		if err != nil {
			return "", false, err
		}
		rows, err := quotes.QuoteSnapshots(ctx, id, from, day)
		if err != nil {
			return "", false, err
		}
		for i := len(rows) - 1; i >= 0; i-- {
			if rows[i].Last != nil {
				return strconv.FormatFloat(*rows[i].Last, 'f', -1, 64), true, nil
			}
		}
		return "", false, nil
	}
}

// offersOf returns the Offers function: the strikes of the puts of the latest option_eod session in the week up
// to the day that have not expired, ascending. The option_eod row has no strike; the terms reader gives it.
func offersOf(res *symbolIDs, eod store.OptionEodReader, terms store.OptionTermsReader) func(context.Context, string, contracts.Day) ([]string, error) {
	return func(ctx context.Context, symbol string, day contracts.Day) ([]string, error) {
		id, ok, err := res.ID(ctx, symbol)
		if err != nil || !ok {
			return nil, err
		}
		from, err := dayBefore(day, quoteLookbackDays)
		if err != nil {
			return nil, err
		}
		rows, err := eod.OptionEod(ctx, id, from, day)
		if err != nil {
			return nil, err
		}
		var last string
		for _, r := range rows {
			if d := r.SessionDay.Format(dayLayout); d > last {
				last = d
			}
		}
		seen := map[int64]bool{}
		var optIDs []int64
		for _, r := range rows {
			if r.SessionDay.Format(dayLayout) == last && !seen[int64(r.InstrumentID)] {
				seen[int64(r.InstrumentID)] = true
				optIDs = append(optIDs, int64(r.InstrumentID))
			}
		}
		// A strike stays the text the terms give: it is deduplicated as text and ordered as a decimal.
		have := map[string]*big.Rat{}
		err = inBatches(optIDs, func(ids []int64) error {
			got, err := terms.OptionTerms(ctx, ids)
			for _, t := range got {
				k, ok := new(big.Rat).SetString(t.Strike)
				if t.Right == "P" && ok && string(t.ExpirationDate) > string(day) {
					have[t.Strike] = k
				}
			}
			return err
		})
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(have))
		for k := range have {
			out = append(out, k)
		}
		sort.Slice(out, func(i, j int) bool {
			if c := have[out[i]].Cmp(have[out[j]]); c != 0 {
				return c < 0
			}
			return out[i] < out[j]
		})
		return out, nil
	}
}
