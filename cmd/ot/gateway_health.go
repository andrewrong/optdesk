package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/broker/grpcbroker"
	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/notify/core"
	"github.com/andrewrong/optdesk/internal/pages/data_health_page"
	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/gateway-health.md#9-gateway-health-wire.

const (
	// envGatewaySockets is the line of .env that names the gateway sockets: a comma list of alias=socket path, such
	// as main=<path>,wife=<path>. An unset line means no gateway: the page has no gateways array and no pass runs.
	envGatewaySockets = "OT_GATEWAY_SOCKETS"
	// extGateway is the name of the gateway deps in the ext deps.
	extGateway = "gateway_health"
	// gatewayHealthDeadline is the deadline of the whole read of one gateway (dial and Health).
	gatewayHealthDeadline = 2 * time.Second
)

// gatewayDeps are the constructors of the gateway wire, in the ext deps so that wire.go holds nothing of it.
type gatewayDeps struct {
	newHealth   broker.NewHealthFunc
	newNotifier notify.NewGatewayNotifierFunc
}

func init() {
	extDeps[extGateway] = func() any { return gatewayDeps{newHealth: dialHealth, newNotifier: core.NewGateways} }
}

// dialHealth adapts grpcbroker.DialHealth to broker.NewHealthFunc.
func dialHealth(ctx context.Context, socketPath string, o broker.Options) (broker.HealthReader, error) {
	c, err := grpcbroker.DialHealth(ctx, socketPath, o)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// gatewayEntry is one gateway of the line: its alias and the socket of its pyside.
type gatewayEntry struct{ alias, socket string }

// parseGatewaySockets parses the comma list of alias=socket path. An alias is lower-case letters and digits (it is
// part of a mark key, which holds no underscore) and appears once; a socket path stays under maxSocketPath.
func parseGatewaySockets(v string) ([]gatewayEntry, error) {
	bad := fmt.Errorf("%s is not a comma list of alias=socket path", envGatewaySockets)
	var out []gatewayEntry
	for _, part := range strings.Split(v, ",") {
		alias, sock, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || !validGatewayAlias(alias) || sock == "" || len(sock) >= maxSocketPath {
			return nil, bad
		}
		for _, e := range out {
			if e.alias == alias {
				return nil, bad
			}
		}
		out = append(out, gatewayEntry{alias, sock})
	}
	return out, nil
}

func validGatewayAlias(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

// gatewayReading is what one gateway answered: Answered is false when the dial or the call failed.
type gatewayReading struct {
	alias    string
	answered bool
	health   broker.Health
}

// gatewaySource reads the health of the configured gateways. It keeps nothing between reads.
type gatewaySource struct {
	entries []gatewayEntry
	dial    broker.NewHealthFunc
}

// gatewaySourceOf reads the line and the deps. It gives an empty source (no gateway) when the line is unset, the
// deps carry no dialer, or the line is malformed (one log line, no value).
func gatewaySourceOf(a *app) (gatewaySource, gatewayDeps) {
	d, ok := a.deps.ext[extGateway].(gatewayDeps)
	if !ok || d.newHealth == nil {
		return gatewaySource{}, d
	}
	env, err := a.envFile()
	if err != nil || env[envGatewaySockets] == "" {
		return gatewaySource{}, d
	}
	entries, err := parseGatewaySockets(env[envGatewaySockets])
	if err != nil {
		a.log.Warn("gateway sockets ignored", "component", "ot", "key", envGatewaySockets, "status", "malformed")
		return gatewaySource{}, d
	}
	return gatewaySource{entries: entries, dial: d.newHealth}, d
}

// read asks every gateway at once, each under its own deadline, and returns the readings in the order of the line.
func (s gatewaySource) read(ctx context.Context) []gatewayReading {
	out := make([]gatewayReading, len(s.entries))
	var wg sync.WaitGroup
	for i, e := range s.entries {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = s.readOne(ctx, e)
		}()
	}
	wg.Wait()
	return out
}

func (s gatewaySource) readOne(ctx context.Context, e gatewayEntry) gatewayReading {
	ctx, cancel := context.WithTimeout(ctx, gatewayHealthDeadline)
	defer cancel()
	r, err := s.dial(ctx, e.socket, broker.Options{})
	if err != nil || r == nil {
		return gatewayReading{alias: e.alias}
	}
	defer func() { _ = r.Close() }()
	h, err := r.Health(ctx)
	if err != nil {
		return gatewayReading{alias: e.alias}
	}
	h.Name = e.alias
	return gatewayReading{alias: e.alias, answered: true, health: h}
}

// pageGateways is the Gateways of the page input: one entry per gateway, by alias. A rule of the verdict is
// the builder's; this copies the fields.
func pageGateways(rs []gatewayReading) []data_health_page.GatewayInput {
	if len(rs) == 0 {
		return nil
	}
	out := make([]data_health_page.GatewayInput, 0, len(rs))
	for _, r := range rs {
		g := data_health_page.GatewayInput{Name: r.alias, Answered: r.answered, State: r.health.State}
		for _, f := range r.health.Farms {
			g.Farms = append(g.Farms, data_health_page.GatewayFarm{Name: f.Name, Up: f.Up})
		}
		out = append(out, g)
	}
	return out
}

// gatewayFacts is one fact per reading, built by notify.FactOf.
func gatewayFacts(rs []gatewayReading) []notify.GatewayFact {
	out := make([]notify.GatewayFact, 0, len(rs))
	for _, r := range rs {
		farms := make([]notify.GatewayFarm, 0, len(r.health.Farms))
		for _, f := range r.health.Farms {
			farms = append(farms, notify.GatewayFarm{Name: f.Name, Up: f.Up, Since: f.Since})
		}
		out = append(out, notify.FactOf(r.alias, r.answered, r.health.State, farms))
	}
	return out
}

// gatewayPasser is the gateway pass of the scheduler tick.
type gatewayPasser interface {
	pass(ctx context.Context) (int, error)
}

// gatewayPass reads the gateways and hands one fact per gateway to the notifier, on every call. It keeps no
// previous verdict: the notifier and its marks decide what is sent.
type gatewayPass struct {
	src gatewaySource
	n   notify.GatewayNotifier
}

func (p gatewayPass) pass(ctx context.Context) (int, error) {
	return p.n.NotifyGateways(ctx, gatewayFacts(p.src.read(ctx)))
}

// newGatewayPass builds the pass over the sent store and the sender. It is nil (no pass) when no gateway is
// configured, the deps carry no constructor, or the build fails (one log line, no error text).
func newGatewayPass(a *app, sent store.SentStore, s notify.Sender, clk clockwork.Clock) gatewayPasser {
	src, d := gatewaySourceOf(a)
	if len(src.entries) == 0 || d.newNotifier == nil {
		return nil
	}
	n, err := d.newNotifier(s, sent, clk)
	if err != nil {
		a.log.Warn("gateway notifier not built", "component", "ot", "status", "failed")
		return nil
	}
	return gatewayPass{src: src, n: n}
}
