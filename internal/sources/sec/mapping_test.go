package sec

import (
	"os"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

var retrieved = time.Date(2026, 11, 6, 22, 0, 0, 0, time.UTC)

func readFile(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func find(facts []contracts.FundamentalFact, tax, concept string) []contracts.FundamentalFact {
	var out []contracts.FundamentalFact
	for _, f := range facts {
		if f.Taxonomy == tax && f.Concept == concept {
			out = append(out, f)
		}
	}
	return out
}

func TestMapKeepsAccnFiledForm(t *testing.T) {
	t.Parallel()
	body := readFile(t, "companyfacts_sample.json")
	got, err := mapCompanyFacts("cik:0000320193", body, retrieved, "", "")
	if err != nil {
		t.Fatal(err)
	}
	ocf := find(got, "us-gaap", "NetCashProvidedByUsedInOperatingActivities")
	if len(ocf) != 2 {
		t.Fatalf("ocf facts = %d, want 2 (every filed value is kept)", len(ocf))
	}
	f := ocf[0]
	if f.DocID != "0000320193-25-000079" || f.Form != "10-K" || f.Filed.Format("2006-01-02") != "2025-10-31" {
		t.Errorf("provenance %+v", f)
	}
	if f.Source != "sec" || f.Subject != "cik:0000320193" || f.Currency != "USD" || f.Unit != "USD" {
		t.Errorf("identity %+v", f)
	}
	if f.Value != "111482000000" || f.PeriodStart == nil || f.PeriodStart.Format("2006-01-02") != "2024-09-29" ||
		f.PeriodEnd.Format("2006-01-02") != "2025-09-27" {
		t.Errorf("value/period %+v", f)
	}
	if !f.RetrievedAt.Equal(retrieved) || len(f.RawSha256) != 64 {
		t.Errorf("retrieved %v sha %q", f.RetrievedAt, f.RawSha256)
	}
	if debt := find(got, "us-gaap", "LongTermDebt"); len(debt) != 1 || debt[0].PeriodStart != nil {
		t.Errorf("instant fact %+v", debt)
	}
}

func TestMapFilingRange(t *testing.T) {
	t.Parallel()
	got, err := mapCompanyFacts("cik:0000320193", readFile(t, "companyfacts_sample.json"), retrieved, "2026-01-01", "2026-12-31")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Form != "10-Q" {
		t.Fatalf("got %+v", got)
	}
}

func TestCoverPageSharesKept(t *testing.T) {
	t.Parallel()
	got, err := mapCompanyFacts("cik:0000320193", readFile(t, "companyfacts_sample.json"), retrieved, "", "")
	if err != nil {
		t.Fatal(err)
	}
	s := find(got, "dei", "EntityCommonStockSharesOutstanding")
	if len(s) != 1 || s[0].Unit != "shares" || s[0].Currency != "" || s[0].Value != "14776353000" {
		t.Fatalf("got %+v", s)
	}
}

func TestUnlistedConceptDropped(t *testing.T) {
	t.Parallel()
	got, err := mapCompanyFacts("cik:0000320193", readFile(t, "companyfacts_sample.json"), retrieved, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(find(got, "us-gaap", "AssetsUnlistedForTest")) + len(find(got, "dei", "EntityPublicFloat")); n != 0 {
		t.Fatalf("%d unlisted facts kept", n)
	}
}

func TestSicFromSubmissions(t *testing.T) {
	t.Parallel()
	got, err := mapSIC("cik:0000320193", readFile(t, "submissions_sample.json"), retrieved)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	f := got[0]
	if f.Concept != "StandardIndustrialClassification" || f.Taxonomy != "sec" || f.Unit != "sic" || f.Value != "6021" {
		t.Errorf("fact %+v", f)
	}
	if f.PeriodEnd.Format("2006-01-02") != "2026-11-06" || f.PeriodStart != nil || f.Source != "sec" {
		t.Errorf("period/source %+v", f)
	}
	none, err := mapSIC("cik:0000320193", []byte(`{"sic":""}`), retrieved)
	if err != nil || len(none) != 0 {
		t.Errorf("empty sic gave %+v %v", none, err)
	}
}
