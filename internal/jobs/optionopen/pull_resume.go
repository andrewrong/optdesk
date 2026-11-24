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

// resume reads the rows stored for the batch for the session day of its items and the slot of the manifest and splits
// the items into the final rows in hand (in the order of the items) and the items still to ask. The final rows of
// added contracts (the stock's stored contracts that are not items of the plan: only a live band that kept them wrote a
// row for them) follow the rows of the items, so an added contract is final or asked again like a planned one.
func (p *pull) resume(ctx context.Context, items []contracts.OptionOpenItem, chain ContractLister) (
	final []contracts.OptionOpen, ask []contracts.OptionOpenItem, err error) {
	if len(items) == 0 {
		return nil, nil, nil
	}
	ids := make([]int64, len(items))
	inPlan := make(map[int64]bool, len(items))
	for i, it := range items {
		ids[i] = int64(it.InstrumentID)
		inPlan[ids[i]] = true
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
			return nil, nil, err
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
		return nil, nil, err
	}
	have := make(map[int]contracts.OptionOpen, len(stored))
	for _, r := range stored {
		if r.Slot == p.m.Slot && isFinal(r) {
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
	return final, ask, nil
}
