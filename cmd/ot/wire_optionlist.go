package main

import (
	"context"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/optionopen"
	"github.com/andrewrong/optdesk/internal/store/openquote"
	"github.com/andrewrong/optdesk/internal/store/runview"
)

// Design: docs/design/option-contract-lists.md#6 list-ot-wire (ot); docs/design/option-open-gap-jp.md#Same-morning-visibility
// (entry 8: the three list jobs get their ports).

// The ids of the list jobs and the reason that holds the US one when the broker cannot list contracts or the store
// cannot hold lists. The zone slot (US, before the plan) and the JP list are the same job code over two more
// manifests.
const (
	jobOptionListPreopen   = "option_list_preopen"
	jobOptionListZone      = "option_list_zone"
	jobOptionListPreopenJP = "option_list_preopen_jp"
	reasonListAbsent       = "list_absent"
)

// listKind is what tells the three list jobs apart: the id, the constructor over ListDeps and whether a store
// without the list port leaves the job out with a not_wired row (the zone and JP jobs) or holds it (the US job).
type listKind struct {
	id       string
	newJob   func(optionopen.ListDeps) (contracts.Job, error)
	leaveOut bool
}

var (
	listUS   = listKind{id: jobOptionListPreopen, newJob: optionopen.NewListPreopen}
	listZone = listKind{id: jobOptionListZone, newJob: optionopen.NewListZone, leaveOut: true}
	listJP   = listKind{id: jobOptionListPreopenJP, newJob: optionopen.NewListPreopenJP, leaveOut: true}
)

func init() {
	// Orders 185, 186 and 187 place the jobs before the gap pass (190) in the registration list only; the keys of a
	// turn run by DatasetID. The wiring adds no time rule: the window is the manifest's.
	registerJob(jobFamily{
		id: jobOptionListPreopen, order: 185, startOff: ifOpenDSN(brokerDownOff),
		build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			return buildOptionList(ctx, a, jc, listUS)
		},
	})
	registerJob(jobFamily{
		id: jobOptionListZone, order: 186, startOff: ifOpenDSN(brokerDownOff),
		build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			return buildOptionList(ctx, a, jc, listZone)
		},
	})
	registerJob(jobFamily{
		id: jobOptionListPreopenJP, order: 187, startOff: ifOpenDSN(brokerDownOff),
		build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			return buildOptionList(ctx, a, jc, listJP)
		},
	})
}

// buildOptionList opens the store, dials a client of its own with one resolve in flight (as the resolve jobs do:
// a listing is one request and the gateway must not see two at once) and builds the job over it.
func buildOptionList(ctx context.Context, a *app, jc *jobCtx, k listKind) (contracts.Job, string, error) {
	stores, reason, ok := openOptionOpen(ctx, a, jc, k.id, true)
	if !ok {
		return nil, "", nil
	}
	own := *jc
	if jc.brokerUp {
		c, err := a.deps.dialBroker(ctx, jc.cfg.socketPath, broker.Options{ResolveInFlight: 1})
		if err != nil {
			reason = firstReason(reason, reasonBrokerDown)
		} else {
			jc.extra.add(c)
			own.brk = c
		}
	}
	return listJobOf(a, &own, stores, reason, k)
}

// optionListJob builds the US job over the opened store.
func optionListJob(a *app, jc *jobCtx, stores openquote.Stores, reason string) (contracts.Job, string, error) {
	return listJobOf(a, jc, stores, reason, listUS)
}

// listPorts are the ports every list job gets: the spot of the zone check (a Snapshotter over the broker), the
// history of a missing close and the reason writer, so that a nil port never silently turns off the gateway ask,
// the zone check or the reason rows.
type listPorts struct {
	Spot    optionopen.Snapshotter
	History broker.HistoryClient
	Reasons runview.ReasonWriter
}

func listPortsOf(jc *jobCtx, id string) (listPorts, error) {
	rw, err := jc.reasonsFor(id)
	if err != nil {
		return listPorts{}, err
	}
	return listPorts{Spot: jc.brk, History: historyOf(jc.brk), Reasons: rw}, nil
}

// listJobOf builds the job k over the opened store. A broker without ContractLister or a store without
// OptionListStore holds the US job with list_absent (one startup line); a stand-in that answers a Permanent error
// takes its place, so the key ends Permanent for the day should the job ever run. The zone and the JP job are
// left out instead (when the store opened), with one startup line and a not_wired row naming the key of the store.
func listJobOf(a *app, jc *jobCtx, stores openquote.Stores, reason string, k listKind) (contracts.Job, string, error) {
	ports, err := listPortsOf(jc, k.id)
	if err != nil {
		return nil, "", err
	}
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
		if k.leaveOut && reason == "" {
			a.log.Warn("option list job not wired", "component", "ot", "job", k.id, "status", reasonListAbsent)
			jc.notWired.record(k.id, openquote.DSNEnvOptionOpen)
			return nil, "", nil
		}
		ls = absentListStore{}
		reason = firstReason(reason, reasonListAbsent)
	}
	f := *jc.f
	j, err := f.listed(groupOptions, func(entries []jobs.WatchEntry) (contracts.Job, error) {
		return k.newJob(optionopen.ListDeps{
			Stored:       openStored(jc, stores, f.fixedEntries(entries)),
			Lister:       lister,
			ListStore:    ls,
			Clock:        jc.f.clk,
			Spot:         ports.Spot,
			History:      ports.History,
			ReasonWriter: ports.Reasons,
			Log:          func(line string) { a.log.Info(line, "component", "ot", "job", k.id) },
		})
	})
	return held(a, k.id, j, reason, err)
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
