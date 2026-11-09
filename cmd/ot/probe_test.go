package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/profile"
)

// fakeReference answers each contract through answer; asked records the symbols of the one call.
type fakeReference struct {
	answer func(c broker.Contract) broker.DividendProjection
	asked  [][]string
}

func (f *fakeReference) Dividends(_ context.Context, cs []broker.Contract) ([]broker.DividendProjection, error) {
	var syms []string
	out := make([]broker.DividendProjection, 0, len(cs))
	for _, c := range cs {
		syms = append(syms, c.Symbol)
		p := f.answer(c)
		p.Contract = c
		out = append(out, p)
	}
	f.asked = append(f.asked, syms)
	return out, nil
}

func (f *fakeReference) Close() error { return nil }

func okAnswer(broker.Contract) broker.DividendProjection {
	d := "20261112"
	return broker.DividendProjection{Status: broker.DividendOK, NextExDate: &d}
}

// runProbe runs `ot probe dividends args` with the fake and returns the output and the exit code.
func runProbe(t *testing.T, f *fakeReference, args ...string) (string, int) {
	t.Helper()
	var out bytes.Buffer
	a := &app{
		profile: profile.Profile{Name: "real"},
		stdout:  &out,
		getenv: func(k string) string {
			return map[string]string{"OT_HOME": "/h", "SECRET_DSN": "postgres://u:pw@h/db"}[k]
		},
		deps: deps{newReference: func(context.Context, string, broker.Options) (broker.ReferenceClient, error) {
			return f, nil
		}},
	}
	err := runProbeCommand(a, append([]string{"dividends"}, args...))
	code := 0
	if err != nil {
		code = 1
		var ee *exitError
		if errors.As(err, &ee) {
			code = ee.code
		}
	}
	return out.String(), code
}

func TestProbeDividendsPass(t *testing.T) {
	t.Parallel()
	out, code := runProbe(t, &fakeReference{answer: okAnswer})
	if code != 0 {
		t.Fatalf("exit %d, output:\n%s", code, out)
	}
	for _, want := range []string{"probe: US pass\n", "probe: JP pass\n", "AAPL", "7203"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestProbeDividendsTimeoutOnNonPayerPasses(t *testing.T) {
	t.Parallel()
	f := &fakeReference{answer: func(c broker.Contract) broker.DividendProjection {
		if c.Symbol == "TSLA" || c.Symbol == "SPY" {
			return broker.DividendProjection{Status: broker.DividendTimeout}
		}
		return okAnswer(c)
	}}
	out, code := runProbe(t, f)
	if code != 0 || !strings.Contains(out, "probe: US pass") {
		t.Fatalf("exit %d, output:\n%s", code, out)
	}
}

func TestProbeDividendsErrorOnNonPayerFails(t *testing.T) {
	t.Parallel()
	f := &fakeReference{answer: func(c broker.Contract) broker.DividendProjection {
		if c.Symbol == "TSLA" {
			return broker.DividendProjection{Status: broker.DividendError, ErrorCode: 200}
		}
		return okAnswer(c)
	}}
	out, code := runProbe(t, f)
	if code == 0 || !strings.Contains(out, "probe: US fail") || !strings.Contains(out, "probe: JP pass") {
		t.Fatalf("exit %d, output:\n%s", code, out)
	}
}

func TestProbeDividendsMissingDateFails(t *testing.T) {
	t.Parallel()
	for name, p := range map[string]broker.DividendProjection{
		"ok without date": {Status: broker.DividendOK},
		"ok bad date":     {Status: broker.DividendOK, NextExDate: ptr("2026-11-12")},
		"no value":        {Status: broker.DividendNoValue},
		"timeout":         {Status: broker.DividendTimeout},
	} {
		f := &fakeReference{answer: func(c broker.Contract) broker.DividendProjection {
			if c.Symbol == "KO" {
				return p
			}
			return okAnswer(c)
		}}
		out, code := runProbe(t, f)
		if code == 0 || !strings.Contains(out, "probe: US fail") {
			t.Errorf("%s: exit %d, output:\n%s", name, code, out)
		}
	}
}

func TestProbeDividendsPerMarket(t *testing.T) {
	t.Parallel()
	f := &fakeReference{answer: func(c broker.Contract) broker.DividendProjection {
		if c.Currency == "JPY" {
			return broker.DividendProjection{Status: broker.DividendNoValue}
		}
		return okAnswer(c)
	}}
	out, code := runProbe(t, f)
	if code == 0 || !strings.Contains(out, "probe: US pass") || !strings.Contains(out, "probe: JP fail") {
		t.Fatalf("exit %d, output:\n%s", code, out)
	}
	// --symbols limits the run to the markets of the named symbols
	f = &fakeReference{answer: f.answer}
	out, code = runProbe(t, f, "--symbols", "AAPL,MSFT,KO")
	if code != 0 || strings.Contains(out, "JP") || !strings.Contains(out, "probe: US pass") {
		t.Fatalf("US only: exit %d, output:\n%s", code, out)
	}
	if len(f.asked) != 1 || len(f.asked[0]) != 3 {
		t.Fatalf("asked %v, want one call of three symbols", f.asked)
	}
	if _, code = runProbe(t, f, "--symbols", "NOPE"); code == 0 {
		t.Fatal("an unknown symbol must fail")
	}
}

func TestProbeDividendsPrintsNoSecret(t *testing.T) {
	t.Parallel()
	f := &fakeReference{answer: func(broker.Contract) broker.DividendProjection {
		return broker.DividendProjection{Status: broker.DividendError, ErrorCode: 321}
	}}
	out, _ := runProbe(t, f)
	for _, bad := range []string{"postgres", "pw", "/h", "ibkr.sock", "DU"} {
		if strings.Contains(out, bad) {
			t.Errorf("output holds %q:\n%s", bad, out)
		}
	}
	// a dial failure prints the fixed text only
	var buf bytes.Buffer
	a := &app{profile: profile.Profile{Name: "real"}, stdout: &buf,
		getenv: func(string) string { return "/h" },
		deps: deps{newReference: func(context.Context, string, broker.Options) (broker.ReferenceClient, error) {
			return nil, errors.New("dial unix /h/run/ibkr.sock: secret-detail")
		}}}
	err := runProbeCommand(a, []string{"dividends"})
	if err == nil || strings.Contains(err.Error()+buf.String(), "secret-detail") || strings.Contains(err.Error()+buf.String(), "ibkr.sock") {
		t.Fatalf("dial failure: err %v, output %q", err, buf.String())
	}
}

func ptr[T any](v T) *T { return &v }

func init() {
	leakExcluded["probe"] = "takes no DSN and opens no store; TestProbeDividendsPrintsNoSecret covers its output"
}
