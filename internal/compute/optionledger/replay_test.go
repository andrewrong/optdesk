package optionledger

import (
	"embed"
	"encoding/json"
	"math/rand"
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
)

//go:embed testdata/*.json
var testdataFS embed.FS

type fixture struct {
	Input    compute.LedgerInput
	Params   compute.LedgerParams
	Expected compute.LedgerResult
}

func loadFixture(t *testing.T, name string) fixture {
	t.Helper()
	raw, err := testdataFS.ReadFile("testdata/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var f fixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func checkFixture(t *testing.T, name string) compute.LedgerResult {
	t.Helper()
	f := loadFixture(t, name)
	got := New().Replay(f.Input, f.Params)
	if !reflect.DeepEqual(oldFigures(got), f.Expected) {
		g, _ := json.MarshalIndent(got, "", " ")
		w, _ := json.MarshalIndent(f.Expected, "", " ")
		t.Fatalf("%s: got\n%s\nwant\n%s", name, g, w)
	}
	return got
}

// optRow builds the attributes of an option trade row; time is the order time.
func optRow(tx, qty, proceeds, comm, oc, date, conid, strike, expiry, putCall, notes string) compute.LedgerTrade {
	return compute.LedgerTrade{TransactionID: tx, Revision: 1, Attributes: map[string]string{
		"assetCategory": "OPT", "symbol": "ABC", "underlyingSymbol": "ABC", "conid": conid,
		"strike": strike, "expiry": expiry, "putCall": putCall, "multiplier": "100",
		"quantity": qty, "proceeds": proceeds, "ibCommission": comm, "currency": "USD",
		"openCloseIndicator": oc, "notes": notes, "tradeDate": date, "orderTime": date + ";100000",
	}}
}

func stkRow(tx, qty, price, proceeds, comm, date string) compute.LedgerTrade {
	return compute.LedgerTrade{TransactionID: tx, Revision: 1, Attributes: map[string]string{
		"assetCategory": "STK", "symbol": "ABC", "quantity": qty, "tradePrice": price,
		"proceeds": proceeds, "ibCommission": comm, "currency": "USD",
		"tradeDate": date, "orderTime": date + ";100000",
	}}
}

func replay(trades ...compute.LedgerTrade) compute.LedgerResult {
	return New().Replay(compute.LedgerInput{Alias: "a", Trades: trades, AsOf: "2026-04-01"}, compute.LedgerParams{MarkMaxAgeSessions: 5})
}

func TestIdsStableWhenOlderHistoryAdded(t *testing.T) {
	t.Parallel()
	f := loadFixture(t, "wheel_one_cycle")
	before := New().Replay(f.Input, f.Params)
	older := []compute.LedgerTrade{
		optRow("900", "-1", "80", "-0.65", "O", "20251205", "999", "30", "20251219", "P", ""),
		optRow("901", "1", "0", "0", "C", "20251219", "999", "30", "20251219", "P", "Ep"),
	}
	in := f.Input
	in.Trades = append(append([]compute.LedgerTrade{}, older...), f.Input.Trades...)
	after := New().Replay(in, f.Params)
	ids := func(r compute.LedgerResult) (out []string) {
		for _, p := range r.Positions {
			out = append(out, p.PositionID)
		}
		for _, c := range r.Cycles {
			out = append(out, c.CycleID)
		}
		return out
	}
	want := ids(before)
	got := ids(after)
	for _, id := range want {
		found := false
		for _, g := range got {
			found = found || g == id
		}
		if !found {
			t.Errorf("id %s lost after older history: %v", id, got)
		}
	}
	if len(got) != len(want)+1 {
		t.Errorf("want one extra position, got %v", got)
	}
}

func TestSameRowsSameResult(t *testing.T) {
	t.Parallel()
	f := loadFixture(t, "wheel_one_cycle")
	first := New().Replay(f.Input, f.Params)
	if !reflect.DeepEqual(first, New().Replay(f.Input, f.Params)) {
		t.Fatal("two replays differ")
	}
	shuffled := f.Input
	shuffled.Trades = append([]compute.LedgerTrade{}, f.Input.Trades...)
	rand.New(rand.NewSource(7)).Shuffle(len(shuffled.Trades), func(i, j int) {
		shuffled.Trades[i], shuffled.Trades[j] = shuffled.Trades[j], shuffled.Trades[i]
	})
	if !reflect.DeepEqual(first, New().Replay(shuffled, f.Params)) {
		t.Fatal("row order changed the result")
	}
	if first.RuleVersion != 1 {
		t.Fatalf("rule version %d", first.RuleVersion)
	}
}
