package optionledger

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/andrewrong/optdesk/internal/sources"
)

type fixture struct {
	Statements []struct {
		AccountID string              `json:"account_id"`
		Trades    []map[string]string `json:"trades"`
	} `json:"statements"`
	Expected struct {
		Canonical string `json:"canonical"`
		SHA256    string `json:"sha256"`
	} `json:"expected"`
}

func loadFixture(t *testing.T) (fixture, []sources.FlexStatement) {
	t.Helper()
	b, err := os.ReadFile("testdata/report_a.json")
	if err != nil {
		t.Fatal(err)
	}
	var f fixture
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	var sts []sources.FlexStatement
	for _, s := range f.Statements {
		sts = append(sts, sources.FlexStatement{AccountID: s.AccountID, Trades: s.Trades})
	}
	return f, sts
}

func TestAccountAttributesRemovedBeforeHash(t *testing.T) {
	t.Parallel()
	_, sts := loadFixture(t)
	alias, rows, ok, err := mapStatement(map[string]string{"TESTACCT1": "a"}, sts[0])
	if err != nil || !ok || alias != "a" || len(rows) != 1 {
		t.Fatalf("map: %v %v %q %d", err, ok, alias, len(rows))
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(rows[0].Attributes), &got); err != nil {
		t.Fatal(err)
	}
	if _, has := got["accountId"]; has {
		t.Fatal("accountId kept")
	}
	if _, has := got["acctAlias"]; has {
		t.Fatal("acctAlias kept")
	}
	if rows[0].Source != "flex" || rows[0].TransactionID != "1001" || rows[0].LevelOfDetail != "EXECUTION" {
		t.Fatalf("row: %+v", rows[0])
	}
	// the hash does not depend on the removed attributes
	plain := map[string]string{}
	for k, v := range got {
		plain[k] = v
	}
	want, _ := AttributesSHA256(plain)
	if rows[0].AttributesSHA256 != want {
		t.Fatal("hash over other attributes")
	}
}

func TestHashEqualsFixtureHash(t *testing.T) {
	t.Parallel()
	f, sts := loadFixture(t)
	_, rows, _, err := mapStatement(map[string]string{"TESTACCT1": "a"}, sts[0])
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].AttributesSHA256 != f.Expected.SHA256 {
		t.Fatalf("hash %s, fixture %s", rows[0].AttributesSHA256, f.Expected.SHA256)
	}
	if rows[0].Attributes != f.Expected.Canonical {
		t.Fatalf("attributes %s, fixture %s", rows[0].Attributes, f.Expected.Canonical)
	}
}
