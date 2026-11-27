package option_pnl_page

import (
	"bytes"
	"errors"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
)

// cashBooker answers CashIfAssigned from fixed lines and records the puts it was given.
type cashBooker struct {
	sumBooker
	due  []optionattrapi.CashDue
	err  error
	puts [][]optionattrapi.OpenPut
}

func (b *cashBooker) CashIfAssigned(puts []optionattrapi.OpenPut) ([]optionattrapi.CashDue, error) {
	b.mu.Lock()
	b.puts = append(b.puts, puts)
	b.mu.Unlock()
	return b.due, b.err
}

func cashLines(vm viewModel) []line { return vm.Accounts[0].CashDue }

func TestCashDueOneCurrency(t *testing.T) {
	t.Parallel()
	r := newRig()
	a := lotPos("p1", "AAA", "101", "190.00", "open") // 2 contracts x 100 x 190
	b := lotPos("p2", "CCC", "303", "38.00", "open")
	b.Quantity = "-3"
	r.replay.positions = []compute.LedgerPosition{a, b}
	r.attr.results["190.00"] = res("1", "1", "1", "1", "1", "", "-1", "0", "100")
	r.attr.results["38.00"] = res("1", "1", "1", "1", "1", "", "-1", "0", "100")
	got := cashLines(r.build(t))
	if len(got) != 1 {
		t.Fatalf("lines = %+v", got)
	}
	if got[0].Text != "$49,400.00 · 5 张合约 · 2 个仓位" {
		t.Errorf("text = %q", got[0].Text)
	}
	if got[0].Label != cashDueLabelZh || got[0].Hint == nil || *got[0].Hint != cashDueHintZh || got[0].Tone != "none" {
		t.Errorf("line = %+v", got[0])
	}
}

func TestCashDueTwoCurrencies(t *testing.T) {
	t.Parallel()
	r := newRig()
	a := lotPos("p1", "AAA", "101", "190.00", "open")
	b := lotPos("p2", "JJJ", "303", "3000", "open")
	b.Currency, b.Quantity = "JPY", "-1"
	c := lotPos("p3", "CCC", "404", "10.00", "open")
	r.replay.positions = []compute.LedgerPosition{a, b, c}
	got := cashLines(r.build(t))
	if len(got) != 2 {
		t.Fatalf("lines = %+v", got)
	}
	if got[0].Text != "$40,000.00 · 4 张合约 · 2 个仓位" || got[1].Text != "¥300,000.00 · 1 张合约 · 1 个仓位" {
		t.Errorf("texts = %q, %q", got[0].Text, got[1].Text)
	}
	if got[0].ID == got[1].ID {
		t.Errorf("ids repeat: %q", got[0].ID)
	}
}

func TestCashDueNoMarkLotCounts(t *testing.T) {
	t.Parallel()
	r := newRig()
	a := lotPos("p1", "AAA", "101", "90.00", "open")
	a.Multiplier = "10" // not the usual 100
	r.replay.positions = []compute.LedgerPosition{a}
	// no option_day row for 101: the lot is a no_mark card
	vm := r.build(t)
	got := cashLines(vm)
	if len(got) != 1 || got[0].Text != "$1,800.00 · 2 张合约 · 1 个仓位" {
		t.Fatalf("lines = %+v", got)
	}
}

func TestCashDueNoOpenPut(t *testing.T) {
	t.Parallel()
	r := newRig()
	ended := lotPos("p1", "AAA", "101", "90.00", "expired")
	long := lotPos("p2", "BBB", "202", "50.00", "open")
	long.Quantity = "2"
	r.replay.positions = []compute.LedgerPosition{ended, long}
	raw := r.raw(t)
	if len(cashLines(r.build(t))) != 0 {
		t.Fatalf("lines for no open short put")
	}
	if bytes.Contains(raw, []byte(`"cash_due"`)) {
		t.Errorf("page holds a cash_due key")
	}
}

func TestCashDueBadInputMuted(t *testing.T) {
	t.Parallel()
	r := newRig()
	a := lotPos("p1", "AAA", "101", "abc", "open")
	r.replay.positions = []compute.LedgerPosition{a}
	r.attr.results["abc"] = res("1", "1", "1", "1", "1", "", "-1", "0", "100")
	vm := r.build(t)
	got := cashLines(vm)
	if len(got) != 1 || got[0].Text != "无法计算：个别仓位的行权价或张数不是数字" || got[0].Tone != "muted" {
		t.Fatalf("lines = %+v", got)
	}
	if len(vm.Accounts[0].Lots) != 1 || vm.Accounts[0].Ledger == nil {
		t.Errorf("page not built around the refusal")
	}
	r2 := newRig()
	r2.cfg.Booker = &cashBooker{err: errors.Join(errors.New("x"), optionattrapi.ErrBadInput)}
	r2.replay.positions = []compute.LedgerPosition{lotPos("p1", "AAA", "101", "90.00", "open")}
	got = cashLines(r2.build(t))
	if len(got) != 1 || got[0].Tone != "muted" {
		t.Fatalf("booker refusal: %+v", got)
	}
}

func TestCashDueFromBooker(t *testing.T) {
	t.Parallel()
	r := newRig()
	b := &cashBooker{due: []optionattrapi.CashDue{{Currency: "USD", Lots: 7, Contracts: "9.5", Cash: "12345.678"}}}
	r.cfg.Booker = b
	r.replay.positions = []compute.LedgerPosition{lotPos("p1", "AAA", "101", "90.00", "open")}
	got := cashLines(r.build(t))
	if len(got) != 1 || got[0].Text != "$12,345.68 · 9.5 张合约 · 7 个仓位" {
		t.Fatalf("lines = %+v", got)
	}
	if len(b.puts) != 1 || len(b.puts[0]) != 1 || b.puts[0][0] != (optionattrapi.OpenPut{Currency: "USD", Strike: "90.00", Multiplier: "100", Quantity: "-2"}) {
		t.Errorf("puts = %+v", b.puts)
	}
}

func TestCashDueLineConstants(t *testing.T) {
	t.Parallel()
	if cashDueLabelZh == "" || cashDueHintZh == "" || cashDueRefusedZh == "" {
		t.Fatal("a cash due text is empty")
	}
	r := newRig()
	r.replay.positions = []compute.LedgerPosition{lotPos("p1", "AAA", "101", "90.00", "open")}
	l := cashLines(r.build(t))[0]
	if l.Label != cashDueLabelZh || *l.Hint != cashDueHintZh {
		t.Errorf("line does not carry the constants: %+v", l)
	}
}
