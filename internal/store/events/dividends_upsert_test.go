package events

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestUpsertDividendsPresentSendsDataBeforeAnswer(t *testing.T) {
	t.Parallel()
	db := &recDB{}
	s := &dividendStore{db: db}
	r := divRow("2026-11-09", 5, "present")
	wc, err := s.UpsertDividends(context.Background(), []contracts.DividendExpectation{r}, "dividend:2026-11-09")
	if err != nil || wc.Written != 1 {
		t.Fatalf("%+v, %v", wc, err)
	}
	call := db.calls[0]
	if len(call) != 2 || !strings.Contains(call[0].sql, "INTO obs.dividend_expectation") ||
		!strings.Contains(call[1].sql, "INTO obs.dividend_check") {
		t.Fatalf("statements = %+v; want the data row then the answer", call)
	}
	if call[0].args[1] != "2026-11-14" || call[0].args[3] != "0.25" {
		t.Errorf("data args = %v", call[0].args)
	}
	for _, st := range call {
		for _, a := range st.args {
			if a == 5 || a == int64(5) {
				t.Errorf("Attempt travelled as an argument: %v", st.args)
			}
		}
	}
}

func TestUpsertDividendsNoValueSendsOnlyAnswer(t *testing.T) {
	t.Parallel()
	db := &recDB{}
	s := &dividendStore{db: db}
	if _, err := s.UpsertDividends(context.Background(), []contracts.DividendExpectation{divRow("2026-11-09", 1, "no_value")}, "w"); err != nil {
		t.Fatal(err)
	}
	if len(db.calls[0]) != 1 || !strings.Contains(db.calls[0][0].sql, "INTO obs.dividend_check") {
		t.Errorf("statements = %+v; want the answer alone", db.calls[0])
	}
}

func TestUpsertDividendsRefusesPresentWithoutDate(t *testing.T) {
	t.Parallel()
	db := &recDB{}
	r := divRow("2026-11-09", 1, "present")
	r.NextExDate = nil
	_, err := (&dividendStore{db: db}).UpsertDividends(context.Background(), []contracts.DividendExpectation{r}, "w")
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Permanent || len(db.calls) != 0 {
		t.Errorf("got %v with %d calls; want Permanent and no call", err, len(db.calls))
	}
}
