package data_health_page

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"
)

var sinceNow = time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)

func sinceGateway(t *testing.T, g GatewayInput) (GatewayVM, map[string]json.RawMessage) {
	t.Helper()
	rows := gatewayRows(sinceNow, []GatewayInput{g})
	b, err := json.Marshal(rows[0])
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	return rows[0], raw
}

func TestConnectionLostTable(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		answered bool
		state    string
		want     bool
	}{
		{false, "up", true},
		{false, "", true},
		{true, "soft_lost", true},
		{true, "down", true},
		{true, "up", false},
		{true, "farm_broken", false},
		{true, "", true},
		{true, "SESSION_STATE_UNSPECIFIED", true},
	} {
		if got := ConnectionLost(c.answered, c.state); got != c.want {
			t.Errorf("ConnectionLost(%v, %q) = %v, want %v", c.answered, c.state, got, c.want)
		}
	}
}

func TestSinceShownOnlyWhenLostNow(t *testing.T) {
	t.Parallel()
	since := sinceNow.Add(-6 * 24 * time.Hour)
	for _, st := range []string{"up", "farm_broken"} {
		vm, _ := sinceGateway(t, GatewayInput{Name: "a", Answered: true, State: st, Since: since})
		if vm.Since != "" || vm.SinceText != "" {
			t.Errorf("state %q: since %q since_text %q", st, vm.Since, vm.SinceText)
		}
	}
}

func TestSinceTextPrefix(t *testing.T) {
	t.Parallel()
	since := time.Date(2026, 11, 14, 3, 4, 5, 0, time.UTC)
	vm, _ := sinceGateway(t, GatewayInput{Name: "a", Answered: true, State: "down", Since: since})
	if want := "无连接自 " + since.Format(timeLayout); vm.SinceText != want {
		t.Errorf("since_text %q, want %q", vm.SinceText, want)
	}
	if !strings.HasPrefix(vm.SinceText, "无连接自 ") {
		t.Errorf("prefix: %q", vm.SinceText)
	}
	if vm.Since != since.Format(timeLayout) {
		t.Errorf("since %q", vm.Since)
	}
}

func TestNoSinceWithoutRow(t *testing.T) {
	t.Parallel()
	vm, raw := sinceGateway(t, GatewayInput{Name: "a", Answered: false})
	if vm.Since != "" || vm.SinceText != "" {
		t.Errorf("since %q since_text %q", vm.Since, vm.SinceText)
	}
	for _, k := range []string{"since", "since_text"} {
		if _, ok := raw[k]; ok {
			t.Errorf("key %q present", k)
		}
	}
}

func TestSinceTextHasNoIdOrPort(t *testing.T) {
	t.Parallel()
	vm, _ := sinceGateway(t, GatewayInput{Name: "a", Answered: true, State: "soft_lost", Since: sinceNow.Add(-time.Hour)})
	if vm.SinceText == "" {
		t.Fatal("no since_text")
	}
	if regexp.MustCompile(`\b[A-Z]{1,2}\d{5,}\b|:\d{4,5}\b`).MatchString(vm.SinceText) {
		t.Errorf("id or port in %q", vm.SinceText)
	}
}
