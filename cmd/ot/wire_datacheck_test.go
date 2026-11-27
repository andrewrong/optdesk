package main

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/datacheck"
	"github.com/andrewrong/optdesk/internal/jobs/optioncandidates"
	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/store/coverage"
	"github.com/andrewrong/optdesk/internal/store/finding"
)

type fakeFindingWriter struct{ finding.Writer }

func (fakeFindingWriter) Close() error { return nil }

type fakeFindingProbe struct{ finding.Probe }

func (fakeFindingProbe) Close() error { return nil }

type fakeDataCheckSender struct{}

func (fakeDataCheckSender) Send(context.Context, string) error { return nil }

func TestDataCheckHeldWithoutDSN(t *testing.T) {
	t.Parallel()
	const want = "OT_PG_DSN_DATACHECK is not set (run setup pg)"
	if got := startDisabled(runOnceConfig{}, true)["data_check"]; got != want {
		t.Fatalf("reason %q, want %q", got, want)
	}
	if got, held := startDisabled(runOnceConfig{dataCheckDSN: "dc"}, true)["data_check"]; held {
		t.Fatalf("held with the line set: %q", got)
	}
	fam, ok := jobFamilies["data_check"]
	if !ok || fam.order != 240 {
		t.Fatalf("family %+v", fam)
	}
	jc := &jobCtx{cfg: runOnceConfig{}, f: &jobFactory{}, extra: &closerSet{}}
	if j, reason, err := fam.build(t.Context(), nil, jc); j != nil || reason != "" || err != nil {
		t.Fatalf("build without the line: %v %q %v", j, reason, err)
	}
}

func dataCheckOpenersFor(dsn *string, sender func() (notify.Sender, error), got *datacheck.Stores) dataCheckOpeners {
	return dataCheckOpeners{
		writer: func(_ context.Context, d string) (finding.Writer, error) { *dsn = d; return fakeFindingWriter{}, nil },
		probe:  func(context.Context, string) (finding.Probe, error) { return fakeFindingProbe{}, nil },
		cov:    func(context.Context, string) (coverage.Reader, error) { return fakeCoverage{}, nil },
		sender: sender,
		mk: func(st datacheck.Stores, _ clock.Clock) (contracts.Job, error) {
			*got = st
			return nil, nil
		},
	}
}

func TestNilAnnouncerWithoutToken(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		sender  func() (notify.Sender, error)
		wantAnn bool
	}{
		"no token": {func() (notify.Sender, error) { return nil, errors.New("OT_TELEGRAM_TOKEN is missing in .env") }, false},
		"token":    {func() (notify.Sender, error) { return fakeDataCheckSender{}, nil }, true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var dsn string
			var st datacheck.Stores
			jc := &jobCtx{cfg: runOnceConfig{dataCheckDSN: "dc-dsn"}, extra: &closerSet{}}
			if _, err := (jobFactory{}).dataCheck(t.Context(), jc, dataCheckOpenersFor(&dsn, tc.sender, &st)); err != nil {
				t.Fatal(err)
			}
			if dsn != "dc-dsn" || st.Cov == nil || st.Probe == nil || st.Out == nil || len(st.Expected) == 0 {
				t.Fatalf("stores %+v dsn %q", st, dsn)
			}
			if (st.Announce != nil) != tc.wantAnn {
				t.Fatalf("announcer %v, want present %v", st.Announce, tc.wantAnn)
			}
		})
	}
}

func TestEveryLedgerJobIsDailyOrReasoned(t *testing.T) {
	t.Parallel()
	expected := dataCheckExpected(jobTable, dataHealthManifests, dataHealthNoLedger, dataCheckNotDaily)
	for id, reason := range dataCheckNotDaily {
		if reason == "" {
			t.Errorf("%s: not-daily entry has no reason", id)
		}
		if _, ok := jobRowOf(id); !ok {
			t.Errorf("not-daily list names %s, which is not in jobTable", id)
		}
		if _, ok := dataHealthNoLedger[id]; ok {
			t.Errorf("%s is in both fixed lists", id)
		}
	}
	for _, r := range jobTable {
		m, ledger := dataHealthManifests[r.ID]
		if !ledger || len(m.datasets) == 0 {
			continue
		}
		_, notDaily := dataCheckNotDaily[r.ID]
		for _, ds := range m.datasets {
			in := slices.Contains(expected, datacheck.ExpectedDataset{Dataset: ds, Calendar: m.calendar})
			if !in && !notDaily {
				t.Errorf("%s (%s) is neither expected nor in dataCheckNotDaily", r.ID, ds)
			}
		}
	}
	// a job outside both lists is expected; one in dataCheckNotDaily or without ledger is not.
	rows := []jobRow{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	man := map[string]dataHealthManifest{"a": {"XNYS", []string{"x.a"}, ""}, "b": {"XNYS", []string{"x.b"}, ""}}
	got := dataCheckExpected(rows, man, map[string]string{"c": "r"}, map[string]string{"b": "r"})
	if !slices.Equal(got, []datacheck.ExpectedDataset{{Dataset: "x.a", Calendar: "XNYS"}}) {
		t.Fatalf("expected %v", got)
	}
}

func TestDataCheckRuleVersionsWired(t *testing.T) {
	t.Parallel()
	var dsn string
	var st datacheck.Stores
	jc := &jobCtx{cfg: runOnceConfig{dataCheckDSN: "dc-dsn"}, extra: &closerSet{}}
	sender := func() (notify.Sender, error) { return fakeDataCheckSender{}, nil }
	if _, err := (jobFactory{}).dataCheck(t.Context(), jc, dataCheckOpenersFor(&dsn, sender, &st)); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"candidates.option_candidate": optioncandidates.RuleVersion}
	if len(st.RuleVersions) != 1 || st.RuleVersions["candidates.option_candidate"] != want["candidates.option_candidate"] {
		t.Fatalf("RuleVersions %v, want %v", st.RuleVersions, want)
	}
}
