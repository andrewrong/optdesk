package yfinance

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
)

const (
	earningsPerSymbol = 15 * time.Second // time allowed for one symbol of a batch
	earningsFixedPart = 15 * time.Second // added to the per-symbol time of a batch
)

// earningsArgv is the command the earnings source runs. It is the one-shot refearn command of pyside, started
// from the repository root like the refbars command.
var earningsArgv = []string{"uv", "run", "--locked", "--project", "pyside", "python", "-m", "optdesk.refearn"}

var currencyOfMarket = map[string]string{"US": "USD", "JP": "JPY"}

type earningsSymbol struct {
	Symbol   string `json:"symbol"`
	Currency string `json:"currency"`
}

type earningsRequest struct {
	Symbols []earningsSymbol `json:"symbols"`
	FromDay string           `json:"from_day"`
}

type earningsResult struct {
	Symbol    string   `json:"symbol"`
	Currency  string   `json:"currency"`
	ErrorCode string   `json:"error_code"`
	Dates     []string `json:"dates"`
}

type earningsAnswer struct {
	Results []earningsResult `json:"results"`
}

// systemNow is the system time. time.Now is forbidden outside internal/clock and the arch rules keep this
// package from importing the real clock, so, as New does with time.Until, the time is read through it.
func systemNow() time.Time {
	epoch := time.Unix(0, 0)
	return epoch.Add(-time.Until(epoch))
}

type earningsSource struct {
	currency string
	symbols  []string
	run      CommandFunc
	now      func() time.Time
}

type codedError struct {
	code string
	msg  string
}

func (e *codedError) Error() string { return "yfinance earnings: " + e.msg + " (" + e.code + ")" }
func (e *codedError) Kind() contracts.ErrorKind {
	if e.code == sources.BarCodeNoData {
		return contracts.NoValue
	}
	return contracts.Retryable
}
func (e *codedError) ErrorCode() string { return e.code }

// NewEarnings builds the earnings source of market ("US" or "JP") that asks cfg.Symbols of the refearn command
// through run. The command is fixed; cfg.BaseURL, APIKey, Doer and Sleep are not used.
func NewEarnings(market string, run CommandFunc, cfg sources.EarningsConfig) (sources.EarningsSource, error) {
	cur, ok := currencyOfMarket[market]
	if !ok {
		return nil, &typedError{contracts.Permanent, "no earnings for the market"}
	}
	if run == nil {
		return nil, &typedError{contracts.Permanent, "no command runner"}
	}
	return &earningsSource{currency: cur, symbols: append([]string(nil), cfg.Symbols...), run: run, now: systemNow}, nil
}

// Fetch asks the symbols in batches of 10, one command run per batch, and stops after a batch with a
// rate_limited result. It returns one listing per date of a result without an error code. It returns an error
// only when no batch gave a result without an error code; then the code is the first one that is not no_data,
// or no_data when all are.
func (s *earningsSource) Fetch(ctx context.Context) ([]sources.EarningsListing, error) {
	out := []sources.EarningsListing{}
	answered := false
	firstCode := ""
	note := func(code string) {
		if code == "" {
			return
		}
		if firstCode == "" || (firstCode == sources.BarCodeNoData && code != sources.BarCodeNoData) {
			firstCode = code
		}
	}
	fromDay := s.now().UTC().Format("2006-01-02")
	for start := 0; start < len(s.symbols); start += batchSize {
		batch := s.symbols[start:min(start+batchSize, len(s.symbols))]
		results, code := s.askBatch(ctx, batch, fromDay)
		if code != "" {
			note(code)
			if code == sources.BarCodeTimeout && ctx.Err() != nil {
				break
			}
			continue
		}
		limited := false
		for _, r := range results {
			note(r.ErrorCode)
			switch r.ErrorCode {
			case "":
				answered = true
				for _, d := range r.Dates {
					out = append(out, sources.EarningsListing{Symbol: r.Symbol, ReportDate: contracts.Day(d)})
				}
			case sources.BarCodeRateLimited:
				limited = true
			}
		}
		if limited {
			break
		}
	}
	if !answered && len(s.symbols) > 0 {
		return nil, &codedError{firstCode, "no batch gave a date answer"}
	}
	return out, nil
}

// askBatch runs the command once. It returns one result per symbol of the batch in order, or the code of the
// failure of the whole batch.
func (s *earningsSource) askBatch(ctx context.Context, batch []string, fromDay string) ([]earningsResult, string) {
	req := earningsRequest{FromDay: fromDay, Symbols: make([]earningsSymbol, 0, len(batch))}
	for _, sym := range batch {
		req.Symbols = append(req.Symbols, earningsSymbol{Symbol: sym, Currency: s.currency})
	}
	in, err := json.Marshal(req)
	if err != nil {
		return nil, sources.BarCodeSourceChanged
	}
	bound := earningsPerSymbol*time.Duration(len(batch)) + earningsFixedPart
	argv := append(append([]string(nil), earningsArgv...), "--deadline", strconv.Itoa(int((bound-slack)/time.Second)))
	bctx, cancel := context.WithTimeout(ctx, bound)
	defer cancel()
	stdout, exit, err := s.run(bctx, argv, in)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, sources.BarCodeTimeout
		}
		return nil, sources.BarCodeSourceChanged
	}
	if exit != 0 {
		return nil, sources.BarCodeSourceChanged
	}
	var ans earningsAnswer
	if err := json.Unmarshal(stdout, &ans); err != nil {
		return nil, sources.BarCodeSourceChanged
	}
	by := make(map[string]earningsResult, len(ans.Results))
	for _, r := range ans.Results {
		by[r.Symbol] = r
	}
	results := make([]earningsResult, 0, len(batch))
	for _, sym := range batch {
		r, ok := by[sym]
		if !ok {
			r = earningsResult{Symbol: sym, ErrorCode: sources.BarCodeSourceChanged}
		}
		results = append(results, r)
	}
	return results, ""
}
