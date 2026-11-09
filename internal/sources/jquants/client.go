// Package jquants is the J-Quants V2 earnings-date source: one GET per symbol over an injected HTTP
// client, the key only in the x-api-key header, no retry, no redirect, and the first failure stops the
// remaining calls.
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

// maxBody caps the bytes read from a response.
const maxBody = 4 << 20

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
	return &client{base: strings.TrimRight(cfg.BaseURL, "/"), key: cfg.APIKey, symbols: cfg.Symbols, doer: d}, nil
}

// Fetch makes one call per symbol and returns every dated row of every company. The first failure
// returns at once: no retry and no further call.
func (c *client) Fetch(ctx context.Context) ([]sources.EarningsListing, error) {
	var all []sources.EarningsListing
	for _, sym := range c.symbols {
		rows, err := c.one(ctx, sym)
		if err != nil {
			return nil, err
		}
		all = append(all, rows...)
	}
	return all, nil
}

func (c *client) one(ctx context.Context, sym string) ([]sources.EarningsListing, error) {
	q := url.Values{}
	q.Set("code", sym+"0")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/v2/fins/earnings-date?"+q.Encode(), nil)
	if err != nil {
		return nil, fail(contracts.Permanent, "bad_request", "jquants: bad request")
	}
	req.Header.Set("x-api-key", c.key)
	resp, err := c.doer.Do(req)
	if err != nil {
		// Report no text of the transport error.
		if ctx.Err() != nil {
			return nil, &codedError{kind: contracts.Retryable, code: "network", msg: "jquants: request cancelled", err: ctx.Err()}
		}
		return nil, fail(contracts.Retryable, "network", "jquants: request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, fail(contracts.Retryable, "rate_limited", "jquants: status 429")
	case resp.StatusCode >= 500:
		return nil, fail(contracts.Retryable, "http_status", fmt.Sprintf("jquants: status %d", resp.StatusCode))
	case resp.StatusCode != http.StatusOK:
		return nil, fail(contracts.Permanent, "http_status", fmt.Sprintf("jquants: status %d", resp.StatusCode))
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fail(contracts.Retryable, "network", "jquants: reading response failed")
	}
	return parseBody(raw)
}
