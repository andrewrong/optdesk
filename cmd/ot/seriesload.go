package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

func init() { register("series-load", seriesLoad) }

// stringList is a repeatable string flag.
type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

// seriesLoad wires the history loader: it parses the flags, builds the source, the writer and the loader
// (deps.newSeriesHistory), calls Load once and prints the count fetched per series. The loop, the range
// rule and the writes are the loader's.
func seriesLoad(a *app, args []string) error {
	fs := flag.NewFlagSet("series-load", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var ids stringList
	fs.Var(&ids, "series", "FRED series id; repeatable (required)")
	from := fs.String("from", "", "first observation day YYYY-MM-DD (required)")
	to := fs.String("to", "", "last observation day YYYY-MM-DD (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(ids) == 0 {
		return fmt.Errorf("--series is required")
	}
	fromDay, err := requiredDay("--from", *from)
	if err != nil {
		return err
	}
	toDay, err := requiredDay("--to", *to)
	if err != nil {
		return err
	}
	if a.profile.Name == "demo" {
		return fmt.Errorf("series-load needs a real profile; the demo profile opens no store")
	}
	env, err := a.envFile()
	if err != nil {
		return err
	}
	dsn, key := env[store.DSNEnvSeriesFred], env["FRED_API_KEY"]
	for _, l := range []struct{ name, v string }{{store.DSNEnvSeriesFred, dsn}, {"FRED_API_KEY", key}} {
		if l.v == "" {
			return fmt.Errorf("%s is missing in .env", l.name)
		}
	}
	ctx := context.Background()
	d := a.deps
	ser, err := d.openSeries(ctx, dsn)
	if err != nil {
		return fmt.Errorf("open series store with %s failed", store.DSNEnvSeriesFred)
	}
	defer func() { _ = ser.Close() }()
	src, err := d.newFRED(sources.FREDConfig{BaseURL: fredBaseURL, APIKey: key, Clock: d.newClock()})
	if err != nil {
		return fmt.Errorf("build FRED source failed")
	}
	loader, err := d.newSeriesHistory(src, ser)
	if err != nil {
		return fmt.Errorf("build series history loader failed")
	}
	res, err := loader.Load(ctx, sources.FetchPlan{SeriesIDs: ids, From: fromDay, To: toDay})
	if err != nil {
		// The error text may carry a URL with the key: only its class is shown.
		kind := "unclassified"
		var te contracts.TypedError
		if errors.As(err, &te) {
			kind = string(te.Kind())
		}
		return fmt.Errorf("series-load failed (%s): the error text is withheld because it may hold a DSN or a key", kind)
	}
	for _, r := range res {
		if _, err := fmt.Fprintf(a.stdout, "%s fetched %d\n", r.SeriesID, r.Fetched); err != nil {
			return err
		}
	}
	return nil
}

// requiredDay parses the value of a required day flag.
func requiredDay(name, v string) (contracts.Day, error) {
	if v == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	t, err := time.Parse(dayLayout, v)
	if err != nil {
		return "", fmt.Errorf("%s %q is not YYYY-MM-DD", name, v)
	}
	return contracts.Day(t.Format(dayLayout)), nil
}
