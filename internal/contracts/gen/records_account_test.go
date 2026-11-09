package gen

import (
	"encoding/json"
	"testing"
)

func TestAccountPositionAcceptsDesignCaptureID(t *testing.T) {
	t.Parallel()
	var p AccountPosition
	raw := []byte(`{"capture_id":"a:2026-11-09:run1","alias":"a","captured_at":"2026-11-09T21:00:00Z",` +
		`"session_day":"2026-11-09","symbol":"SPY","sec_type":"option","exchange":"","currency":"USD",` +
		`"expiry":"20261120","strike":"500","multiplier":"100","right":"put","con_id":1,` +
		`"quantity":"-1","avg_cost":"1.5"}`)
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("design capture id refused: %v", err)
	}
}

func TestAccountValueAcceptsDesignCaptureID(t *testing.T) {
	t.Parallel()
	var v AccountValue
	raw := []byte(`{"capture_id":"a:2026-11-09:run1","alias":"a","captured_at":"2026-11-09T21:00:00Z",` +
		`"session_day":"2026-11-09","tag":"NetLiquidation","value":"100.5","currency":"USD"}`)
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("design capture id refused: %v", err)
	}
}
