package optioneod

import (
	"reflect"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestSelect(t *testing.T) {
	t.Parallel()
	t.Run("run outcomes", func(t *testing.T) {
		t.Parallel()
		runOutcomes(t)
	})
	u := underlying{Symbol: "TEST", Currency: "USD"}
	cases := []struct {
		name     string
		chain    broker.Chain
		price    float64
		m        manifest
		expiries []string
		strikes  []string
	}{
		{"nearest expiries on or after the day", broker.Chain{Expirations: []string{"20261120", "20261105", "20261106", "20261113", "20261204"},
			Strikes: []string{"100"}},
			100, manifest{MaxExpiries: 2, StrikeBand: 0.1, MaxStrikesPerSide: 8}, []string{"20261106", "20261113"}, []string{"100"}},
		{"band cuts", broker.Chain{Expirations: []string{"20261106"}, Strikes: []string{"80", "90", "100", "110", "120"}},
			100, manifest{MaxExpiries: 1, StrikeBand: 0.1, MaxStrikesPerSide: 8}, []string{"20261106"}, []string{"90", "100", "110"}},
		{"per side nearest first, at or above counts above", broker.Chain{Expirations: []string{"20261106"},
			Strikes: []string{"96", "97", "98", "99", "100", "101", "102"}},
			100, manifest{MaxExpiries: 1, StrikeBand: 0.5, MaxStrikesPerSide: 2}, []string{"20261106"}, []string{"98", "99", "100", "101"}},
		{"compare by value", broker.Chain{Expirations: []string{"20261106"}, Strikes: []string{"100", "95", "1000", "9.5"}},
			98, manifest{MaxExpiries: 1, StrikeBand: 0.1, MaxStrikesPerSide: 8}, []string{"20261106"}, []string{"95", "100"}},
		{"nothing in band", broker.Chain{Expirations: []string{"20261106"}, Strikes: []string{"50"}},
			100, manifest{MaxExpiries: 1, StrikeBand: 0.1, MaxStrikesPerSide: 8}, nil, nil},
		{"all expired", broker.Chain{Expirations: []string{"20261105"}, Strikes: []string{"100"}},
			100, manifest{MaxExpiries: 3, StrikeBand: 0.1, MaxStrikesPerSide: 8}, nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := selectContracts(tc.chain, u, "2026-11-06", tc.price, tc.m)
			var exp, st []string
			for _, c := range got {
				if c.Right != "C" {
					continue
				}
				exp = append(exp, c.Expiry)
				st = append(st, c.Strike)
			}
			if len(tc.expiries) == 0 {
				if len(got) != 0 {
					t.Fatalf("want empty, got %v", got)
				}
				return
			}
			if len(got) != 2*len(exp) {
				t.Fatalf("both rights expected: %d contracts for %d calls", len(got), len(exp))
			}
			seenE, seenS := uniq(exp), uniq(st)
			if !reflect.DeepEqual(seenE, tc.expiries) || !reflect.DeepEqual(seenS, tc.strikes) {
				t.Fatalf("expiries %v strikes %v, want %v %v", seenE, seenS, tc.expiries, tc.strikes)
			}
		})
	}
}

// runOutcomes (a subtest of TestSelect) shows what Run does with the answer of the selection: an empty one is Permanent, no price is
// Retryable, and neither writes.
func runOutcomes(t *testing.T) {
	cases := []struct {
		name  string
		setup func(b *fakeBroker)
		kind  contracts.ErrorKind
	}{
		{"empty selection is permanent", func(b *fakeBroker) {
			b.chains = []broker.Chain{{Expirations: []string{"20261105"}, Strikes: []string{"100"}}}
		}, contracts.Permanent},
		{"no underlying price is retryable", func(b *fakeBroker) { b.price = nil }, contracts.Retryable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, "2026-11-06 17:00")
			tc.setup(r.b)
			wantFailure(t, r, tc.kind, 0)
		})
	}
}

func uniq(in []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func TestSelectContractFields(t *testing.T) {
	t.Parallel()
	chain := broker.Chain{Exchange: "SMART", TradingClass: "TEST", Multiplier: "100", Expirations: []string{"20261106"}, Strikes: []string{"100.50"}}
	got := selectContracts(chain, underlying{Symbol: "TEST", Currency: "USD"}, "2026-11-06", 100, manifest{MaxExpiries: 1, StrikeBand: 0.1, MaxStrikesPerSide: 1})
	want := broker.Contract{SecType: "OPT", Symbol: "TEST", Exchange: "SMART", Currency: "USD", Expiry: "20261106", Strike: "100.50",
		Right: "C", Multiplier: "100", TradingClass: "TEST"}
	if len(got) != 2 || got[0] != want || got[1].Right != "P" {
		t.Fatalf("got %+v", got)
	}
}

func wideChain() broker.Chain {
	return broker.Chain{Expirations: []string{"20261127"}, Strikes: seq(250, 400, 5)}
}

func callStrikes(cs []broker.Contract) []string {
	var out []string
	for _, c := range cs {
		if c.Right == "C" {
			out = append(out, c.Strike)
		}
	}
	return out
}

func TestSelectWideBandNoSideCap(t *testing.T) {
	t.Parallel()
	m := manifest{MaxExpiries: 1, StrikeBand: 0.1, MaxStrikesPerSide: 8, StrikeBandBelow: 0.20, StrikeBandAbove: 0.05}
	got := selectContracts(wideChain(), underlying{Symbol: "T", Currency: "USD"}, "2026-11-12", 355.51, m)
	want := seq(285, 370, 5)
	if len(want) != 18 || !reflect.DeepEqual(callStrikes(got), want) || len(got) != 36 {
		t.Fatalf("strikes %v (%d contracts), want %v for both rights", callStrikes(got), len(got), want)
	}
}

func TestSelectWideBandBuyPrice(t *testing.T) {
	t.Parallel()
	m := manifest{MaxExpiries: 1, StrikeBand: 0.1, MaxStrikesPerSide: 8, StrikeBandBelow: 0.20, StrikeBandAbove: 0.05,
		MaxStrikes: 5, BuyLow: "0.80", BuyHigh: "1.05", buy: map[string]string{"T": "340"}}
	got := selectContracts(wideChain(), underlying{Symbol: "T", Currency: "USD"}, "2026-11-12", 355.51, m)
	// the 5 nearest 340 are 330..350, all inside the band strikes 285..370
	if want := seq(285, 370, 5); !reflect.DeepEqual(callStrikes(got), want) {
		t.Fatalf("strikes %v, want %v", callStrikes(got), want)
	}
	m.StrikeBandBelow, m.StrikeBandAbove = 0.01, 0.01 // band 352..359 holds 355 only
	got = selectContracts(wideChain(), underlying{Symbol: "T", Currency: "USD"}, "2026-11-12", 355.51, m)
	if want := []string{"330", "335", "340", "345", "350", "355"}; !reflect.DeepEqual(callStrikes(got), want) {
		t.Fatalf("strikes %v, want union %v", callStrikes(got), want)
	}
}

func TestSelectWideBandExpiries(t *testing.T) {
	t.Parallel()
	chain := broker.Chain{Expirations: []string{"20261125", "20261126", "20261218", "20270107", "20270108"}, Strikes: []string{"100"}}
	m := manifest{StrikeBand: 0.1, StrikeBandBelow: 0.20, StrikeBandAbove: 0.05, MinDTE: 14, MaxDTE: 56}
	seen := map[string]bool{}
	for _, c := range selectContracts(chain, underlying{Symbol: "T", Currency: "USD"}, "2026-11-12", 100, m) {
		seen[c.Expiry] = true
	}
	if len(seen) != 3 || !seen["20261126"] || !seen["20261218"] || !seen["20270107"] {
		t.Fatalf("expiries %v", seen)
	}
}

func TestValidateRefusesNegativeStrikeBand(t *testing.T) {
	t.Parallel()
	base := func() manifest { return manifest{Zone: "America/New_York", FreezeAfter: "1700"} }
	if err := base().validate(); err != nil {
		t.Fatalf("valid base refused: %v", err)
	}
	below, above := base(), base()
	below.StrikeBandBelow, above.StrikeBandAbove = -0.01, -0.01
	for name, m := range map[string]manifest{"below": below, "above": above} {
		err := m.validate()
		if err == nil || !strings.Contains(err.Error(), "band") {
			t.Errorf("%s: negative band gave error %v, want one naming the band", name, err)
		}
	}
}

func TestSelectWideBandOneKeyFallsBackToStrikeBand(t *testing.T) {
	t.Parallel()
	m := manifest{MaxExpiries: 1, StrikeBand: 0.1, StrikeBandBelow: 0.20}
	got := selectContracts(wideChain(), underlying{Symbol: "T", Currency: "USD"}, "2026-11-12", 355.51, m)
	// 284.41 .. 391.06 (above falls back to 0.10), no cap by side
	if want := seq(285, 390, 5); !reflect.DeepEqual(callStrikes(got), want) {
		t.Fatalf("strikes %v, want %v", callStrikes(got), want)
	}
}

func TestValidateRefusesOneBuyKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, low, high, key string
	}{
		{"low alone", "0.80", "", "buy_high"},
		{"high alone", "", "1.05", "buy_low"},
		{"low not below high", "1.05", "1.05", "buy_low"},
		{"low above high", "1.10", "0.90", "buy_low"},
		{"low not decimal", "abc", "1.05", "buy_low"},
		{"high not decimal", "0.80", "x", "buy_high"},
		{"low not positive", "0", "1.05", "buy_low"},
		{"high negative", "0.80", "-1", "buy_high"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseManifest([]byte(buyKeyManifest(c.low, c.high)))
			if err == nil || !strings.Contains(err.Error(), c.key) {
				t.Fatalf("err %v, want one naming %s", err, c.key)
			}
		})
	}
	for _, ok := range [][2]string{{"0.80", "1.05"}, {"", ""}} {
		if _, err := parseManifest([]byte(buyKeyManifest(ok[0], ok[1]))); err != nil {
			t.Fatalf("low %q high %q: %v", ok[0], ok[1], err)
		}
	}
}

func buyKeyManifest(low, high string) string {
	s := "version = 1\nid = \"x\"\n"
	if low != "" {
		s += "buy_low = \"" + low + "\"\n"
	}
	if high != "" {
		s += "buy_high = \"" + high + "\"\n"
	}
	return s + "[[underlying]]\nsymbol = \"SPY\"\ncon_id = 1\ncurrency = \"USD\"\n"
}

func TestSelectBuyPriceWithoutKeysKeepsBand(t *testing.T) {
	t.Parallel()
	m := manifest{MaxExpiries: 1, StrikeBand: 0.1, MaxStrikesPerSide: 8, MaxStrikes: 5, buy: map[string]string{"T": "340"}}
	got := selectContracts(wideChain(), underlying{Symbol: "T", Currency: "USD"}, "2026-11-12", 355.51, m)
	plain := m
	plain.buy = nil
	want := selectContracts(wideChain(), underlying{Symbol: "T", Currency: "USD"}, "2026-11-12", 355.51, plain)
	if !reflect.DeepEqual(callStrikes(got), callStrikes(want)) || len(got) == 0 {
		t.Fatalf("strikes %v, want the band strikes %v", callStrikes(got), callStrikes(want))
	}
}

func TestValidateRefusesNonDecimalPrefixedBuyKey(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"0x1", "0x1p-1", "0b1"} {
		if _, err := parseManifest([]byte(buyKeyManifest("0.80", v))); err == nil || !strings.Contains(err.Error(), "buy_high") {
			t.Fatalf("buy_high %q: err %v, want an error naming buy_high", v, err)
		}
	}
}
