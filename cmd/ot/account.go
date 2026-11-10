package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/broker/grpcbroker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/accountcapture"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/pelletier/go-toml/v2"
)

const jobAccountCapture = "account_capture"

func init() {
	registerJob(jobFamily{
		id: jobAccountCapture, order: 100,
		build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			j, err := a.accountCapture(ctx, *jc.f, jc.extra)
			return j, "", err
		},
	})
}

// The lines of .env that account_capture reads (through deps.readEnv, never the process environment). Their
// values are never printed: an error names the key only.
const (
	envAccountSockets = "OT_ACCOUNT_SOCKETS"
	envIbkrGateways   = "OT_IBKR_GATEWAYS"
)

// dialAccountFunc dials the account service on one socket (grpcbroker.DialAccount).
type dialAccountFunc func(ctx context.Context, socketPath string, o broker.Options) (broker.AccountReader, error)

// newAccountCaptureFunc is the constructor of the account_capture job (accountcapture.New).
type newAccountCaptureFunc func(cfg accountcapture.Config) (contracts.Job, error)

// dialAccountClient adapts grpcbroker.DialAccount to dialAccountFunc.
func dialAccountClient(ctx context.Context, socketPath string, o broker.Options) (broker.AccountReader, error) {
	c, err := grpcbroker.DialAccount(ctx, socketPath, o)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// downReader answers every call with the error of the failed dial, so that a gateway that is down is a
// failed read of its alias (degraded when it is not main) and not a refusal of the whole job.
type downReader struct{ err error }

func (r downReader) Account(context.Context) (broker.Account, error) { return broker.Account{}, r.err }
func (r downReader) Executions(context.Context) (broker.Executions, error) {
	return broker.Executions{}, r.err
}
func (downReader) Close() error { return nil }

// parseAccountSockets parses the comma list of label=socket path.
func parseAccountSockets(v string) (map[string]string, error) {
	out := map[string]string{}
	for _, part := range strings.Split(v, ",") {
		label, path, ok := strings.Cut(strings.TrimSpace(part), "=")
		label, path = strings.TrimSpace(label), strings.TrimSpace(path)
		if !ok || label == "" || path == "" || len(path) >= maxSocketPath {
			return nil, fmt.Errorf("%s is not a comma list of label=socket path", envAccountSockets)
		}
		if _, dup := out[label]; dup {
			return nil, fmt.Errorf("%s repeats a label", envAccountSockets)
		}
		out[label] = path
	}
	return out, nil
}

// accountMain reads the gateways file at path and returns the `account` field of its one required entry.
func accountMain(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("%s is not set", envIbkrGateways)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("%s names a file that cannot be read", envIbkrGateways)
	}
	var f struct {
		Gateway []struct {
			Account  string `toml:"account"`
			Required bool   `toml:"required"`
		} `toml:"gateway"`
	}
	if err := toml.Unmarshal(raw, &f); err != nil {
		return "", fmt.Errorf("%s names a file that is not valid", envIbkrGateways)
	}
	main, n := "", 0
	for _, g := range f.Gateway {
		if g.Required {
			main, n = g.Account, n+1
		}
	}
	if n != 1 || main == "" {
		return "", fmt.Errorf("%s needs exactly one required entry with an account", envIbkrGateways)
	}
	return main, nil
}

// accountCapture builds account_capture from the lines of .env. An unset OT_ACCOUNT_SOCKETS refuses the job
// like a file with no required entry (Decision 8). Every check comes before a socket is
// dialed or the store is opened; the job is nil only for a deps without newAccountCapture; what it opens goes to extra, which withWiring closes.
func (a *app) accountCapture(ctx context.Context, f jobFactory, extra *closerSet) (contracts.Job, error) {
	if f.d.newAccountCapture == nil {
		// A deps literal of a test that does not exercise account_capture; newDeps always sets it.
		return nil, nil
	}
	env, err := a.envFile()
	if err != nil {
		return nil, err
	}
	if env[envAccountSockets] == "" {
		return nil, fmt.Errorf("%s is not set", envAccountSockets)
	}
	sockets, err := parseAccountSockets(env[envAccountSockets])
	if err != nil {
		return nil, err
	}
	main, err := accountMain(env[envIbkrGateways])
	if err != nil {
		return nil, err
	}
	if _, ok := sockets[main]; !ok {
		return nil, fmt.Errorf("the main label of %s is not in %s", envIbkrGateways, envAccountSockets)
	}
	dsn := env[store.DSNEnvAccount]
	if dsn == "" {
		return nil, fmt.Errorf("%s is not set", store.DSNEnvAccount)
	}
	labels := make([]string, 0, len(sockets))
	for l := range sockets {
		labels = append(labels, l)
	}
	sort.Strings(labels)
	readers := make(map[string]broker.AccountReader, len(sockets))
	for _, l := range labels {
		r, err := f.d.dialAccount(ctx, sockets[l], broker.Options{})
		if err != nil {
			a.log.Warn("dial account failed", "component", "ot", "job", "account_capture", "alias", l, "status", "unreachable")
			r = downReader{err: err}
		}
		extra.addCloser(r)
		readers[l] = r
	}
	w, err := f.d.openAccount(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open account store with %s failed", store.DSNEnvAccount)
	}
	extra.addCloser(w)
	j, err := f.d.newAccountCapture(accountcapture.Config{Main: main, Readers: readers, Writer: w, Clock: f.clk})
	if err != nil {
		return nil, fmt.Errorf("build account_capture job failed: %w", err)
	}
	return j, nil
}
