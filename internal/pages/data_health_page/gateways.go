// gateways.go maps the gateway inputs to the gateways array of the page (docs/design/gateway-health.md,
// Decisions 10 and 12). The builder computes no number: the states are pyside's.
package data_health_page

import "time"

// GatewayInput is one gateway as the page reads it; State is that of broker.Health and Answered is false when
// the reader failed.
type GatewayInput struct {
	Name     string
	Answered bool
	State    string
	Farms    []GatewayFarm
	// Since is the time the stored no-connection value began; zero when there is no row
	// (docs/design/gateway-down-since.md, Decision 5).
	Since time.Time
}

// GatewayFarm is one data farm the gateway named.
type GatewayFarm struct {
	Name string
	Up   bool
}

const (
	verdictSessionUp       = "session_up"
	verdictSessionRetrying = "session_retrying"
	verdictFarmDown        = "farm_down"
	verdictUnreachable     = "unreachable"
)

// GatewayVM is one element of the gateways array.
type GatewayVM struct {
	Name    string          `json:"name"`
	Verdict string          `json:"verdict"`
	Farms   []GatewayFarmVM `json:"farms"`
	Note    string          `json:"note"`
	// Since and SinceText are set only while the connection is lost now (Decision 6).
	Since     string `json:"since,omitempty"`
	SinceText string `json:"since_text,omitempty"`
}

// GatewayFarmVM is one farm of a gateway.
type GatewayFarmVM struct {
	Name string `json:"name"`
	Up   bool   `json:"up"`
}

// ConnectionLost reports whether a gateway has no API connection: no answer, or the state soft_lost or down.
// up and farm_broken are false (the session is up). An unknown state is true, as gatewayVerdict treats it as
// unreachable (docs/design/gateway-down-since.md, Decision 3).
func ConnectionLost(answered bool, state string) bool {
	if !answered {
		return true
	}
	switch state {
	case "up", "farm_broken":
		return false
	}
	return true
}

// gatewayVerdict maps the fields of one input to a verdict key. A state the page does not know (empty, or the
// SESSION_STATE_UNSPECIFIED of an older pyside) is never session_up: it gives unreachable.
func gatewayVerdict(g GatewayInput) string {
	if ConnectionLost(g.Answered, g.State) {
		if g.Answered && (g.State == "soft_lost" || g.State == "down") {
			return verdictSessionRetrying
		}
		return verdictUnreachable
	}
	if g.State == "farm_broken" {
		return verdictFarmDown
	}
	return verdictSessionUp
}

var gatewayTexts = map[string]string{
	verdictSessionUp:       zhGatewaySessionUp,
	verdictSessionRetrying: zhGatewayRetrying,
	verdictFarmDown:        zhGatewayFarmDown,
	verdictUnreachable:     zhGatewayUnreachable,
}

// gatewayRows returns the gateways array, or nil (the key is omitted) when there is no gateway.
func gatewayRows(now time.Time, in []GatewayInput) []GatewayVM {
	if len(in) == 0 {
		return nil
	}
	out := make([]GatewayVM, 0, len(in))
	for _, g := range in {
		farms := make([]GatewayFarmVM, 0, len(g.Farms))
		for _, f := range g.Farms {
			farms = append(farms, GatewayFarmVM(f))
		}
		v := gatewayVerdict(g)
		vm := GatewayVM{Name: g.Name, Verdict: v, Farms: farms, Note: gatewayTexts[v]}
		if ConnectionLost(g.Answered, g.State) && !g.Since.IsZero() {
			vm.Since = g.Since.In(now.Location()).Format(timeLayout)
			vm.SinceText = zhNoConnectionPrefix + vm.Since
		}
		out = append(out, vm)
	}
	return out
}
