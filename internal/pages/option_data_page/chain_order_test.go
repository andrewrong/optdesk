package option_data_page

import (
	"reflect"
	"testing"
)

func orderRow(id int64, expiry, right string, strike *float64) chainRow {
	r := chainRow{InstrumentID: id, Strike: newGap("no strike")}
	if expiry != "" {
		r.Expiry = &expiry
	}
	if right != "" {
		r.Right = &right
	}
	if strike != nil {
		r.Strike = priceCell(strike)
	}
	return r
}

func TestSortChainRows(t *testing.T) {
	t.Parallel()
	f := func(v float64) *float64 { return &v }
	rows := []chainRow{ // open-interest order
		orderRow(1, "2026-12-10", "put", f(100)),
		orderRow(2, "2026-11-12", "put", f(90)),
		orderRow(3, "", "call", f(50)),
		orderRow(4, "2026-12-10", "call", f(120)),
		orderRow(5, "2026-11-12", "call", f(200)),
		orderRow(6, "2026-11-12", "call", f(80)),
		orderRow(7, "2026-11-12", "call", nil),
		orderRow(8, "2026-11-12", "call", f(80)),
		orderRow(9, "2026-12-10", "put", f(9)),
		orderRow(10, "", "put", f(1)),
	}
	sortChainRows(rows)
	var got []int64
	for _, r := range rows {
		got = append(got, r.InstrumentID)
	}
	want := []int64{6, 8, 5, 7, 2, 4, 9, 1, 3, 10}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}
