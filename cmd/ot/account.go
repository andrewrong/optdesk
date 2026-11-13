package main

import (
	"context"
	"fmt"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/broker/grpcbroker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/accountcapture"
	"github.com/andrewrong/optdesk/internal/store"
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

// accountCapture builds account_capture from the lines of .env. An unset OT_ACCOUNT_SOCKETS refuses the job
// like a missing accounts.toml (Decision 8). Every check comes before a socket is
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
	book, err := a.loadBook()
	if err != nil {
		return nil, err
	}
	dsn := env[store.DSNEnvAccount]
	if dsn == "" {
		return nil, fmt.Errorf("%s is not set", store.DSNEnvAccount)
	}
	socketReaders := make([]broker.AccountReader, len(sockets))
	for i, sock := range sockets {
		r, err := f.d.dialAccount(ctx, sock, broker.Options{})
		if err != nil {
			a.log.Warn("dial account failed", "component", "ot", "job", "account_capture", "socket", i, "status", "unreachable")
			r = downReader{err: err}
		}
		extra.addCloser(r)
		socketReaders[i] = r
	}
	readers := newGatewayResolver(book, socketReaders).Readers()
	for _, al := range statementAliases(book) {
		delete(readers, al)
	}
	w, err := f.d.openAccount(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open account store with %s failed", store.DSNEnvAccount)
	}
	extra.addCloser(w)
	j, err := f.d.newAccountCapture(accountcapture.Config{Main: book.Main(), Readers: readers, Writer: w, Clock: f.clk})
	if err != nil {
		return nil, fmt.Errorf("build account_capture job failed: %w", err)
	}
	return j, nil
}
