package flex

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
)

const (
	flexHost       = "https://ndcdyn.interactivebrokers.com"
	sendRequest    = "/AccountManagement/FlexWebService/SendRequest"
	getStatement   = "/AccountManagement/FlexWebService/GetStatement"
	apiVersion     = "3"
	maxBody        = 50 << 20
	maxPolls       = 10
	errInProgress  = "1019"
	requestTimeout = 60 * time.Second
)

// The kinds of a flex error.
const (
	kindNetwork    = "network"
	kindHTTPStatus = "http_status"
	kindFlexError  = "flex_error"
	kindInProgress = "in_progress"
	kindTooLarge   = "too_large"
	kindNotReport  = "not_report"
)

// flexError is a contracts.TypedError that names the flex kind and a code; its text holds neither
// the token, the query id nor an address.
type flexError struct {
	flexKind string
	code     string
}

func (e *flexError) Error() string {
	if e.code == "" {
		return "flex: " + e.flexKind
	}
	return "flex: " + e.flexKind + " code=" + e.code
}

// Kind is Retryable for a network failure, a 5xx answer and an unfinished report, else Permanent.
func (e *flexError) Kind() contracts.ErrorKind {
	switch e.flexKind {
	case kindNetwork, kindInProgress:
		return contracts.Retryable
	case kindHTTPStatus:
		if strings.HasPrefix(e.code, "5") {
			return contracts.Retryable
		}
	}
	return contracts.Permanent
}

// FlexKind is the flex kind: network, http_status, flex_error, in_progress, too_large or not_report.
func (e *flexError) FlexKind() string { return e.flexKind }

// Code is the HTTP status or the Flex error code, or empty.
func (e *flexError) Code() string { return e.code }

func failCode(kind, code string) error { return &flexError{flexKind: kind, code: code} }

type client struct {
	token string
	wait  func(ctx context.Context, d time.Duration) error
	clk   clock.Clock
	pacer *pacer
	http  *http.Client
}

type option func(*client)

// withTransport injects the HTTP transport; tests pass a fake and open no port.
func withTransport(rt http.RoundTripper) option {
	return func(c *client) { c.http.Transport = rt }
}

// withPacer replaces the process-wide pacer.
func withPacer(p *pacer) option { return func(c *client) { c.pacer = p } }

var _ sources.NewFlexFunc = New

// New builds the Flex source. Token, Wait and Clock are required; no environment is read.
func New(cfg sources.FlexConfig) (sources.FlexSource, error) { return newClient(cfg) }

func newClient(cfg sources.FlexConfig, opts ...option) (*client, error) {
	if cfg.Token == "" || cfg.Wait == nil || cfg.Clock == nil {
		return nil, sources.ErrFlexNotConfigured
	}
	c := &client{
		token: cfg.Token, wait: cfg.Wait, clk: cfg.Clock, pacer: sharedPacer,
		http: &http.Client{
			Timeout:       requestTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
	for _, o := range opts {
		o(c)
	}
	return c, nil
}

// Fetch asks for the report of the window (SendRequest) and then polls GetStatement by reference
// code, only while the service answers error 1019, at most 10 times.
func (c *client) Fetch(ctx context.Context, req sources.FlexRequest) (sources.FlexReport, error) {
	q := url.Values{}
	q.Set("t", c.token)
	q.Set("q", req.QueryID)
	q.Set("v", apiVersion)
	if req.First != "" {
		q.Set("fd", compact(string(req.First)))
	}
	if req.Last != "" {
		q.Set("td", compact(string(req.Last)))
	}
	body, err := c.get(ctx, sendRequest, q)
	if err != nil {
		return sources.FlexReport{}, err
	}
	sr, err := parseStatusResponse(body)
	if err != nil {
		return sources.FlexReport{}, err
	}
	if sr.Status != "Success" || sr.ReferenceCode == "" {
		return sources.FlexReport{}, failCode(kindFlexError, sr.ErrorCode)
	}
	pq := url.Values{}
	pq.Set("t", c.token)
	pq.Set("q", sr.ReferenceCode)
	pq.Set("v", apiVersion)
	for i := 0; i < maxPolls; i++ {
		body, err := c.get(ctx, getStatement, pq)
		if err != nil {
			return sources.FlexReport{}, err
		}
		root, err := rootName(body)
		if err != nil {
			return sources.FlexReport{}, err
		}
		if root == "FlexStatementResponse" {
			st, err := parseStatusResponse(body)
			if err != nil {
				return sources.FlexReport{}, err
			}
			if st.ErrorCode == errInProgress {
				continue
			}
			return sources.FlexReport{}, failCode(kindFlexError, st.ErrorCode)
		}
		stmts, err := parseReport(body)
		if err != nil {
			return sources.FlexReport{}, err
		}
		sum := sha256.Sum256(body)
		return sources.FlexReport{FetchedAt: c.clk.Now(), RawSHA256: hex.EncodeToString(sum[:]), Statements: stmts}, nil
	}
	return sources.FlexReport{}, failCode(kindInProgress, errInProgress)
}

func compact(day string) string { return strings.ReplaceAll(day, "-", "") }

// get makes one paced GET of a constant path and returns the body, capped at 50 MB.
func (c *client) get(ctx context.Context, path string, q url.Values) ([]byte, error) {
	if d := c.pacer.reserve(c.clk.Now()); d > 0 {
		if err := c.wait(ctx, d); err != nil {
			return nil, failCode(kindNetwork, "")
		}
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, flexHost+path+"?"+q.Encode(), nil)
	if err != nil {
		return nil, failCode(kindNetwork, "")
	}
	resp, err := c.http.Do(r)
	if err != nil {
		// The cause may hold the address or the token: it is dropped.
		return nil, failCode(kindNetwork, "")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, failCode(kindHTTPStatus, fmt.Sprint(resp.StatusCode))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, failCode(kindNetwork, "")
	}
	if len(body) > maxBody {
		return nil, failCode(kindTooLarge, "")
	}
	return body, nil
}
