package option_data_page

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
	"github.com/andrewrong/optdesk/internal/store"
)

type fakeEarnings struct {
	rows []contracts.EarningsDate
	err  error
	mu   sync.Mutex
	got  []string
}

func (f *fakeEarnings) EarningsDates(_ context.Context, issuer string, from, to contracts.Day) ([]contracts.EarningsDate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.got = append(f.got, issuer+" "+string(from)+" "+string(to))
	return f.rows, f.err
}

func earnRow(t *testing.T, status, src string, report string) contracts.EarningsDate {
	t.Helper()
	r := contracts.EarningsDate{}
	r.Status, r.Source = gen.EarningsDateStatus(status), gen.EarningsDateSource(src)
	if report != "" {
		d := sday(t, report)
		r.ReportDate = &d
	}
	return r
}

// earningsCfg has one option expiring 2026-11-20 and the session day 2026-11-06.
func earningsCfg(t *testing.T, er store.EarningsDateReader) Config {
	t.Helper()
	c := cfg(&fakeEod{rows: []contracts.OptionEod{eodRow(t, 1, f64(5), "2026-11-06T21:00:00Z")}}, &fakeQuotes{})
	c.Terms = &fakeTerms{rows: []store.OptionTerms{{InstrumentID: 1, Right: "C", Strike: "100", ExpirationDate: "2026-11-20"}}}
	c.Earnings, c.Issuer = er, "cik:0000000001"
	return c
}

func buildChecked(t *testing.T, c Config) map[string]any {
	t.Helper()
	b, err := Build(t.Context(), c, "2026-11-06", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateBytes(t, b); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func under(m map[string]any) map[string]any { return m["underlying"].(map[string]any) }

func TestEarningsListedInTerm(t *testing.T) {
	t.Parallel()
	fe := &fakeEarnings{rows: []contracts.EarningsDate{
		earnRow(t, "absent", "none", ""),
		earnRow(t, "listed", "alphavantage", "2026-11-20"),
	}}
	u := under(buildChecked(t, earningsCfg(t, fe)))
	if u["earnings_text"] != "E 11-20 · source vendor" || u["earnings_tone"] != "in_term" {
		t.Fatalf("underlying = %v", u)
	}
	if len(fe.got) != 1 || fe.got[0] != "cik:0000000001 2026-07-09 2026-11-06" {
		t.Fatalf("read = %v", fe.got)
	}
	fe.rows = []contracts.EarningsDate{earnRow(t, "listed", "owner", "2026-11-06")}
	u = under(buildChecked(t, earningsCfg(t, fe)))
	if u["earnings_text"] != "E 11-06 · source owner" || u["earnings_tone"] != "in_term" {
		t.Fatalf("owner underlying = %v", u)
	}
	fe.rows = []contracts.EarningsDate{earnRow(t, "listed", "jquants", "2026-11-20")}
	if u = under(buildChecked(t, earningsCfg(t, fe))); u["earnings_text"] != "E 11-20 · source vendor" {
		t.Fatalf("jquants underlying = %v", u)
	}
}

func TestEarningsListedOutside(t *testing.T) {
	t.Parallel()
	fe := &fakeEarnings{rows: []contracts.EarningsDate{earnRow(t, "listed", "jquants", "2026-11-21")}}
	u := under(buildChecked(t, earningsCfg(t, fe)))
	if u["earnings_text"] != "E 11-21 · source vendor" || u["earnings_tone"] != "outside" {
		t.Fatalf("underlying = %v", u)
	}
}

func TestEarningsNoneCases(t *testing.T) {
	t.Parallel()
	cases := map[string][]contracts.EarningsDate{
		"no row":        nil,
		"absent":        {earnRow(t, "absent", "alphavantage", "")},
		"unsupported":   {earnRow(t, "unsupported", "none", "")},
		"error":         {earnRow(t, "error", "alphavantage", "")},
		"before day":    {earnRow(t, "listed", "alphavantage", "2026-11-05")},
		"listed no day": {earnRow(t, "listed", "alphavantage", "")},
	}
	for name, rows := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			u := under(buildChecked(t, earningsCfg(t, &fakeEarnings{rows: rows})))
			if _, ok := u["earnings_text"]; ok || u["earnings_tone"] != "none" {
				t.Fatalf("underlying = %v", u)
			}
		})
	}
}

func TestEarningsOmittedWithoutReader(t *testing.T) {
	t.Parallel()
	base := earningsCfg(t, nil)
	base.Issuer = ""
	withIssuer := earningsCfg(t, nil)
	a, err := Build(t.Context(), base, "2026-11-06", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Build(t.Context(), withIssuer, "2026-11-06", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range [][]byte{a, b} {
		if err := validateBytes(t, page); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(a, b) || bytes.Contains(a, []byte("earnings_")) {
		t.Fatalf("outputs differ or carry earnings:\n%s\n%s", a, b)
	}
	// A reader with an empty issuer is not read either.
	fe := &fakeEarnings{}
	c := earningsCfg(t, fe)
	c.Issuer = ""
	out, err := Build(t.Context(), c, "2026-11-06", fixedAt)
	if err != nil || !bytes.Equal(out, a) || len(fe.got) != 0 {
		t.Fatalf("empty issuer: err %v, reads %v", err, fe.got)
	}
	if err := validateBytes(t, out); err != nil {
		t.Fatal(err)
	}
	// Nothing is read when the eod read holds no rows.
	c = earningsCfg(t, fe)
	c.Eod = &fakeEod{}
	empty, err := Build(t.Context(), c, "2026-11-06", fixedAt)
	if err != nil || len(fe.got) != 0 {
		t.Fatalf("empty eod: err %v, reads %v", err, fe.got)
	}
	if err := validateBytes(t, empty); err != nil {
		t.Fatal(err)
	}
}

func TestEarningsReaderError(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	b, err := Build(t.Context(), earningsCfg(t, &fakeEarnings{err: boom}), "2026-11-06", fixedAt)
	if !errors.Is(err, boom) || b != nil {
		t.Fatalf("got %s, %v", b, err)
	}
}
