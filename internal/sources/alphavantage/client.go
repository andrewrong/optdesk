// Package alphavantage is the Alpha Vantage EARNINGS_CALENDAR source: one GET per Fetch over an injected
// HTTP client, no retry, no redirect. The key is in the query only and never in an error.
package alphavantage

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
const maxBody = 16 << 20

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
	base string
	key  string
	doer sources.HTTPDoer
}

var _ sources.NewEarningsFunc = New

// New builds the Alpha Vantage source. BaseURL and APIKey are required; no environment is read, and
// cfg.Symbols is ignored.
func New(cfg sources.EarningsConfig) (sources.EarningsSource, error) {
	if cfg.BaseURL == "" || cfg.APIKey == "" {
		return nil, fail(contracts.Permanent, "bad_config", "alphavantage: BaseURL and APIKey are required")
	}
	d := cfg.Doer
	if d == nil {
		d = &http.Client{
			Timeout:       30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	}
	return &client{base: strings.TrimRight(cfg.BaseURL, "/"), key: cfg.APIKey, doer: d}, nil
}

// Fetch makes exactly one request and parses the calendar it returns.
func (c *client) Fetch(ctx context.Context) ([]sources.EarningsListing, error) {
	q := url.Values{}
	q.Set("function", "EARNINGS_CALENDAR")
	q.Set("horizon", "3month")
	q.Set("apikey", c.key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/query?"+q.Encode(), nil)
	if err != nil {
		return nil, fail(contracts.Permanent, "bad_request", "alphavantage: bad request")
	}
	resp, err := c.doer.Do(req)
	if err != nil {
		// The error of net/http carries the URL, and with it the key: report no text of it.
		if ctx.Err() != nil {
			return nil, &codedError{kind: contracts.Retryable, code: "network", msg: "alphavantage: request cancelled", err: ctx.Err()}
		}
		return nil, fail(contracts.Retryable, "network", "alphavantage: request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, fail(contracts.Retryable, "rate_limited", "alphavantage: status 429")
	case resp.StatusCode >= 500:
		return nil, fail(contracts.Retryable, "http_status", fmt.Sprintf("alphavantage: status %d", resp.StatusCode))
	case resp.StatusCode != http.StatusOK:
		return nil, fail(contracts.Permanent, "http_status", fmt.Sprintf("alphavantage: status %d", resp.StatusCode))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fail(contracts.Retryable, "network", "alphavantage: reading response failed")
	}
	return parseCalendar(body)
}
