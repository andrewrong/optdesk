//go:build pg

package events

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/dividend"
	"github.com/andrewrong/optdesk/internal/store/fundkey"
)

// keysMigrations is the list every pg test of this package applies, in order: master.instrument (0001 to 0008),
// obs.fundamental_fact (0017, which 0046 alters), the two event tables (0020, 0023) and the keys (0046).
var keysMigrations = []string{
	"0001_master_session_day.sql", "0002_series.sql", "0003_ops_ledger.sql", "0004_session_day_grant.sql",
	"0005_session_day_loader.sql", "0006_option_eod.sql", "0007_quote_snapshot.sql", "0008_master_instrument.sql",
	"0017_fundamental_fact.sql", "0020_events.sql", "0023_dividends.sql", "0046_fundamentals_keys.sql",
}

// keysRoles are the roles the files of keysMigrations grant to.
var keysRoles = []string{
	"ot_owner", "ot_runner", "ot_loader", "ot_ro_pages", "ot_w_events", "ot_w_fundamental", "ot_w_master",
	"ot_w_option_eod", "ot_w_quote_snapshot", "ot_w_series_fred",
}

func applyMigration(t *testing.T, name, file string) {
	t.Helper()
	raw, err := os.ReadFile("../../../migrations/" + file)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	up, _, _ := strings.Cut(string(raw), "-- +goose Down")
	execAdmin(t, name, strings.Replace(up, "-- +goose Up", "", 1))
}

func execAdmin(t *testing.T, name, sql string, args ...any) {
	t.Helper()
	if _, err := pgxAs(t, name).Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%.60s: %v", sql, err)
	}
}

func connectAs(t *testing.T, name, role string) *pgx.Conn {
	t.Helper()
	c, err := pgx.Connect(context.Background(), connURL(t, name, role))
	if err != nil {
		t.Fatalf("connect as %s: %v", role, err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

// seedInstrument makes the instrument a present dividend row refers to.
func seedInstrument(t *testing.T, name string, id int) {
	t.Helper()
	execAdmin(t, name, `INSERT INTO master.instrument (instrument_id, instrument_type, name, currency, mic, calendar_id, status,
 price_increment, lot_size, recorded_at) OVERRIDING SYSTEM VALUE
VALUES ($1, 'stock', 'X', 'USD', 'XNAS', 'XNYS', 'active', 0.01, 1, now())`, id)
}

type earningsPG interface {
	store.EarningsDateStore
	fundkey.EarningsUpserter
}

type dividendsPG interface {
	dividend.Store
	fundkey.DividendUpserter
}

func openEarnings(t *testing.T, name, role string) earningsPG {
	t.Helper()
	s, err := OpenEarningsDate(context.Background(), connURL(t, name, role))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	u, ok := s.(earningsPG)
	if !ok {
		t.Fatalf("%T does not upsert", s)
	}
	return u
}

func openDividends(t *testing.T, name, role string) dividendsPG {
	t.Helper()
	s, err := OpenDividend(context.Background(), connURL(t, name, role))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	u, ok := s.(dividendsPG)
	if !ok {
		t.Fatalf("%T does not upsert", s)
	}
	return u
}

func tableCount(t *testing.T, name, table string) int {
	t.Helper()
	var n int
	if err := pgxAs(t, name).QueryRow(context.Background(), `SELECT count(*) FROM `+table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func upsertOne(t *testing.T, s earningsPG, r contracts.EarningsDate) fundkey.WriteCount {
	t.Helper()
	wc, err := s.UpsertEarnings(context.Background(), []contracts.EarningsDate{r}, "test")
	if err != nil {
		t.Fatal(err)
	}
	return wc
}

func read(t *testing.T, s earningsPG, from, to string) []contracts.EarningsDate {
	t.Helper()
	got, err := s.EarningsDates(context.Background(), issuer, contracts.Day(from), contracts.Day(to))
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func onDay(r contracts.EarningsDate, day string) contracts.EarningsDate {
	r.ObservedOn.Time = mustDay(day)
	return r
}

func TestMovedAlphaVantageDateReplacesRow(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	s := openEarnings(t, name, "ot_w_events")
	r := listedRow("alphavantage", "2026-11-20", dayPtr("2026-09-30"))
	if wc := upsertOne(t, s, r); wc.Written != 1 || wc.Unchanged != 0 {
		t.Fatalf("first call = %+v, want 1 written", wc)
	}
	if wc := upsertOne(t, s, r); wc.Written != 0 || wc.Unchanged != 1 {
		t.Fatalf("equal call = %+v, want 1 unchanged", wc)
	}
	moved := r
	moved.ReportDate = dayPtr("2026-11-25")
	if wc := upsertOne(t, s, moved); wc.Written != 1 {
		t.Fatalf("moved call = %+v, want 1 written", wc)
	}
	if n := tableCount(t, name, "obs.earnings_date"); n != 1 {
		t.Errorf("%d data rows, want the one row replaced", n)
	}
	got := read(t, s, "2026-11-09", "2026-11-09")
	if len(got) != 1 || got[0].ReportDate.Format("2006-01-02") != "2026-11-25" {
		t.Errorf("view = %+v, want the moved date", got)
	}
}

func TestTwoPeriodsOfOneIssuerAreTwoRows(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	s := openEarnings(t, name, "ot_w_events")
	upsertOne(t, s, onDay(listedRow("alphavantage", "2026-08-01", dayPtr("2026-06-30")), "2026-07-20"))
	upsertOne(t, s, listedRow("alphavantage", "2026-11-20", dayPtr("2026-09-30")))
	if n := tableCount(t, name, "obs.earnings_date"); n != 2 {
		t.Errorf("%d data rows, want 2", n)
	}
}

func TestOptionalColumnsNeverBlanked(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	s := openEarnings(t, name, "ot_w_events")
	full := listedRow("jquants", "2026-11-20", dayPtr("2027-03-31"))
	est := "1.25"
	full.Estimate, full.TimeOfDay = &est, "post-market"
	upsertOne(t, s, full)
	bare := listedRow("jquants", "2026-11-20", nil)
	if wc := upsertOne(t, s, bare); wc.Written != 0 || wc.Unchanged != 1 {
		t.Errorf("a run without optional values = %+v, want 1 unchanged", wc)
	}
	got := read(t, s, "2026-11-09", "2026-11-09")
	if len(got) != 1 || got[0].Estimate == nil || *got[0].Estimate != "1.25" || got[0].TimeOfDay != "post-market" ||
		got[0].FiscalPeriodEnd == nil || got[0].FiscalPeriodEnd.Format("2006-01-02") != "2027-03-31" {
		t.Errorf("view = %+v, want estimate, time of day and fiscal end kept", got)
	}
}

func TestErrorAnswerNeverReplacesListedAnswer(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	s := openEarnings(t, name, "ot_w_events")
	upsertOne(t, s, listedRow("jquants", "2026-11-20", nil))
	if wc := upsertOne(t, s, row("2026-11-09", 1, "error")); wc.Written != 0 {
		t.Errorf("error over listed = %+v, want unchanged", wc)
	}
	if got := read(t, s, "2026-11-09", "2026-11-09"); len(got) != 1 || got[0].Status != "listed" {
		t.Errorf("view = %+v, want the listed answer", got)
	}
	upsertOne(t, s, row("2026-11-10", 1, "error"))
	upsertOne(t, s, onDay(listedRow("jquants", "2026-11-20", nil), "2026-11-10"))
	if got := read(t, s, "2026-11-10", "2026-11-10"); len(got) != 1 || got[0].Status != "listed" {
		t.Errorf("view = %+v, want the listed answer to replace the error", got)
	}
}

func TestSecondAnswerOfADayOverwritesFirst(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	s := openEarnings(t, name, "ot_w_events")
	upsertOne(t, s, row("2026-11-09", 1, "absent"))
	if wc := upsertOne(t, s, listedRow("jquants", "2026-11-20", nil)); wc.Written != 1 {
		t.Errorf("listed over absent = %+v, want 1 written", wc)
	}
	got := read(t, s, "2026-11-09", "2026-11-09")
	if len(got) != 1 || got[0].Status != "listed" || got[0].ReportDate == nil || got[0].Attempt != 1 {
		t.Errorf("view = %+v, want one listed answer", got)
	}
	if n := tableCount(t, name, "obs.earnings_check"); n != 1 {
		t.Errorf("%d answers, want 1", n)
	}
}

func TestReplayNeverOverwritesAnswer(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	s := openEarnings(t, name, "ot_w_events")
	ctx := context.Background()
	upsertOne(t, s, listedRow("alphavantage", "2026-11-20", dayPtr("2026-09-30")))
	moved := listedRow("alphavantage", "2026-11-25", dayPtr("2026-09-30"))
	if _, err := s.ReplayEarnings(ctx, []contracts.EarningsDate{moved, row("2026-11-08", 1, "absent")}, "replay"); err != nil {
		t.Fatal(err)
	}
	got := read(t, s, "2026-11-08", "2026-11-09")
	if len(got) != 2 || got[0].Status != "absent" || got[1].Status != "listed" {
		t.Fatalf("view = %+v, want the new day stored and the stored answer kept", got)
	}
	if got[1].ReportDate.Format("2006-01-02") != "2026-11-25" {
		t.Errorf("report date = %v, want the data row replayed", got[1].ReportDate)
	}
	if _, err := s.ReplayEarnings(ctx, []contracts.EarningsDate{row("2026-11-09", 1, "error")}, "replay"); err != nil {
		t.Fatal(err)
	}
	if got := read(t, s, "2026-11-09", "2026-11-09"); got[0].Status != "listed" {
		t.Errorf("a replayed error overwrote the answer: %+v", got)
	}
}

func columns(t *testing.T, c *pgx.Conn, view string) []string {
	t.Helper()
	rows, err := c.Query(context.Background(), `SELECT column_name || ' ' || data_type FROM information_schema.columns
WHERE table_schema = 'rd' AND table_name = $1 ORDER BY ordinal_position`, view)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func TestViewsKeepThirteenColumnsAndTypes(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	c := connectAs(t, name, "ot_w_events")
	wantE := "observed_on date,issuer text,attempt integer,symbol text,market text,status text,report_date date," +
		"fiscal_period_end date,time_of_day text,estimate text,error_code text,source text,received_at timestamp with time zone"
	wantD := "session_day date,instrument_id integer,attempt integer,issuer text,status text,past_12m text,next_12m text," +
		"next_amount text,next_ex_date date,currency text,error_code text,source text,received_at timestamp with time zone"
	if got := strings.Join(columns(t, c, "earnings_date_r1"), ","); got != wantE {
		t.Errorf("rd.earnings_date_r1 = %s", got)
	}
	if got := strings.Join(columns(t, c, "dividend_expectation_r1"), ","); got != wantD {
		t.Errorf("rd.dividend_expectation_r1 = %s", got)
	}
	s := openEarnings(t, name, "ot_w_events")
	upsertOne(t, s, listedRow("jquants", "2026-11-20", nil))
	upsertOne(t, s, row("2026-11-10", 1, "absent"))
	got := read(t, s, "2026-11-09", "2026-11-10")
	if len(got) != 2 || got[0].Attempt != 1 || got[0].ReportDate == nil || got[0].ReportDate.Format("2006-01-02") != "2026-11-20" ||
		got[1].ReportDate != nil || got[1].FiscalPeriodEnd != nil || got[1].Status != "absent" {
		t.Errorf("rows = %+v, want attempt 1, the report date of the data row, NULL dates for absent", got)
	}
	seedInstrument(t, name, divInstrument)
	d := openDividends(t, name, "ot_w_events")
	if _, err := d.UpsertDividends(context.Background(), []contracts.DividendExpectation{
		divRow("2026-11-09", 1, "present"), divRow("2026-11-10", 1, "no_value")}, "test"); err != nil {
		t.Fatal(err)
	}
	dv, err := d.Dividends(context.Background(), divInstrument, "2026-11-09", "2026-11-10")
	if err != nil || len(dv) != 2 || dv[0].InstrumentID != divInstrument || dv[0].Attempt != 1 || dv[0].NextAmount == nil ||
		dv[1].NextExDate != nil || dv[1].NextAmount != nil {
		t.Errorf("dividends = %+v, %v", dv, err)
	}
}

func TestDividendResolveFailedAnswerStored(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	d := openDividends(t, name, "ot_w_events")
	r := divRow("2026-11-09", 1, "error")
	r.InstrumentID, r.ErrorCode = 999, "resolve_failed"
	wc, err := d.UpsertDividends(context.Background(), []contracts.DividendExpectation{r}, "test")
	if err != nil || wc.Written != 1 {
		t.Fatalf("%+v, %v", wc, err)
	}
	got, err := d.Dividends(context.Background(), 999, "2026-11-09", "2026-11-09")
	if err != nil || len(got) != 1 || got[0].Status != "error" || got[0].ErrorCode != "resolve_failed" {
		t.Errorf("rows = %+v, %v; want the stored answer of an id that is not in master.instrument", got, err)
	}
}

func TestDividendPresentUnknownInstrumentRefused(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	d := openDividends(t, name, "ot_w_events")
	r := divRow("2026-11-09", 1, "present")
	r.InstrumentID = 999
	_, err := d.UpsertDividends(context.Background(), []contracts.DividendExpectation{r}, "test")
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Permanent {
		t.Fatalf("err = %v, want Permanent", err)
	}
	if n := tableCount(t, name, "obs.dividend_check") + tableCount(t, name, "obs.dividend_expectation"); n != 0 {
		t.Errorf("%d rows after a refused call, want the transaction rolled back", n)
	}
}

func TestMovedExDateIsSecondRow(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	seedInstrument(t, name, divInstrument)
	d := openDividends(t, name, "ot_w_events")
	ctx := context.Background()
	first := divRow("2026-11-09", 1, "present")
	moved := divRow("2026-11-10", 1, "present")
	moved.NextExDate.Time = mustDay("2026-11-21")
	if _, err := d.UpsertDividends(ctx, []contracts.DividendExpectation{first}, "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.UpsertDividends(ctx, []contracts.DividendExpectation{moved}, "test"); err != nil {
		t.Fatal(err)
	}
	if n := tableCount(t, name, "obs.dividend_expectation"); n != 2 {
		t.Errorf("%d data rows, want 2: the old ex-date stays", n)
	}
	got, err := d.Dividends(ctx, divInstrument, "2026-11-09", "2026-11-10")
	if err != nil || len(got) != 2 || got[0].NextExDate.Format("2006-01-02") != "2026-11-14" ||
		got[1].NextExDate.Format("2006-01-02") != "2026-11-21" {
		t.Errorf("rows = %+v, %v", got, err)
	}
}

func TestEventsRoleCannotDeleteOrUpdateKey(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	seedInstrument(t, name, divInstrument)
	upsertOne(t, openEarnings(t, name, "ot_w_events"), listedRow("jquants", "2026-11-20", nil))
	if _, err := openDividends(t, name, "ot_w_events").UpsertDividends(context.Background(),
		[]contracts.DividendExpectation{divRow("2026-11-09", 1, "present")}, "test"); err != nil {
		t.Fatal(err)
	}
	c := connectAs(t, name, "ot_w_events")
	for _, q := range []string{
		`DELETE FROM obs.earnings_date`, `DELETE FROM obs.earnings_check`, `DELETE FROM obs.dividend_expectation`,
		`DELETE FROM obs.dividend_check`,
		`UPDATE obs.earnings_date SET issuer = 'x'`, `UPDATE obs.earnings_date SET period_key = DATE '2026-01-01'`,
		`UPDATE obs.earnings_date SET received_at = now()`, `UPDATE obs.earnings_check SET issuer = 'x'`,
		`UPDATE obs.earnings_check SET observed_on = DATE '2026-01-01'`,
		`UPDATE obs.dividend_expectation SET instrument_id = 1`, `UPDATE obs.dividend_expectation SET ex_date = DATE '2026-01-01'`,
		`UPDATE obs.dividend_check SET instrument_id = 1`, `UPDATE obs.dividend_check SET session_day = DATE '2026-01-01'`,
		`TRUNCATE obs.earnings_date`,
	} {
		if _, err := c.Exec(context.Background(), q); err == nil {
			t.Errorf("ot_w_events ran %q", q)
		}
	}
}

func TestPagesRoleReadsBothViews(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	seedInstrument(t, name, divInstrument)
	w := openEarnings(t, name, "ot_w_events")
	upsertOne(t, w, listedRow("jquants", "2026-11-20", nil))
	wd := openDividends(t, name, "ot_w_events")
	if _, err := wd.UpsertDividends(context.Background(), []contracts.DividendExpectation{divRow("2026-11-09", 1, "present")}, "test"); err != nil {
		t.Fatal(err)
	}
	p := openEarnings(t, name, "ot_ro_pages")
	if got := read(t, p, "2026-11-09", "2026-11-09"); len(got) != 1 || got[0].ReportDate == nil {
		t.Errorf("earnings as ot_ro_pages = %+v", got)
	}
	pd := openDividends(t, name, "ot_ro_pages")
	got, err := pd.Dividends(context.Background(), divInstrument, "2026-11-09", "2026-11-09")
	if err != nil || len(got) != 1 || got[0].NextAmount == nil {
		t.Errorf("dividends as ot_ro_pages = %+v, %v", got, err)
	}
}

// oldRows are the rows of the tables as 0020 and 0023 made them, written before 0046 is applied.
func oldEarnings(t *testing.T, name, observed string, attempt int, report, estimate string) {
	t.Helper()
	execAdmin(t, name, `INSERT INTO obs.earnings_date (observed_on, issuer, attempt, symbol, market, status, report_date,
 fiscal_period_end, time_of_day, estimate, error_code, source, received_at)
VALUES ($1::text::date, $2, $3, 'AAPL', 'US', 'listed', $4::text::date, NULL, '', $5, '', 'jquants', now())`,
		observed, issuer, attempt, report, estimate)
}

func oldDividend(t *testing.T, name string, day string, attempt, instrument int, amount string) {
	t.Helper()
	execAdmin(t, name, `INSERT INTO obs.dividend_expectation (session_day, instrument_id, attempt, issuer, status, past_12m,
 next_12m, next_amount, next_ex_date, currency, error_code, source, received_at)
VALUES ($1::text::date, $2, $3, $4, 'present', '1.00', '1.00', $5, DATE '2026-11-14', 'USD', '', 'ibkr_tick_456', now())`,
		day, instrument, attempt, issuer, amount)
}

func TestMigrationKeepsNewestPerKey(t *testing.T) {
	t.Parallel()
	name := newDatabaseWith(t, keysMigrations[:len(keysMigrations)-1])
	seedInstrument(t, name, divInstrument)
	oldEarnings(t, name, "2026-11-01", 1, "2026-11-20", "1.00")
	oldEarnings(t, name, "2026-11-02", 1, "2026-11-20", "2.00")
	oldEarnings(t, name, "2026-11-02", 2, "2026-11-20", "3.00")
	oldDividend(t, name, "2026-11-01", 1, divInstrument, "0.20")
	oldDividend(t, name, "2026-11-02", 1, divInstrument, "0.25")
	oldDividend(t, name, "2026-11-02", 2, divInstrument, "0.30")
	applyMigration(t, name, "0046_fundamentals_keys.sql")
	var est, amount string
	ctx := context.Background()
	c := pgxAs(t, name)
	if err := c.QueryRow(ctx, `SELECT estimate FROM obs.earnings_date`).Scan(&est); err != nil || est != "3.00" {
		t.Errorf("estimate = %q, %v; want the newest row of the key", est, err)
	}
	if err := c.QueryRow(ctx, `SELECT amount FROM obs.dividend_expectation`).Scan(&amount); err != nil || amount != "0.30" {
		t.Errorf("amount = %q, %v; want the newest row of the key", amount, err)
	}
	if n := tableCount(t, name, "obs.earnings_date"); n != 1 {
		t.Errorf("%d earnings rows, want 1", n)
	}
	if n := tableCount(t, name, "obs.dividend_expectation"); n != 1 {
		t.Errorf("%d dividend rows, want 1", n)
	}
	if n := tableCount(t, name, "obs.earnings_check") + tableCount(t, name, "obs.dividend_check"); n != 4 {
		t.Errorf("%d answers, want one per day and key: 4", n)
	}
}

func TestMigrationUnknownInstrumentAnswerBecomesNoValue(t *testing.T) {
	t.Parallel()
	name := newDatabaseWith(t, keysMigrations[:len(keysMigrations)-1])
	oldDividend(t, name, "2026-11-02", 1, 999, "0.30")
	applyMigration(t, name, "0046_fundamentals_keys.sql")
	var status string
	var exDate *string
	err := pgxAs(t, name).QueryRow(context.Background(),
		`SELECT status, ex_date::text FROM obs.dividend_check WHERE instrument_id = 999`).Scan(&status, &exDate)
	if err != nil || status != "no_value" || exDate != nil {
		t.Errorf("answer = %q, %v, %v; want no_value without an ex-date", status, exDate, err)
	}
	if n := tableCount(t, name, "obs.dividend_expectation"); n != 0 {
		t.Errorf("%d data rows, want none for an unknown instrument", n)
	}
	got, err := openDividends(t, name, "ot_ro_pages").Dividends(context.Background(), 999, "2026-11-02", "2026-11-02")
	if err != nil || len(got) != 1 || got[0].Status != "no_value" {
		t.Errorf("view = %+v, %v", got, err)
	}
}
