package optioneod

import (
	"reflect"
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
