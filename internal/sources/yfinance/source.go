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
	batchSize  = 10
	slack      = 10 * time.Second // the command stops this long before its bound
	fixedPart  = 15 * time.Second // added to the per-pair time of a batch
	defaultPer = 15 * time.Second
)

var _ sources.NewBarSourceFunc = New

// New is sources.NewBarSourceFunc: cfg.Command is the argv prefix of the refbars command and
// cfg.PerPair the time allowed for one pair (zero means 15 seconds). It runs the real command and
// measures the time left of the context against the system time.
func New(cfg sources.BarConfig) (sources.BarSource, error) {
	if len(cfg.Command) == 0 {
		return nil, &typedError{contracts.Permanent, "no command is configured"}
	}
	return newSource(cfg, Exec, time.Until), nil
}

type source struct {
	argv    []string
	perPair time.Duration
	run     CommandFunc
	until   func(time.Time) time.Duration // time left until a deadline; a test passes a fake clock
}

func newSource(cfg sources.BarConfig, run CommandFunc, until func(time.Time) time.Duration) *source {
	per := cfg.PerPair
	if per <= 0 {
		per = defaultPer
	}
	return &source{argv: append([]string(nil), cfg.Command...), perPair: per, run: run, until: until}
}

type typedError struct {
	kind contracts.ErrorKind
	msg  string
}

func (e *typedError) Error() string             { return "yfinance: " + e.msg }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

type wirePair struct {
	Symbol   string `json:"symbol"`
	Currency string `json:"currency"`
}

type wireRequest struct {
	Pairs   []wirePair    `json:"pairs"`
	FromDay contracts.Day `json:"from_day"`
	ToDay   contracts.Day `json:"to_day"`
	// WithActions and Adjusted are left out of the JSON when false, so a plain request is the one sent before.
	WithActions bool `json:"with_actions,omitempty"`
	Adjusted    bool `json:"adjusted,omitempty"`
}

type wireBar struct {
	SessionDay contracts.Day `json:"session_day"`
	Close      string        `json:"close"`
	Open       string        `json:"open"`
	High       string        `json:"high"`
	Low        string        `json:"low"`
	Volume     string        `json:"volume"`
	AdjClose   string        `json:"adj_close,omitempty"`
}

type wireAction struct {
	Day   contracts.Day `json:"day"`
	Value string        `json:"value"`
}

type wireResult struct {
	Symbol    string       `json:"symbol"`
	Currency  string       `json:"currency"`
	ErrorCode string       `json:"error_code"`
	Bars      []wireBar    `json:"bars"`
	Dividends []wireAction `json:"dividends,omitempty"`
	Splits    []wireAction `json:"splits,omitempty"`
}

type wireAnswer struct {
	Results []wireResult `json:"results"`
}

// DailyBars asks the pairs in batches of 10, one command run per batch. It keeps the answers of earlier
// batches when a later one fails, stops after a batch with a rate_limited pair, and stops when 10 seconds
// or less of the context are left; the pairs not asked get the code of the reason. It returns an error only
// when no batch gave an answer.
func (s *source) DailyBars(ctx context.Context, req sources.BarRequest) (sources.BarResult, error) {
	res := sources.BarResult{Bars: []sources.RefBar{}, Unanswered: []sources.BarUnanswered{}}
	answered := false
	stopCode := ""
	for start := 0; start < len(req.Pairs); start += batchSize {
		batch := req.Pairs[start:min(start+batchSize, len(req.Pairs))]
		if stopCode == "" {
			left, bounded := s.timeLeft(ctx)
			if bounded && left <= slack {
				stopCode = sources.BarCodeTimeout
			}
			if ctx.Err() != nil {
				stopCode = sources.BarCodeTimeout
			}
			if stopCode == "" {
				bound := s.perPair*time.Duration(len(batch)) + fixedPart
				if bounded && left < bound {
					bound = left
				}
				bars, actions, unanswered, ok := s.askBatch(ctx, req, batch, bound)
				res.Bars = append(res.Bars, bars...)
				res.Actions = append(res.Actions, actions...)
				res.Unanswered = append(res.Unanswered, unanswered...)
				answered = answered || ok
				for _, u := range unanswered {
					if u.Code == sources.BarCodeRateLimited {
						stopCode = sources.BarCodeRateLimited
					}
				}
				continue
			}
		}
		for _, p := range batch {
			res.Unanswered = append(res.Unanswered, sources.BarUnanswered{Pair: p, Code: stopCode})
		}
	}
	if !answered && len(req.Pairs) > 0 {
		return sources.BarResult{}, &typedError{contracts.Retryable, "no batch of the command gave an answer"}
	}
	return res, nil
}

func (s *source) timeLeft(ctx context.Context) (time.Duration, bool) {
	d, ok := ctx.Deadline()
	if !ok {
		return 0, false
	}
	return s.until(d), true
}

// askBatch runs the command once; ok is true when it gave a readable answer.
func (s *source) askBatch(ctx context.Context, req sources.BarRequest, batch []sources.BarPair,
	bound time.Duration) (bars []sources.RefBar, actions []sources.RefAction, unanswered []sources.BarUnanswered, ok bool) {
	failAll := func(code string) ([]sources.RefBar, []sources.RefAction, []sources.BarUnanswered, bool) {
		out := make([]sources.BarUnanswered, 0, len(batch))
		for _, p := range batch {
			out = append(out, sources.BarUnanswered{Pair: p, Code: code})
		}
		return nil, nil, out, false
	}
	wr := wireRequest{FromDay: req.From, ToDay: req.To, WithActions: req.Actions, Adjusted: req.Adjusted, Pairs: make([]wirePair, 0, len(batch))}
	for _, p := range batch {
		wr.Pairs = append(wr.Pairs, wirePair{Symbol: p.Symbol, Currency: p.Currency})
	}
	in, err := json.Marshal(wr)
	if err != nil {
		return failAll(sources.BarCodeSourceChanged)
	}
	argv := append(append([]string(nil), s.argv...), "--deadline", strconv.Itoa(int((bound-slack)/time.Second)))
	bctx, cancel := context.WithTimeout(ctx, bound)
	defer cancel()
	stdout, code, err := s.run(bctx, argv, in)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return failAll(sources.BarCodeTimeout)
		}
		return failAll(sources.BarCodeSourceChanged)
	}
	if code != 0 {
		return failAll(sources.BarCodeSourceChanged)
	}
	var ans wireAnswer
	if err := json.Unmarshal(stdout, &ans); err != nil {
		return failAll(sources.BarCodeSourceChanged)
	}
	byPair := make(map[sources.BarPair]wireResult, len(ans.Results))
	for _, r := range ans.Results {
		byPair[sources.BarPair{Symbol: r.Symbol, Currency: r.Currency}] = r
	}
	for _, p := range batch {
		r, found := byPair[p]
		switch {
		case !found:
			unanswered = append(unanswered, sources.BarUnanswered{Pair: p, Code: sources.BarCodeSourceChanged})
		case r.ErrorCode != "":
			unanswered = append(unanswered, sources.BarUnanswered{Pair: p, Code: r.ErrorCode})
		default:
			for _, b := range r.Bars {
				bars = append(bars, sources.RefBar{Pair: p, Day: b.SessionDay, Close: b.Close,
					Open: b.Open, High: b.High, Low: b.Low, Volume: b.Volume, AdjClose: b.AdjClose})
			}
			for _, a := range r.Dividends {
				actions = append(actions, sources.RefAction{Pair: p, Day: a.Day, Kind: "dividend", Value: a.Value})
			}
			for _, a := range r.Splits {
				actions = append(actions, sources.RefAction{Pair: p, Day: a.Day, Kind: "split", Value: a.Value})
			}
		}
	}
	return bars, actions, unanswered, true
}
