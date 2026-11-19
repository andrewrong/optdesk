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

// isFinal is a final status of a stored row (Decision 8): ok and not_listed are never asked again for their day and
// slot; every other status, no_subscription among them, is asked again.
func isFinal(status int) bool { return status == stOK || status == stNotListed }

// resume reads the rows stored for the batch for the session day of its items and the slot of the manifest and splits
// the items into the final rows in hand (in the order of the items) and the items still to ask.
func (p *pull) resume(ctx context.Context, items []contracts.OptionOpenItem) (final []contracts.OptionOpen, ask []contracts.OptionOpenItem, err error) {
	if len(items) == 0 {
		return nil, nil, nil
	}
	ids := make([]int64, len(items))
	for i, it := range items {
		ids[i] = int64(it.InstrumentID)
	}
	stored, err := p.reader.OptionOpenSlot(ctx, contracts.Day(items[0].SessionDay.Format("2006-01-02")), p.m.Slot, ids)
	if err != nil {
		return nil, nil, err
	}
	have := make(map[int]contracts.OptionOpen, len(stored))
	for _, r := range stored {
		if r.Slot == p.m.Slot && isFinal(r.Status) {
			have[r.InstrumentID] = r
		}
	}
	for _, it := range items {
		if r, ok := have[it.InstrumentID]; ok {
			final = append(final, r)
		} else {
			ask = append(ask, it)
		}
	}
	return final, ask, nil
}
