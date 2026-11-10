package contracts_test

import "testing"

func openRow() map[string]any {
	return map[string]any{
		"session_day": "2026-11-11", "capture_id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8", "slot": "0945",
		"instrument_id": 1001, "underlying_id": 7, "plan_id": "6ba7b811-9dad-11d1-80b4-00c04fd430c8",
		"generation": 1, "market_data_type": 1, "requested_at": "2026-11-11T14:45:00Z",
		"sent_at": "2026-11-11T14:45:01Z", "received_at": "2026-11-11T14:45:02Z", "event_ts": nil,
		"connection_generation": 3, "status": 1, "error_code": nil, "skew": false, "outside_band": false,
		"currency": "USD", "bid": 1.0, "ask": 1.1, "bid_size": 3, "ask_size": 4, "last": nil, "volume": nil,
		"underlying_price": 100.0, "underlying_ccy": "USD", "underlying_event_ts": nil,
		"iv_bid_decimal": nil, "iv_mid_decimal": 0.3, "iv_ask_decimal": nil, "iv_decimal": 0.3,
		"delta": -0.2, "gamma": 0.01, "theta_per_calendar_day": -0.02, "vega_per_vol_point": 0.1,
	}
}

func planRow() map[string]any {
	return map[string]any{
		"session_day": "2026-11-11", "slot": "0945", "plan_id": "6ba7b811-9dad-11d1-80b4-00c04fd430c8",
		"instrument_id": 1001, "underlying_id": 7, "planned_at": "2026-11-11T13:45:00Z",
		"class": "puts", "deferred": false, "rank": 12, "planned_spot": 100.5,
		"sigma": 0.3, "sigma_source": "prior_close",
	}
}

func notListedRow() map[string]any {
	return map[string]any{
		"underlying_id": 7, "expiry": "2026-12-18", "strike": "95", "right": "P", "trading_class": "AAPL",
		"checked_at": "2026-11-10T22:00:00Z", "retry_at": "2026-11-17T22:00:00Z",
		"error_code": nil, "error_text": nil,
	}
}

func TestOptionOpenDatasetsAcceptAndReject(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		row  func() map[string]any
		must []string
	}{
		{"obs.option_open", openRow, []string{"capture_id", "slot", "plan_id"}},
		{"plan.option_open_item", planRow, []string{"slot", "plan_id"}},
		{"master.option_not_listed", notListedRow, []string{"retry_at"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := compileDataset(t, c.name)
			if err := s.Validate(c.row()); err != nil {
				t.Fatalf("base row rejected: %v", err)
			}
			for _, k := range c.must {
				m := c.row()
				delete(m, k)
				if s.Validate(m) == nil {
					t.Errorf("row without %s accepted", k)
				}
			}
			m := c.row()
			m["extra"] = 1
			if s.Validate(m) == nil {
				t.Error("extra property accepted")
			}
		})
	}
}

func TestOptionOpenStatusAndClassEnums(t *testing.T) {
	t.Parallel()
	so := compileDataset(t, "obs.option_open")
	for _, st := range []int{0, 10} {
		m := openRow()
		m["status"] = st
		if so.Validate(m) == nil {
			t.Errorf("status %d accepted", st)
		}
	}
	m := openRow()
	m["status"] = 9
	if err := so.Validate(m); err != nil {
		t.Errorf("status 9: %v", err)
	}
	sp := compileDataset(t, "plan.option_open_item")
	for _, cl := range []string{"near_spot", "round"} {
		m := planRow()
		m["class"] = cl
		if sp.Validate(m) == nil {
			t.Errorf("class %s accepted", cl)
		}
	}
	for _, cl := range []string{"around_spot", "puts", "calls", "held"} {
		m := planRow()
		m["class"] = cl
		if err := sp.Validate(m); err != nil {
			t.Errorf("class %s: %v", cl, err)
		}
	}
}
