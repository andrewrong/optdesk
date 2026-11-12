package main

import (
	"context"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/optionopen"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

// Design: docs/design/option-contract-lists.md#6 list-ot-wire (ot).

// jobOptionListPreopen is the id of the contract list job; reasonListAbsent holds it when the broker cannot list
// contracts or the store cannot hold lists.
const (
	jobOptionListPreopen = "option_list_preopen"
	reasonListAbsent     = "list_absent"
)

func init() {
	// Order 185 places the job before the gap pass (190) in the registration list only; the keys of a turn run by
	// DatasetID. The wiring adds no time rule: the window is the manifest's.
	registerJob(jobFamily{
		id: jobOptionListPreopen, order: 185, startOff: ifOpenDSN(brokerDownOff),
		build: buildOptionList,
	})
}

func buildOptionList(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
	stores, reason, ok := openOptionOpen(ctx, a, jc, jobOptionListPreopen, true)
	if !ok {
		return nil, "", nil
	}
	return optionListJob(a, jc, stores, reason)
}

// optionListJob builds the job over the opened store. A broker without ContractLister or a store without
// OptionListStore holds the job with list_absent (one startup line); a stand-in that answers a Permanent error
// takes its place, so the key ends Permanent for the day should the job ever run.
func optionListJob(a *app, jc *jobCtx, stores openquote.Stores, reason string) (contracts.Job, string, error) {
	lister, lok := jc.brk.(broker.ContractLister)
	if !lok {
		lister = absentLister{}
		// With the socket down the broker is a stand-in: the reason is the dead socket, not the missing call.
		if jc.brokerUp {
			reason = firstReason(reason, reasonListAbsent)
		} else {
			reason = firstReason(reason, reasonBrokerDown)
		}
	}
	ls, sok := any(stores).(openquote.OptionListStore)
	if !sok {
		ls = absentListStore{}
		reason = firstReason(reason, reasonListAbsent)
	}
	f := *jc.f
	j, err := f.listed(groupOptions, func(entries []jobs.WatchEntry) (contracts.Job, error) {
		return optionopen.NewListPreopen(optionopen.ListDeps{
			Stored:    openStored(jc, stores, f.fixedEntries(entries)),
			Lister:    lister,
			ListStore: ls,
			Clock:     jc.f.clk,
			Log:       func(line string) { a.log.Info(line, "component", "ot", "job", jobOptionListPreopen) },
		})
	})
	return held(a, jobOptionListPreopen, j, reason, err)
}

// permanentErr is the error of a stand-in: its kind is Permanent.
type permanentErr struct{}

func (permanentErr) Error() string             { return reasonListAbsent }
func (permanentErr) Kind() contracts.ErrorKind { return contracts.Permanent }

// absentLister stands in for a broker that cannot list contracts.
type absentLister struct{}

func (absentLister) ListContracts(context.Context, broker.Contract, contracts.Day, string, string) ([]broker.Contract, error) {
	return nil, permanentErr{}
}

func (absentLister) ListChains(context.Context, broker.Contract) ([]broker.Chain, error) {
	return nil, permanentErr{}
}

// absentListStore stands in for a store that cannot hold lists.
type absentListStore struct{}

func (absentListStore) PutLists(context.Context, []openquote.ListRecord) error { return permanentErr{} }
func (absentListStore) Lists(context.Context, int64) ([]openquote.ListRecord, error) {
	return nil, permanentErr{}
}
func (absentListStore) PutListRuns(context.Context, []openquote.ListRunRecord) error {
	return permanentErr{}
}
func (absentListStore) ListRuns(context.Context, contracts.Day) ([]openquote.ListRunRecord, error) {
	return nil, permanentErr{}
}
