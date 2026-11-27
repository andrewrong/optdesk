package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/datacheck"
	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/store/coverage"
	"github.com/andrewrong/optdesk/internal/store/finding"
	"github.com/andrewrong/optdesk/internal/store/findingpg"
	"github.com/andrewrong/optdesk/internal/store/ops"
)

// Design: docs/design/data-check.md#11 data-check-wiring.

// jobDataCheck is the id of the data_check job.
const jobDataCheck = "data_check"

// reasonNoDataCheckDSN is why data_check is held while the line of its writer is empty.
const reasonNoDataCheckDSN = finding.DSNEnvDataCheck + " is not set (run setup pg)"

// dataCheckNotDaily is the fixed list of the jobs that store a ledger dataset but are not planned on every session
// day of their calendar, with the reason. plan_absent does not expect their datasets (Decision 6).
var dataCheckNotDaily = map[string]string{
	"calendar_refresh":        "it refreshes the calendar when the source changes, not on every session day",
	"calendar_hours_us":       "it stores a notice only on a day whose hours differ",
	"calendar_hours_jp":       "it stores a notice only on a day whose hours differ",
	"underlying_resolve":      "it runs when the underlying list changes",
	"option_open_resolve_gap": "it resolves only the contracts the main resolve left open",
}

// dataCheckExpected is the list of datasets that have a plan on every session day of their calendar: the ledger
// datasets of the jobs of jobTable that are not in noLedger and not in notDaily, once per (dataset, calendar), sorted.
func dataCheckExpected(rows []jobRow, manifests map[string]dataHealthManifest, noLedger, notDaily map[string]string) []datacheck.ExpectedDataset {
	seen := map[datacheck.ExpectedDataset]bool{}
	var out []datacheck.ExpectedDataset
	for _, r := range rows {
		if _, ok := noLedger[r.ID]; ok {
			continue
		}
		if _, ok := notDaily[r.ID]; ok {
			continue
		}
		m := manifests[r.ID]
		for _, ds := range m.datasets {
			e := datacheck.ExpectedDataset{Dataset: ds, Calendar: m.calendar}
			if !seen[e] {
				seen[e] = true
				out = append(out, e)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Dataset != out[j].Dataset {
			return out[i].Dataset < out[j].Dataset
		}
		return out[i].Calendar < out[j].Calendar
	})
	return out
}

// senderAnnouncer adapts a notify.Sender to a datacheck.Announcer.
type senderAnnouncer struct{ s notify.Sender }

func (a senderAnnouncer) Announce(ctx context.Context, text string) error { return a.s.Send(ctx, text) }

// dataCheckOpeners are what the job is built from; the family passes the real ones, a test passes fakes.
type dataCheckOpeners struct {
	writer finding.OpenWriterFunc
	probe  finding.OpenProbeFunc
	cov    coverage.OpenFunc
	sender func() (notify.Sender, error)
	mk     func(datacheck.Stores, clock.Clock) (contracts.Job, error)
}

func init() {
	registerJob(jobFamily{
		id: jobDataCheck, order: 240,
		startOff: func(cfg runOnceConfig, _ bool) string {
			if cfg.dataCheckDSN == "" {
				return reasonNoDataCheckDSN
			}
			return ""
		},
		build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			if jc.cfg.dataCheckDSN == "" {
				// The reason is the startOff of the family; the scheduler reports the job as disabled with it.
				return nil, "", nil
			}
			j, err := jc.f.dataCheck(ctx, jc, dataCheckOpeners{
				writer: findingpg.OpenWriter, probe: findingpg.OpenProbe, cov: ops.OpenCoverage,
				sender: func() (notify.Sender, error) { return telegramSender(a) }, mk: datacheck.New,
			})
			if err != nil {
				return nil, "", err
			}
			return j, "", nil
		},
	})
}

// dataCheck builds data_check over the writer, the probe and the coverage reader opened with the data check line,
// the expected list of jobTable and the Telegram sender as the announcer; a sender that cannot be built gives a nil
// announcer (the findings are stored and the page shows them).
func (f jobFactory) dataCheck(ctx context.Context, jc *jobCtx, o dataCheckOpeners) (contracts.Job, error) {
	dsn := jc.cfg.dataCheckDSN
	w, err := o.writer(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open data_check writer with %s failed", finding.DSNEnvDataCheck)
	}
	jc.extra.addCloser(w)
	p, err := o.probe(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open data_check probe with %s failed", finding.DSNEnvDataCheck)
	}
	jc.extra.addCloser(p)
	cov, err := o.cov(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open data_check coverage reader with %s failed", finding.DSNEnvDataCheck)
	}
	jc.extra.addCloser(cov)
	st := datacheck.Stores{
		Cov: cov, Probe: p, Out: w,
		Expected: dataCheckExpected(jobTable, dataHealthManifests, dataHealthNoLedger, dataCheckNotDaily),
	}
	if s, err := o.sender(); err == nil && s != nil {
		st.Announce = senderAnnouncer{s}
	}
	j, err := o.mk(st, f.clk)
	if err != nil {
		return nil, fmt.Errorf("build data_check job failed: %w", err)
	}
	return j, nil
}
