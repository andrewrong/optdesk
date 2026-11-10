package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optionledger"
	"github.com/andrewrong/optdesk/internal/store"
)

// flexRow is one stored row: the attributes as text, revision 1.
func flexRow(t *testing.T, id string, attrs map[string]string) store.FlexTradeRow {
	t.Helper()
	attrs["transactionType"], attrs["levelOfDetail"] = "ExchTrade", "EXECUTION"
	attrs["orderTime"] = attrs["tradeDate"] + ";093000"
	raw, err := json.Marshal(attrs)
	if err != nil {
		t.Fatal(err)
	}
	return store.FlexTradeRow{Alias: "a", LevelOfDetail: "EXECUTION", TransactionID: id, Revision: 1, Attributes: string(raw)}
}

func optRow(t *testing.T, id, conid, sym, strike, expiry, pc, qty, proceeds, comm, cur, oc, notes, date string) store.FlexTradeRow {
	return flexRow(t, id, map[string]string{"assetCategory": "OPT", "symbol": sym, "underlyingSymbol": sym, "conid": conid,
		"strike": strike, "expiry": expiry, "putCall": pc, "multiplier": "100", "quantity": qty, "proceeds": proceeds,
		"ibCommission": comm, "currency": cur, "openCloseIndicator": oc, "notes": notes, "tradeDate": date})
}

func stkRow(t *testing.T, id, sym, qty, price, proceeds, comm, notes, date string) store.FlexTradeRow {
	return flexRow(t, id, map[string]string{"assetCategory": "STK", "symbol": sym, "quantity": qty, "tradePrice": price,
		"proceeds": proceeds, "ibCommission": comm, "currency": "USD", "notes": notes, "tradeDate": date})
}

// ledgerRows holds an open put, a closed put, an expired put, a put assigned into 200 shares with a call
// written on them, a close with no open, a cancelled row and a JPY name beside the USD ones.
func ledgerRows(t *testing.T) []store.FlexTradeRow {
	return []store.FlexTradeRow{
		optRow(t, "1001", "111", "ABC", "50", "20260220", "P", "-2", "300", "-1.30", "USD", "O", "", "20260105"),
		optRow(t, "1002", "111", "ABC", "50", "20260220", "P", "2", "0", "0", "USD", "C", "A", "20260220"),
		stkRow(t, "1003", "ABC", "200", "50", "-10000", "-1.00", "A", "20260220"),
		optRow(t, "1004", "222", "ABC", "55", "20260918", "C", "-2", "160", "-1.30", "USD", "O", "", "20260225"),
		optRow(t, "2001", "333", "DEF", "40", "20260220", "P", "-1", "100", "-0.65", "USD", "O", "", "20260105"),
		optRow(t, "2002", "333", "DEF", "40", "20260220", "P", "1", "0", "0", "USD", "C", "Ep", "20260220"),
		optRow(t, "3001", "444", "GHI", "30", "20260320", "P", "-1", "90", "-0.65", "USD", "O", "", "20260105"),
		optRow(t, "3002", "444", "GHI", "30", "20260320", "P", "1", "-20", "-0.65", "USD", "C", "", "20260110"),
		optRow(t, "4001", "555", "JKL", "50", "20260320", "P", "1", "-30", "-0.65", "USD", "C", "", "20260105"),
		optRow(t, "5001", "666", "BBB", "3000", "20260220", "P", "-1", "5000", "-50", "JPY", "O", "", "20260106"),
		optRow(t, "5002", "666", "BBB", "3000", "20260220", "P", "1", "0", "0", "JPY", "C", "Ep", "20260220"),
		optRow(t, "5003", "777", "BBB", "3100", "20260918", "P", "-1", "4000", "-50", "JPY", "O", "", "20260301"),
		optRow(t, "6001", "888", "MNO", "20", "20260918", "P", "-1", "70", "-0.65", "USD", "O", "", "20260302"),
		flexRow(t, "7001", map[string]string{"assetCategory": "OPT", "symbol": "PQR", "tradeDate": "20260303", "notes": "Ca"}),
	}
}

// ledgerApp is an app whose ledger store holds rows of alias a; the replay is the real rule and no mark reader
// is wired, so the open positions print no_mark.
func ledgerApp(t *testing.T, rows []store.FlexTradeRow) (*app, *bytes.Buffer) {
	t.Helper()
	a, _ := testApp(t, ledgerDSNLine)
	a.deps.newClock = func() clock.Clock { return fakeClock{time.Date(2026, 4, 1, 2, 0, 0, 0, time.UTC)} }
	a.deps.newOptionLedger = optionledger.New
	st := &ledgerFakeStore{aliases: []string{"a"}, rows: map[string][]store.FlexTradeRow{"a": rows}}
	a.deps.openOptionLedgerStore = func(context.Context, string) (store.OptionLedgerStore, error) { return st, nil }
	return a, a.stdout.(*bytes.Buffer)
}

func runLedger(t *testing.T, args ...string) string {
	t.Helper()
	a, out := ledgerApp(t, ledgerRows(t))
	if err := ledgerCmd(a, args); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestLedgerCommandGolden(t *testing.T) {
	t.Parallel()
	got := runLedger(t, "--cycles")
	want, err := os.ReadFile("testdata/ledger_golden.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("ot ledger --cycles output differs from testdata/ledger_golden.txt:\n%s", got)
	}
}

func TestLedgerUnresolvedListedApart(t *testing.T) {
	t.Parallel()
	got := runLedger(t)
	for _, want := range []string{"unresolved (1)\n  transaction 4001\n", "review (1)\n  transaction 7001\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
	// the closes with no open and the cancelled rows are in no position section
	for _, section := range []string{"open (", "closed (", "assigned (", "expired ("} {
		i := strings.Index(got, section)
		if i < 0 || i > strings.Index(got, "unresolved (") {
			t.Errorf("section %q is missing or after the lists apart:\n%s", section, got)
		}
	}
	if strings.Contains(got, "JKL") {
		t.Errorf("the unresolved close is printed as a position:\n%s", got)
	}
	only := runLedger(t, "--status", "unresolved")
	if strings.Contains(only, "open (") || !strings.Contains(only, "transaction 4001") {
		t.Errorf("--status unresolved printed:\n%s", only)
	}
}

func TestLedgerNoCrossCurrencySum(t *testing.T) {
	t.Parallel()
	got := runLedger(t)
	if strings.Contains(strings.ToLower(got), "total") {
		t.Errorf("the output has a total:\n%s", got)
	}
	jpy := 0
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "JPY") {
			jpy++
			if strings.Contains(line, "USD") {
				t.Errorf("a line mixes the currencies: %q", line)
			}
		}
	}
	if jpy == 0 || !strings.Contains(got, "5000.0000 JPY") || !strings.Contains(got, "300.0000 USD") {
		t.Errorf("amounts are not printed with their own currency:\n%s", got)
	}
}

func TestLedgerCycleShowsBasisAndPremiumApart(t *testing.T) {
	t.Parallel()
	got := runLedger(t, "--cycles")
	lines := strings.Split(got, "\n")
	find := func(prefix string) string {
		for _, l := range lines {
			if strings.HasPrefix(strings.TrimSpace(l), prefix) {
				return l
			}
		}
		t.Errorf("no line %q:\n%s", prefix, got)
		return ""
	}
	if l := find("cost per share (purchase price):"); !strings.Contains(l, "50") || strings.Contains(l, "premium") {
		t.Errorf("cost line %q", l)
	}
	if l := find("premium income:"); strings.Contains(l, "50.0000") && !strings.Contains(l, "USD") {
		t.Errorf("premium line %q", l)
	}
	find("shares: 200")
	find("stock realized:")
	find("stock unrealized: no_mark")
	if strings.Contains(runLedger(t), "cycle ") {
		t.Error("cycles are printed without --cycles")
	}
}

func TestLedgerTextOfEmptyResult(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	ledgerText(&b, "a", "2026-04-01", compute.LedgerResult{RuleVersion: 1}, ledgerOpts{})
	if !strings.Contains(b.String(), "open (0)") || !strings.Contains(b.String(), "unresolved (0)") {
		t.Errorf("output:\n%s", b.String())
	}
}
