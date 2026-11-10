package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/google/uuid"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/valuation.md#valuation-wire-us (Decision 3).

func init() { register("approve-band", approveBand) }

const (
	bandName     = "valuation.buy_band"
	approvalName = "valuation.band_approval"
	// approveLookbackDays is how far before the approval day the current band is looked for; the page looks
	// 400 days back for the approval (companies_page approvalLookbackDays).
	approveLookbackDays = 400
)

// approveBand is `ot approve-band --symbol S [--day D]`: it reads the current valuation.buy_band of S (the newest
// row on or before D, which must be ok with a value) and writes one valuation.band_approval reading: origin
// owner_command, day D, value the 8% value of the band, the next rev of that day. A symbol with no band is
// refused and nothing is written. Prints one line: `approved SYMBOL DAY VALUE rev N`.
func approveBand(a *app, args []string) error {
	fs := flag.NewFlagSet("approve-band", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	symbol := fs.String("symbol", "", "symbol of the company (as in watch.toml)")
	dayFlag := fs.String("day", "", "approval day YYYY-MM-DD (default: today in "+marketZone+")")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("approve-band: unexpected argument %q", fs.Arg(0))
	}
	if strings.TrimSpace(*symbol) == "" {
		return errors.New("approve-band: --symbol is required")
	}
	if a.profile.Name == "demo" {
		return errors.New("approve-band needs a real profile; the demo profile opens no store")
	}
	clk := a.deps.newClock()
	day, err := dayOrDefault(clk, *dayFlag)
	if err != nil {
		return err
	}
	env, err := a.envFile()
	if err != nil {
		return err
	}
	dsn := env[store.DSNEnvReadings]
	if dsn == "" {
		return fmt.Errorf("%s is missing in .env", store.DSNEnvReadings)
	}
	ctx := context.Background()
	rd, err := a.deps.openReading(ctx, dsn)
	if err != nil {
		return fmt.Errorf("open readings store with %s failed", store.DSNEnvReadings)
	}
	defer func() { _ = rd.Close() }()
	rec, err := approvalOf(ctx, a, rd, *symbol, day, clk.Now())
	if err != nil {
		return err
	}
	if _, err := rd.InsertReadings(ctx, []contracts.ReadingRecord{rec}); err != nil {
		return fmt.Errorf("approve-band: write failed: %w", err)
	}
	_, _ = fmt.Fprintf(a.stdout, "approved %s %s %s rev %d\n", rec.Subject, day, *rec.Value, rec.Rev)
	return nil
}

// approvalOf builds the approval reading of symbol on day from the current band in rd.
func approvalOf(ctx context.Context, a *app, rd store.ReadingStore, symbol string, day contracts.Day, now time.Time) (contracts.ReadingRecord, error) {
	t, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return contracts.ReadingRecord{}, fmt.Errorf("--day %q is not YYYY-MM-DD", day)
	}
	from := contracts.Day(t.AddDate(0, 0, -approveLookbackDays).Format(dayLayout))
	bands, err := rd.LatestReadings(ctx, bandName, "underlying", symbol, from, day)
	if err != nil {
		return contracts.ReadingRecord{}, errors.New("approve-band: the band cannot be read")
	}
	if len(bands) == 0 || bands[len(bands)-1].State != gen.ReadingRecordStateOk || bands[len(bands)-1].Value == nil {
		return contracts.ReadingRecord{}, fmt.Errorf("approve-band: %s has no band on or before %s; nothing written", symbol, day)
	}
	band := bands[len(bands)-1]
	rule, ok := a.deps.computeRules.Rule(approvalName)
	if !ok {
		return contracts.ReadingRecord{}, fmt.Errorf("approve-band: no rule %q", approvalName)
	}
	prev, err := rd.LatestReadings(ctx, approvalName, "underlying", symbol, day, day)
	if err != nil {
		return contracts.ReadingRecord{}, errors.New("approve-band: the approvals cannot be read")
	}
	rev := 1
	if n := len(prev); n > 0 {
		rev = prev[n-1].Rev + 1
	}
	value := *band.Value
	sum := sha256.Sum256([]byte(strings.Join([]string{bandName, symbol, band.Day.Format(dayLayout), strconv.Itoa(band.Rev), value}, "|")))
	rec := contracts.ReadingRecord{
		Name: rule.ID, Version: rule.Version, SubjectKind: "underlying", Subject: symbol,
		Day: types.SerializableDate{Time: t}, Currency: band.Currency, Origin: "owner_command", Cutoff: "session_close",
		JobRunID:   uuid.NewSHA1(uuid.NameSpaceOID, []byte(string(day)+"|approve-band|"+symbol+"|"+strconv.Itoa(rev))).String(),
		RecordedAt: now.UTC(), RowsSha256: hex.EncodeToString(sum[:]), Rev: rev,
		State: "ok", Value: &value, Unit: band.Unit,
		Inputs: []gen.ReadingRecordInputsElem{{
			InputTable: "readings.reading", Keys: []string{bandName + "|" + symbol + "|" + band.Day.Format(dayLayout)},
			CodeVersion: rule.Hash, ConventionVersion: rule.Convention,
		}},
		Parameters: []gen.ReadingRecordParametersElem{},
	}
	return rec, nil
}
