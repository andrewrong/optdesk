package shared

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// quoteColumns are the columns of obs.option_day that a missing value must never blank.
var quoteColumns = []string{
	"bid", "ask", "bid_size", "ask_size", "last", "volume", "underlying_price", "iv_bid_decimal", "iv_mid_decimal",
	"iv_ask_decimal", "iv_decimal", "delta", "gamma", "theta_per_calendar_day", "vega_per_vol_point", "open_interest",
	"oi_asof_date",
}

func TestUpsertDaySQLPlaceholders(t *testing.T) {
	t.Parallel()
	max := 0
	for _, m := range regexp.MustCompile(`\$(\d+)`).FindAllStringSubmatch(UpsertDaySQL, -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatal(err)
		}
		if n > max {
			max = n
		}
	}
	if max != DayArgCount {
		t.Fatalf("highest placeholder is $%d, DayArgCount is %d", max, DayArgCount)
	}
	for i := 1; i <= DayArgCount; i++ {
		if !strings.Contains(UpsertDaySQL, "$"+strconv.Itoa(i)+",") && !strings.Contains(UpsertDaySQL, "$"+strconv.Itoa(i)+")") {
			t.Errorf("placeholder $%d is not used", i)
		}
	}
}

func TestUpsertDaySQLNeverBlanksAQuote(t *testing.T) {
	t.Parallel()
	sql := UpsertDaySQL
	for _, want := range []string{"ON CONFLICT (instrument_id, session_day) DO UPDATE SET", "EXCLUDED.received_at >= "} {
		if !strings.Contains(sql, want) {
			t.Errorf("statement lacks %q", want)
		}
	}
	set := sql[strings.Index(sql, "DO UPDATE SET"):strings.Index(sql, " WHERE ")]
	for _, c := range quoteColumns {
		want := c + " = COALESCE(EXCLUDED." + c + ", option_day." + c + ")"
		if !strings.Contains(set, want) {
			t.Errorf("SET list lacks %q", want)
		}
	}
	for _, part := range strings.Split(set, ",\n") {
		for _, c := range quoteColumns {
			if strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(part, "DO UPDATE SET")), c+" =") &&
				!strings.Contains(part, "COALESCE(EXCLUDED."+c+", option_day."+c+")") {
				t.Errorf("quote column %s is set without COALESCE: %q", c, part)
			}
		}
	}
}
