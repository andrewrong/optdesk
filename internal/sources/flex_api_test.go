package sources_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
)

type flexErr struct{ text string }

func (e flexErr) Error() string           { return e.text }
func (flexErr) Kind() contracts.ErrorKind { return contracts.Permanent }

type fakeFlexSource struct{ gotQuery string }

var _ sources.FlexSource = (*fakeFlexSource)(nil)

func (f *fakeFlexSource) Fetch(_ context.Context, r sources.FlexRequest) (sources.FlexReport, error) {
	f.gotQuery = r.QueryID
	if r.QueryID == "" {
		return sources.FlexReport{}, flexErr{"flex_error: empty query"}
	}
	return sources.FlexReport{
		FetchedAt: time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC),
		RawSHA256: "00",
		Statements: []sources.FlexStatement{{
			AccountID: "ACCT", FromDate: string(r.First), ToDate: string(r.Last),
			Trades:      []map[string]string{{"transactionID": "1"}},
			OtherLevels: 2,
		}},
	}, nil
}

func TestFlexSourceContract(t *testing.T) {
	t.Parallel()
	fake := &fakeFlexSource{}
	newFlex := sources.NewFlexFunc(func(cfg sources.FlexConfig) (sources.FlexSource, error) {
		if cfg.Token == "" {
			return nil, sources.ErrFlexNotConfigured
		}
		return fake, nil
	})
	wait := func(context.Context, time.Duration) error { return nil }
	if _, err := newFlex(sources.FlexConfig{}); !errors.Is(err, sources.ErrFlexNotConfigured) {
		t.Fatalf("empty token: %v", err)
	}
	src, err := newFlex(sources.FlexConfig{Token: "x", Wait: wait, Clock: clockwork.NewFakeClock()})
	if err != nil {
		t.Fatal(err)
	}
	rep, err := src.Fetch(context.Background(), sources.FlexRequest{QueryID: "123", First: "2026-11-01", Last: "2026-11-08"})
	if err != nil {
		t.Fatal(err)
	}
	if fake.gotQuery != "123" || len(rep.Statements) != 1 || rep.Statements[0].FromDate != "2026-11-01" ||
		rep.Statements[0].Trades[0]["transactionID"] != "1" || rep.Statements[0].OtherLevels != 2 {
		t.Fatalf("report: %+v", rep)
	}
	_, err = src.Fetch(context.Background(), sources.FlexRequest{})
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Permanent {
		t.Fatalf("want typed permanent error, got %v", err)
	}
}

func TestFlexFileLoaderContract(t *testing.T) {
	t.Parallel()
	load := sources.LoadFlexFileFunc(func(path string) (sources.FlexFile, error) {
		switch path {
		case "missing":
			return sources.FlexFile{}, sources.ErrFlexNotConfigured
		case "bad":
			return sources.FlexFile{}, fmt.Errorf("login 1: field token_env: %w", sources.ErrFlexConfig)
		}
		return sources.FlexFile{
			Logins:   []sources.FlexLogin{{Name: "main", TokenEnv: "TOK", TradeQueries: []string{"1"}}},
		}, nil
	})
	f, err := load("ok")
	if err != nil {
		t.Fatal(err)
	}
	if f.Logins[0].TokenEnv != "TOK" || f.Logins[0].TradeQueries[0] != "1" {
		t.Fatalf("file: %+v", f)
	}
	if _, err := load("missing"); !errors.Is(err, sources.ErrFlexNotConfigured) {
		t.Fatalf("missing: %v", err)
	}
	if _, err := load("bad"); !errors.Is(err, sources.ErrFlexConfig) || errors.Is(err, sources.ErrFlexNotConfigured) {
		t.Fatalf("bad: %v", err)
	}
}

func TestFlexFileRequiredLoginContract(t *testing.T) {
	t.Parallel()
	load := sources.LoadFlexFileFunc(func(string) (sources.FlexFile, error) {
		return sources.FlexFile{
			Logins: []sources.FlexLogin{
				{Name: "main", TokenEnv: "TOK_MAIN", TradeQueries: []string{"1"}, Required: true},
				{Name: "second", TokenEnv: "TOK_2", TradeQueries: []string{"2"}},
			},
		}, nil
	})
	f, err := load("ok")
	if err != nil {
		t.Fatal(err)
	}
	if !f.Logins[0].Required || f.Logins[1].Required {
		t.Fatalf("required flags: %+v", f.Logins)
	}
}
