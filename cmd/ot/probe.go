package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
)

func init() { register("probe", runProbeCommand) }

const probeUsage = "usage: ot probe dividends [--symbols SYM,SYM,...]"

// probeRole says what a probe symbol must answer (docs/design/event-sources.md Decision 6).
type probeRole int

const (
	probePayer    probeRole = iota // must answer ok with a parsable next ex-date
	probeNonPayer                  // no_value and timeout are fine, error is not
)

// probeSymbol is one symbol of the fixed list of the dividend probe.
type probeSymbol struct {
	market   string
	role     probeRole
	contract broker.Contract
}

func usStock(sym string, role probeRole) probeSymbol {
	return probeSymbol{"US", role, broker.Contract{SecType: "STK", Symbol: sym, Exchange: "SMART", Currency: "USD"}}
}

func jpStock(sym string) probeSymbol {
	return probeSymbol{"JP", probePayer, broker.Contract{SecType: "STK", Symbol: sym, Exchange: "TSEJ", Currency: "JPY"}}
}

// probeList is the fixed list: three US payers, one US non-payer, one US ETF, two JP payers.
var probeList = []probeSymbol{
	usStock("AAPL", probePayer), usStock("MSFT", probePayer), usStock("KO", probePayer),
	usStock("TSLA", probeNonPayer), usStock("SPY", probeNonPayer),
	jpStock("7203"), jpStock("8306"),
}

var probeStatusText = map[broker.DividendStatus]string{
	broker.DividendOK: "ok", broker.DividendNoValue: "no_value", broker.DividendTimeout: "timeout", broker.DividendError: "error",
}

// runProbeCommand is `ot probe dividends [--symbols ...]`. It prints one line per symbol, then one
// `probe: <market> pass|fail` line per market in the run, and exits 1 unless every market passes.
// Output holds symbols, statuses and dates only: no key, DSN, path or account text.
func runProbeCommand(a *app, args []string) error {
	if len(args) == 0 || args[0] != "dividends" {
		return errors.New(probeUsage)
	}
	fs := flag.NewFlagSet("probe dividends", flag.ContinueOnError)
	fs.SetOutput(discard{})
	symbols := fs.String("symbols", "", "comma-separated symbols of the fixed list")
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 {
		return errors.New(probeUsage)
	}
	list, err := selectProbeSymbols(*symbols)
	if err != nil {
		return err
	}
	home := a.getenv("OT_HOME")
	if home == "" {
		return errors.New("OT_HOME is not set")
	}
	socket := filepath.Join(home, "run", "ibkr.sock")
	if len(socket) >= maxSocketPath {
		return errors.New("socket path too long")
	}
	ctx := context.Background()
	client, err := a.deps.newReference(ctx, socket, broker.Options{})
	if err != nil {
		_, _ = fmt.Fprintln(a.stdout, "probe: broker is not reachable")
		return &exitError{code: 1, err: errors.New("probe failed: broker is not reachable")}
	}
	defer func() { _ = client.Close() }()

	pass := map[string]bool{}
	var order []string
	for _, s := range list {
		if _, seen := pass[s.market]; !seen {
			pass[s.market] = true
			order = append(order, s.market)
		}
	}
	for _, m := range order {
		var group []probeSymbol
		for _, s := range list {
			if s.market == m {
				group = append(group, s)
			}
		}
		pass[m] = probeMarket(ctx, a, client, group)
	}
	allPass := true
	for _, m := range order {
		word := "pass"
		if !pass[m] {
			word, allPass = "fail", false
		}
		_, _ = fmt.Fprintf(a.stdout, "probe: %s %s\n", m, word)
	}
	if !allPass {
		return &exitError{code: 1, err: errors.New("probe failed")}
	}
	return nil
}

// probeMarket asks for one market's symbols in one call, prints a line each and reports whether the market passes.
func probeMarket(ctx context.Context, a *app, c broker.ReferenceClient, group []probeSymbol) bool {
	cs := make([]broker.Contract, len(group))
	for i, s := range group {
		cs[i] = s.contract
	}
	got, err := c.Dividends(ctx, cs)
	if err != nil || len(got) != len(group) {
		for _, s := range group {
			_, _ = fmt.Fprintf(a.stdout, "%s %s error\n", s.market, s.contract.Symbol)
		}
		return false
	}
	ok := true
	for i, s := range group {
		p := got[i]
		text := probeStatusText[p.Status]
		date := ""
		if p.Status == broker.DividendOK && p.NextExDate != nil {
			if _, perr := time.Parse("20060102", *p.NextExDate); perr == nil {
				date = *p.NextExDate
			}
		}
		if p.Status == broker.DividendOK && date == "" {
			text = "ok_no_date"
		}
		if text == "" {
			text = "error"
		}
		line := fmt.Sprintf("%s %s %s", s.market, s.contract.Symbol, text)
		if date != "" {
			line += " " + date
		}
		_, _ = fmt.Fprintln(a.stdout, line)
		switch s.role {
		case probePayer:
			ok = ok && p.Status == broker.DividendOK && date != ""
		case probeNonPayer:
			ok = ok && p.Status != broker.DividendError && text != "error"
		}
	}
	return ok
}

// selectProbeSymbols is the fixed list, or the members of it that --symbols names (an unknown symbol is an error).
func selectProbeSymbols(flagValue string) ([]probeSymbol, error) {
	if flagValue == "" {
		return probeList, nil
	}
	var out []probeSymbol
	for _, name := range strings.Split(flagValue, ",") {
		name = strings.TrimSpace(name)
		found := false
		for _, s := range probeList {
			if s.contract.Symbol == name {
				out = append(out, s)
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("probe: unknown symbol %q; the list is fixed (see ot probe dividends in docs/contracts/ot.md)", name)
		}
	}
	return out, nil
}

// discard is an io.Writer that drops flag-package output.
type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
