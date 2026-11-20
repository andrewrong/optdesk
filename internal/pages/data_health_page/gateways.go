// gateways.go maps the gateway inputs to the gateways array of the page (docs/design/gateway-health.md,
// Decisions 10 and 12). The builder computes no number: the states are pyside's.
package data_health_page

// GatewayInput is one gateway as the page reads it; State is that of broker.Health and Answered is false when
// the reader failed.
type GatewayInput struct {
	Name     string
	Answered bool
	State    string
	Farms    []GatewayFarm
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
}

// GatewayFarmVM is one farm of a gateway.
type GatewayFarmVM struct {
	Name string `json:"name"`
	Up   bool   `json:"up"`
}

// gatewayVerdict maps the fields of one input to a verdict key. A state the page does not know (empty, or the
// SESSION_STATE_UNSPECIFIED of an older pyside) is never session_up: it gives unreachable.
func gatewayVerdict(g GatewayInput) string {
	if !g.Answered {
		return verdictUnreachable
	}
	switch g.State {
	case "farm_broken":
		return verdictFarmDown
	case "soft_lost", "down":
		return verdictSessionRetrying
	case "up":
		return verdictSessionUp
	}
	return verdictUnreachable
}

var gatewayTexts = map[string]string{
	verdictSessionUp:       zhGatewaySessionUp,
	verdictSessionRetrying: zhGatewayRetrying,
	verdictFarmDown:        zhGatewayFarmDown,
	verdictUnreachable:     zhGatewayUnreachable,
}

// gatewayRows returns the gateways array, or nil (the key is omitted) when there is no gateway.
func gatewayRows(in []GatewayInput) []GatewayVM {
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
		out = append(out, GatewayVM{Name: g.Name, Verdict: v, Farms: farms, Note: gatewayTexts[v]})
	}
	return out
}
