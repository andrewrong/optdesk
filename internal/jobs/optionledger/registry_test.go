package optionledger

import (
	"errors"
	"testing"
)

func TestRowKeyJoinsKeyAttributes(t *testing.T) {
	t.Parallel()
	spec, ok := sectionByElement("ConversionRate")
	if !ok {
		t.Fatal("ConversionRate is not in the registry")
	}
	got, err := rowKey(spec, map[string]string{"reportDate": "20261109", "fromCurrency": "USD", "toCurrency": "JPY", "rate": "150"})
	if err != nil || got != "20261109|USD|JPY" {
		t.Fatalf("key %q err %v", got, err)
	}
	pos, _ := sectionByElement("OpenPosition")
	got, err = rowKey(pos, map[string]string{"reportDate": "20261109", "levelOfDetail": "SUMMARY", "conid": "11"})
	if err != nil || got != "20261109|SUMMARY|11||" {
		t.Fatalf("absent optional attributes must be empty text: %q err %v", got, err)
	}
}

func TestRowKeyMissingRequiredRefused(t *testing.T) {
	t.Parallel()
	for _, attrs := range []map[string]string{
		{"amount": "1"},
		{"transactionID": ""},
	} {
		spec, _ := sectionByElement("CashTransaction")
		if _, err := rowKey(spec, attrs); !errors.Is(err, errRowKey) {
			t.Fatalf("attrs %v: err %v", attrs, err)
		}
	}
	for _, el := range []string{"ChangeInNAV", "EquitySummaryByReportDateInBase", "ConversionRate", "OpenPosition"} {
		spec, ok := sectionByElement(el)
		if !ok {
			t.Fatalf("%s missing", el)
		}
		if _, err := rowKey(spec, map[string]string{}); !errors.Is(err, errRowKey) {
			t.Fatalf("%s: empty row accepted", el)
		}
	}
}

func TestLotRowKeyCarriesOpenDateTime(t *testing.T) {
	t.Parallel()
	spec, _ := sectionByElement("OpenPosition")
	base := map[string]string{"reportDate": "20261109", "levelOfDetail": "LOT", "conid": "11"}
	a := map[string]string{"openDateTime": "20261001;093000", "originatingTransactionID": "5"}
	b := map[string]string{"openDateTime": "20261002;093000", "originatingTransactionID": "5"}
	for k, v := range base {
		a[k], b[k] = v, v
	}
	ka, err := rowKey(spec, a)
	if err != nil || ka != "20261109|LOT|11|20261001;093000|5" {
		t.Fatalf("key %q err %v", ka, err)
	}
	kb, err := rowKey(spec, b)
	if err != nil || kb == ka {
		t.Fatalf("two lots collide: %q %q", ka, kb)
	}
}

func TestLotRowWithoutLotAttributesRefused(t *testing.T) {
	t.Parallel()
	spec, _ := sectionByElement("OpenPosition")
	_, err := rowKey(spec, map[string]string{"reportDate": "20261109", "levelOfDetail": "LOT", "conid": "11"})
	if !errors.Is(err, errRowKey) {
		t.Fatalf("err %v", err)
	}
	if _, err := rowKey(spec, map[string]string{"reportDate": "20261109", "levelOfDetail": "SUMMARY", "conid": "11"}); err != nil {
		t.Fatalf("summary row refused: %v", err)
	}
}
