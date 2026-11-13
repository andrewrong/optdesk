// Package jquants is the J-Quants V2 earnings-date source: one GET per symbol over an injected HTTP
// client, the key only in the x-api-key header, no redirect. Calls are spaced callGap apart (the Free plan
// allows 5 a minute), a 429 is asked once more after retryWait, and the first failure stops the remaining
// calls. Both waits go through EarningsConfig.Sleep.
package jquants

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
)

const (
	// maxBody caps the bytes read from a response.
	maxBody = 4 << 20
	// callGap is the wait before every call after the first of a Fetch: at most 5 calls in any 60 s.
	callGap = 12 * time.Second
	// retryWait is the wait before the one retry of a 429 (J-Quants sends no Retry-After).
	retryWait = 61 * time.Second
)

// realSleep waits d or until ctx is done, in which case it returns ctx.Err().
func realSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type codedError struct {
	kind contracts.ErrorKind
	code string
	msg  string
	err  error
}

func (e *codedError) Error() string             { return e.msg }
func (e *codedError) Kind() contracts.ErrorKind { return e.kind }
func (e *codedError) ErrorCode() string         { return e.code }
func (e *codedError) Unwrap() error             { return e.err }

func fail(kind contracts.ErrorKind, code, msg string) error {
	return &codedError{kind: kind, code: code, msg: msg}
}

type client struct {
	base    string
	key     string
	symbols []string
	doer    sources.HTTPDoer
	sleep   func(ctx context.Context, d time.Duration) error
}

var _ sources.NewJQuantsFunc = New

// New builds the J-Quants source. BaseURL and APIKey are required; no environment is read.
func New(cfg sources.EarningsConfig) (sources.EarningsSource, error) {
	if cfg.BaseURL == "" || cfg.APIKey == "" {
		return nil, fail(contracts.Permanent, "bad_config", "jquants: BaseURL and APIKey are required")
	}
	d := cfg.Doer
	if d == nil {
		d = &http.Client{
			Timeout:       30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	}
	sleep := cfg.Sleep
	if sleep == nil {
		sleep = realSleep
	}
	return &client{base: strings.TrimRight(cfg.BaseURL, "/"), key: cfg.APIKey, symbols: cfg.Symbols, doer: d, sleep: sleep}, nil
}

// wait runs the injected wait; its error is reported as the network code.
func (c *client) wait(ctx context.Context, d time.Duration) error {
	if err := c.sleep(ctx, d); err != nil {
		return &codedError{kind: contracts.Retryable, code: "network", msg: "jquants: request cancelled", err: err}
	}
	return nil
}

// Fetch makes one call per symbol, callGap apart, and returns every dated row of every company. A 429 is
// asked once more after retryWait (see one). The first failure
// returns at once: no further call.
func (c *client) Fetch(ctx context.Context) ([]sources.EarningsListing, error) {
	var all []sources.EarningsListing
	for i, sym := range c.symbols {
		if i > 0 {
			if err := c.wait(ctx, callGap); err != nil {
				return nil, err
			}
		}
		rows, err := c.one(ctx, sym)
		if err != nil {
			return nil, err
		}
		all = append(all, rows...)
	}
	return all, nil
}

// one asks for a symbol; a 429 waits retryWait and asks once more, a second 429 is rate_limited.
func (c *client) one(ctx context.Context, sym string) ([]sources.EarningsListing, error) {
	rows, status, err := c.get(ctx, sym)
	if status != http.StatusTooManyRequests {
		return rows, err
	}
	if werr := c.wait(ctx, retryWait); werr != nil {
		return nil, werr
	}
	rows, _, err = c.get(ctx, sym)
	return rows, err
}

// get makes one call and returns the HTTP status (0 when none came) beside the result.
func (c *client) get(ctx context.Context, sym string) ([]sources.EarningsListing, int, error) {
	q := url.Values{}
	q.Set("code", sym+"0")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/v2/fins/earnings-date?"+q.Encode(), nil)
	if err != nil {
		return nil, 0, fail(contracts.Permanent, "bad_request", "jquants: bad request")
	}
	req.Header.Set("x-api-key", c.key)
	resp, err := c.doer.Do(req)
	if err != nil {
		// Report no text of the transport error.
		if ctx.Err() != nil {
			return nil, 0, &codedError{kind: contracts.Retryable, code: "network", msg: "jquants: request cancelled", err: ctx.Err()}
		}
		return nil, 0, fail(contracts.Retryable, "network", "jquants: request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, resp.StatusCode, fail(contracts.Retryable, "rate_limited", "jquants: status 429")
	case resp.StatusCode >= 500:
		return nil, 0, fail(contracts.Retryable, "http_status", fmt.Sprintf("jquants: status %d", resp.StatusCode))
	case resp.StatusCode != http.StatusOK:
		return nil, 0, fail(contracts.Permanent, "http_status", fmt.Sprintf("jquants: status %d", resp.StatusCode))
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, 0, fail(contracts.Retryable, "network", "jquants: reading response failed")
	}
	rows, err := parseBody(raw)
	return rows, resp.StatusCode, err
}
