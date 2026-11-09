package flex

import "testing"

func TestContainerRowsKeyedByRowName(t *testing.T) {
	t.Parallel()
	sts, err := parseReport(fixture(t, "report_sections.xml"))
	if err != nil || len(sts) != 1 {
		t.Fatalf("%v %d", err, len(sts))
	}
	s := sts[0].Sections
	if len(s["CashTransaction"]) != 2 || len(s["OpenPosition"]) != 1 || len(s["ConversionRate"]) != 1 {
		t.Fatalf("%v", s)
	}
	if s["CashTransaction"][1]["transactionID"] != "7002" {
		t.Fatalf("%v", s["CashTransaction"])
	}
	for _, k := range []string{"CashTransactions", "Trades", "Trade"} {
		if _, ok := s[k]; ok {
			t.Fatalf("key %s present", k)
		}
	}
}

func TestSingleElementIsOneRow(t *testing.T) {
	t.Parallel()
	sts, err := parseReport(fixture(t, "report_sections.xml"))
	if err != nil {
		t.Fatal(err)
	}
	rows := sts[0].Sections["ChangeInNAV"]
	if len(rows) != 1 || rows[0]["endingValue"] != "1010.25" {
		t.Fatalf("%v", rows)
	}
}

func TestEveryAttributeKeptAsText(t *testing.T) {
	t.Parallel()
	sts, err := parseReport(fixture(t, "report_sections.xml"))
	if err != nil {
		t.Fatal(err)
	}
	r := sts[0].Sections["ConversionRate"][0]
	want := map[string]string{"reportDate": "2026-10-31", "fromCurrency": "USD", "toCurrency": "JPY", "rate": "150.125"}
	if len(r) != len(want) {
		t.Fatalf("%v", r)
	}
	for k, v := range want {
		if r[k] != v {
			t.Fatalf("%s=%q", k, r[k])
		}
	}
	if sts[0].Sections["OpenPosition"][0]["accountId"] != "ACCT_A" {
		t.Fatal("account attribute dropped")
	}
}

func TestTradesFieldUnchanged(t *testing.T) {
	t.Parallel()
	sts, err := parseReport(fixture(t, "report_sections.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sts[0].Trades) != 1 || sts[0].Trades[0]["transactionID"] != "9001" {
		t.Fatalf("%v", sts[0].Trades)
	}
}

func TestNoSectionsGivesEmptyMap(t *testing.T) {
	t.Parallel()
	body := `<FlexQueryResponse><FlexStatements><FlexStatement accountId="ACCT_A"><Trades/></FlexStatement></FlexStatements></FlexQueryResponse>`
	sts, err := parseReport([]byte(body))
	if err != nil || len(sts) != 1 {
		t.Fatalf("%v", err)
	}
	if sts[0].Sections == nil || len(sts[0].Sections) != 0 {
		t.Fatalf("%#v", sts[0].Sections)
	}
}

func TestEntityInSectionStillRefused(t *testing.T) {
	t.Parallel()
	const tail = `<FlexQueryResponse><FlexStatements><FlexStatement accountId="ACCT_A"><ChangeInNAV a="&e;"/>` +
		`</FlexStatement></FlexStatements></FlexQueryResponse>`
	body := `<!DOCTYPE x [<!ENTITY e "v">]>` + tail
	if _, err := parseReport([]byte(body)); err == nil {
		t.Fatal("accepted")
	}
	if _, err := parseReport([]byte(tail)); err == nil {
		t.Fatal("undeclared entity accepted")
	}
}
