package flex

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func flexKindOf(t *testing.T, err error) (string, string) {
	t.Helper()
	var fe *flexError
	if !errors.As(err, &fe) {
		t.Fatalf("not a flex error: %v", err)
	}
	return fe.FlexKind(), fe.Code()
}

func TestDoctypeRefused(t *testing.T) {
	t.Parallel()
	_, err := parseReport(fixture(t, "statement_doctype.xml"))
	if k, _ := flexKindOf(t, err); k != kindNotReport {
		t.Fatalf("kind %q", k)
	}
	if _, err := parseReport([]byte("<!ENTITY a 'b'><FlexQueryResponse/>")); err == nil {
		t.Fatal("entity declaration accepted")
	}
	if _, err := parseReport([]byte("\x00<\x00F\x00l\x00e")); err == nil {
		t.Fatal("NUL body accepted")
	}
}

func TestOtherLevelsCounted(t *testing.T) {
	t.Parallel()
	st, err := parseReport(fixture(t, "statement_trades.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(st) != 1 || st[0].AccountID != "ACCT_A" || st[0].FromDate != "2026-10-01" {
		t.Fatalf("statements %+v", st)
	}
	if len(st[0].Trades) != 2 {
		t.Fatalf("trades %d, want 2", len(st[0].Trades))
	}
	if st[0].Trades[0]["transactionID"] != "9001" || st[0].Trades[0]["ibCommission"] != "-0.65" {
		t.Fatalf("row %v", st[0].Trades[0])
	}
	// ORDER, SUMMARY and the one CashTransaction row.
	if st[0].OtherLevels != 3 {
		t.Fatalf("other %d, want 3", st[0].OtherLevels)
	}
}

func TestParseRefusesBadTrade(t *testing.T) {
	t.Parallel()
	head := `<FlexQueryResponse><FlexStatements><FlexStatement accountId="X"><Trades>`
	tail := `</Trades></FlexStatement></FlexStatements></FlexQueryResponse>`
	for name, row := range map[string]string{
		"no id":      `<Trade levelOfDetail="EXECUTION"/>`,
		"no level":   `<Trade transactionID="1"/>`,
		"other acct": `<Trade transactionID="1" levelOfDetail="EXECUTION" accountId="Y"/>`,
	} {
		if _, err := parseReport([]byte(head + row + tail)); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	if _, err := parseReport([]byte(`<Other/>`)); err == nil || !strings.Contains(err.Error(), kindNotReport) {
		t.Fatalf("other root: %v", err)
	}
}
