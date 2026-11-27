package optionopen

import (
	"context"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

// slotReader is the reader of the stored rows: PullDeps.Reader, else the Writer when it also reads; nil when neither does.
func slotReader(d PullDeps) openquote.OptionOpenSlotReader {
	if d.Reader != nil {
		return d.Reader
	}
	r, _ := d.Writer.(openquote.OptionOpenSlotReader)
	return r
}

// isFinal is a final stored row (Decision 8): ok, not_listed and no_data with the code NO_QUOTE are never asked again
// for their day and slot; every other status, no_subscription and no_data with no code among them, is asked again.
func isFinal(r contracts.OptionOpen) bool {
	return r.Status == stOK || r.Status == stNotListed || isNoQuoteRow(r)
}

// storedCounts is how many ok and partial rows the store held for a batch (or a run) at a moment.
type storedCounts struct{ ok, partial int }

// storedRows is what resume read of the store before an attempt: the counts of ok and partial rows and the instruments
// whose stored row is partial.
type storedRows struct {
	counts  storedCounts
	partial map[int]bool
}

// flushFunc writes rows of the store.
type flushFunc = func(context.Context, []contracts.OptionOpen) error

// keepStoredPartial is a Flush that does not write an answer that is partial again for an instrument whose stored row
// is partial: the stored row stays as written, so an attempt that ends Permanent for an unchanged result rewrites none.
func keepStoredPartial(stored map[int]bool, write flushFunc) flushFunc {
	return func(ctx context.Context, rows []contracts.OptionOpen) error {
		keep := make([]contracts.OptionOpen, 0, len(rows))
		for _, r := range rows {
			if r.Status == stPartial && stored[r.InstrumentID] {
				continue
			}
			keep = append(keep, r)
		}
		if len(keep) == 0 {
			return nil
		}
		return write(ctx, keep)
	}
}

func (c *storedCounts) add(o storedCounts) { c.ok += o.ok; c.partial += o.partial }

// countOKPartial counts the ok and partial rows among rows.
func countOKPartial(rows []contracts.OptionOpen) (c storedCounts) {
	for _, r := range rows {
		switch r.Status {
		case stOK:
			c.ok++
		case stPartial:
			c.partial++
		}
	}
	return c
}

// resume reads the rows stored for the batch for the session day of its items and the slot of the manifest and splits
// the items into the final rows in hand (in the order of the items) and the items still to ask. The final rows of
// added contracts (the stock's stored contracts that are not items of the plan: only a live band that kept them wrote a
// row for them) follow the rows of the items, so an added contract is final or asked again like a planned one.
func (p *pull) resume(ctx context.Context, items []contracts.OptionOpenItem, chain ContractLister) (
	final []contracts.OptionOpen, ask []contracts.OptionOpenItem, prior storedRows, err error) {
	if len(items) == 0 {
		return nil, nil, prior, nil
	}
	ids := make([]int64, 0, len(items))
	inPlan := make(map[int64]bool, len(items))
	for _, it := range items {
		id := int64(it.InstrumentID)
		ids = append(ids, id)
		inPlan[id] = true
	}
	seen := map[int64]bool{}
	for _, it := range items {
		u := int64(it.UnderlyingID)
		if seen[u] {
			continue
		}
		seen[u] = true
		refs, err := chain.ListOptions(ctx, u)
		if err != nil {
			return nil, nil, prior, err
		}
		for _, r := range refs {
			if !inPlan[r.InstrumentID] {
				inPlan[r.InstrumentID] = true
				ids = append(ids, r.InstrumentID)
			}
		}
	}
	stored, err := p.reader.OptionOpenSlot(ctx, contracts.Day(items[0].SessionDay.Format("2006-01-02")), p.m.Slot, ids)
	if err != nil {
		return nil, nil, prior, err
	}
	have := make(map[int]contracts.OptionOpen, len(stored))
	for _, r := range stored {
		if r.Slot != p.m.Slot {
			continue
		}
		prior.counts.add(countOKPartial([]contracts.OptionOpen{r}))
		if r.Status == stPartial {
			if prior.partial == nil {
				prior.partial = map[int]bool{}
			}
			prior.partial[r.InstrumentID] = true
		}
		if isFinal(r) {
			have[r.InstrumentID] = r
		}
	}
	for _, it := range items {
		if r, ok := have[it.InstrumentID]; ok {
			final = append(final, r)
			delete(have, it.InstrumentID)
		} else {
			ask = append(ask, it)
		}
	}
	for _, id := range ids[len(items):] {
		if r, ok := have[int(id)]; ok {
			final = append(final, r)
		}
	}
	return final, ask, prior, nil
}
