package main

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/optionopen"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

// Design: docs/design/option-open-pull.md#10: the plan contracts of the refresh button (a copy of the item-to-contract
// mapping of optionopen's pull, which does not export it).

// marketTargets builds the asked-able targets of the planned items of m whose underlying is one of syms.
func (l *liveQuotes) marketTargets(ctx context.Context, m liveMarket, day contracts.Day, syms []string) ([]optionopen.Target, error) {
	items, err := l.d.Plan.Plan(ctx, day, m.slot)
	if err != nil || len(items) == 0 {
		return nil, err
	}
	var unders []int64
	for _, it := range items {
		if id := int64(it.UnderlyingID); !slices.Contains(unders, id) {
			unders = append(unders, id)
		}
	}
	uis, err := l.d.Insts.Instruments(ctx, unders)
	if err != nil {
		return nil, err
	}
	want := map[int64]bool{}
	for _, in := range uis {
		if slices.Contains(syms, strings.ToUpper(in.Spec.Name)) {
			want[in.ID] = true
		}
	}
	var mine []contracts.OptionOpenItem
	ids := map[int64]bool{}
	for _, it := range items {
		if want[int64(it.UnderlyingID)] && !it.Deferred {
			mine = append(mine, it)
			ids[int64(it.InstrumentID)] = true
		}
	}
	for id := range want {
		ids[id] = true
	}
	conID, spec, err := l.conIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	refs := map[int64]map[int64]openquote.OptionRef{}
	var out []optionopen.Target
	for _, it := range mine {
		u, o := int64(it.UnderlyingID), int64(it.InstrumentID)
		if refs[u] == nil {
			list, err := l.d.Contracts.ListOptions(ctx, u)
			if err != nil {
				return nil, err
			}
			refs[u] = map[int64]openquote.OptionRef{}
			for _, r := range list {
				refs[u][r.InstrumentID] = r
			}
		}
		if t, ok := liveTarget(m, it, spec[u], refs[u][o], conID[u], conID[o]); ok {
			out = append(out, t)
		}
	}
	return out, nil
}

// conIDs reads the instruments of ids and returns their gateway contract ids and specs.
func (l *liveQuotes) conIDs(ctx context.Context, ids map[int64]bool) (map[int64]int64, map[int64]store.InstrumentSpec, error) {
	idList := make([]int64, 0, len(ids))
	for id := range ids {
		idList = append(idList, id)
	}
	insts, err := l.d.Insts.Instruments(ctx, idList)
	if err != nil {
		return nil, nil, err
	}
	conID, spec := map[int64]int64{}, map[int64]store.InstrumentSpec{}
	for _, in := range insts {
		spec[in.ID] = in.Spec
		for _, idf := range in.Identifiers {
			if n, err := strconv.ParseInt(idf.Value, 10, 64); err == nil && idf.Scheme == "ibkr_conid" {
				conID[in.ID] = n
			}
		}
	}
	return conID, spec, nil
}

// liveTarget builds the target of one item; false when a contract field is missing or malformed (nothing is asked).
func liveTarget(m liveMarket, it contracts.OptionOpenItem, us store.InstrumentSpec, ref openquote.OptionRef, uCon, oCon int64) (optionopen.Target, bool) {
	if uCon == 0 || oCon == 0 || ref.InstrumentID == 0 {
		return optionopen.Target{}, false
	}
	exp, err := time.Parse(time.DateOnly, string(ref.Expiry))
	if err != nil {
		return optionopen.Target{}, false
	}
	strike, err := strconv.ParseFloat(ref.Strike, 64)
	if err != nil {
		return optionopen.Target{}, false
	}
	ccy := m.currency
	if us.Currency != "" {
		ccy = us.Currency
	}
	return optionopen.Target{
		Item: it, Symbol: us.Name, Currency: ccy, Expiry: exp, Strike: strike, Right: optionopen.Right(ref.Right),
		Under: broker.Contract{SecType: "STK", Symbol: us.Name, Exchange: m.exchange, Currency: ccy, ConID: uCon},
		Option: broker.Contract{SecType: "OPT", Symbol: us.Name, Exchange: ref.Exchange, Currency: ref.Currency,
			Expiry: strings.ReplaceAll(string(ref.Expiry), "-", ""), Strike: ref.Strike, Right: ref.Right,
			Multiplier: ref.Multiplier, TradingClass: ref.TradingClass, ConID: oCon},
	}, true
}
