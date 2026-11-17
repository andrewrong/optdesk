package optionevidence_test

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optionevidence"
	"github.com/andrewrong/optdesk/internal/compute/optionevidenceapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func atmChain(t *testing.T) []optionevidenceapi.ChainQuote {
	t.Helper()
	raw, err := os.ReadFile("testdata/atm.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Chain []optionevidenceapi.ChainQuote
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f.Chain
}

func q(expiry, right, strike, iv, source string) optionevidenceapi.ChainQuote {
	return optionevidenceapi.ChainQuote{Expiry: contracts.Day(expiry), Right: right, Strike: strike, IV: iv, Source: source}
}

func pick(t *testing.T, chain []optionevidenceapi.ChainQuote, spot string) (string, string) {
	t.Helper()
	return optionevidence.NewATMPicker().ATM(chain, "2026-12-18", spot, "option_eod")
}

func wantPair(t *testing.T, iv, strike, wantIV, wantStrike string) {
	t.Helper()
	if iv != wantIV || strike != wantStrike {
		t.Errorf("got (%q, %q), want (%q, %q)", iv, strike, wantIV, wantStrike)
	}
}

func TestATMNearestStrike(t *testing.T) {
	t.Parallel()
	iv, k := pick(t, atmChain(t), "96.50")
	wantPair(t, iv, k, "0.294", "95")
}

func TestATMTieLowerStrike(t *testing.T) {
	t.Parallel()
	chain := []optionevidenceapi.ChainQuote{
		q("2026-12-18", "P", "100", "0.281", "option_eod"),
		q("2026-12-18", "P", "95", "0.294", "option_eod"),
	}
	iv, k := pick(t, chain, "97.50")
	wantPair(t, iv, k, "0.294", "95")
}

func TestATMPutsOnly(t *testing.T) {
	t.Parallel()
	chain := []optionevidenceapi.ChainQuote{
		q("2026-12-18", "C", "96.5", "0.270", "option_eod"),
		q("2026-12-18", "P", "95", "0.294", "option_eod"),
	}
	iv, k := pick(t, chain, "96.50")
	wantPair(t, iv, k, "0.294", "95")
}

func TestATMSourceMatches(t *testing.T) {
	t.Parallel()
	chain := []optionevidenceapi.ChainQuote{
		q("2026-12-18", "P", "96.5", "0.270", "option_open"),
		q("2026-12-18", "P", "95", "0.294", "option_eod"),
	}
	iv, k := pick(t, chain, "96.50")
	wantPair(t, iv, k, "0.294", "95")
}

func TestATMOtherExpiryIgnored(t *testing.T) {
	t.Parallel()
	chain := []optionevidenceapi.ChainQuote{
		q("2027-01-15", "P", "96.5", "0.270", "option_eod"),
		q("2026-12-18", "P", "95", "0.294", "option_eod"),
	}
	iv, k := pick(t, chain, "96.50")
	wantPair(t, iv, k, "0.294", "95")
}

func TestATMNoIVRowSkipped(t *testing.T) {
	t.Parallel()
	chain := []optionevidenceapi.ChainQuote{
		q("2026-12-18", "P", "96", "", "option_eod"),
		q("2026-12-18", "P", "97", "0", "option_eod"),
		q("2026-12-18", "P", "95", "0.294", "option_eod"),
	}
	iv, k := pick(t, chain, "96.50")
	wantPair(t, iv, k, "0.294", "95")
}

func TestATMBandTenPercent(t *testing.T) {
	t.Parallel()
	in := []optionevidenceapi.ChainQuote{q("2026-12-18", "P", "90", "0.3", "option_eod")}
	iv, k := pick(t, in, "100")
	wantPair(t, iv, k, "0.3", "90")
	out := []optionevidenceapi.ChainQuote{q("2026-12-18", "P", "89.99", "0.3", "option_eod")}
	iv, k = pick(t, out, "100")
	wantPair(t, iv, k, "", "")
}

func TestATMNoRowIsEmpty(t *testing.T) {
	t.Parallel()
	iv, k := pick(t, nil, "96.50")
	wantPair(t, iv, k, "", "")
}

func TestATMBadSpotIsEmpty(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"", "abc", "0", "-5"} {
		iv, k := pick(t, atmChain(t), s)
		wantPair(t, iv, k, "", "")
	}
}

func TestATMDoesNotChangeChain(t *testing.T) {
	t.Parallel()
	chain := atmChain(t)
	before := append([]optionevidenceapi.ChainQuote(nil), chain...)
	pick(t, chain, "96.50")
	if !reflect.DeepEqual(chain, before) {
		t.Error("chain changed")
	}
}

func TestATMPickerContractReal(t *testing.T) {
	t.Parallel()
	p := optionevidence.NewATMPicker()
	chain := atmChain(t)
	iv, k := p.ATM(chain, "2026-12-18", "96.50", "option_eod")
	wantPair(t, iv, k, "0.294", "95")
	iv, k = p.ATM(chain, "2026-12-18", "96.50", "option_none")
	wantPair(t, iv, k, "", "")
}
