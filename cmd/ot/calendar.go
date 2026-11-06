package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

func init() { register("calendar", runCalendar) }

// calendarProvenance says where the rows came from; the loader invents none of it.
type calendarProvenance struct {
	Generator            string `json:"generator"`
	GeneratorVersion     string `json:"generator_version"`
	SourceLibrary        string `json:"source_library"`
	SourceLibraryVersion string `json:"source_library_version"`
	GeneratedAt          string `json:"generated_at"`
}

// calendarFile is the file read by ot calendar load.
type calendarFile struct {
	Provenance *calendarProvenance    `json:"provenance"`
	Rows       []contracts.SessionDay `json:"rows"`
}

func runCalendar(a *app, args []string) error {
	if len(args) < 2 || args[0] != "load" {
		return fmt.Errorf("usage: ot calendar load FILE [FILE ...]")
	}
	if a.profile.Name == "demo" {
		return fmt.Errorf("calendar load needs a real profile; the demo profile opens no store")
	}
	files := make([][]contracts.SessionDay, 0, len(args)-1)
	for _, path := range args[1:] {
		rows, err := readCalendarFile(path)
		if err != nil {
			return err
		}
		files = append(files, rows)
	}
	env, err := a.envFile()
	if err != nil {
		return err
	}
	dsn := env[store.DSNEnvLoader]
	if dsn == "" {
		return fmt.Errorf("%s is missing in .env; ot setup pg creates it", store.DSNEnvLoader)
	}
	ctx := context.Background()
	sd, err := a.deps.openSessionDay(ctx, dsn)
	if err != nil {
		return fmt.Errorf("open session-day store with %s failed; the error text is withheld because it may hold %s", store.DSNEnvLoader, store.DSNEnvLoader)
	}
	defer func() { _ = sd.Close() }()
	for i, rows := range files {
		if err := sd.UpsertSessionDays(ctx, rows); err != nil {
			return fmt.Errorf("load %s failed; the error text is withheld because it may hold %s", args[1+i], store.DSNEnvLoader)
		}
		a.log.Info("calendar "+args[1+i]+" loaded", "component", "ot", "rows", len(rows), "status", "ok")
	}
	return nil
}

// readCalendarFile reads, decodes and checks one file; its errors name the path and no row value.
func readCalendarFile(path string) ([]contracts.SessionDay, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("calendar file %s: cannot be read", path)
	}
	var f calendarFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("calendar file %s: not a valid calendar JSON object", path)
	}
	if f.Provenance == nil {
		return nil, fmt.Errorf("calendar file %s: missing key provenance", path)
	}
	if err := checkProvenance(f.Provenance); err != nil {
		return nil, fmt.Errorf("calendar file %s: %w", path, err)
	}
	if len(f.Rows) == 0 {
		return nil, fmt.Errorf("calendar file %s: rows is empty", path)
	}
	if err := checkRows(f.Rows); err != nil {
		return nil, fmt.Errorf("calendar file %s: %w", path, err)
	}
	return f.Rows, nil
}

func checkProvenance(p *calendarProvenance) error {
	for _, kv := range []struct{ key, val string }{
		{"generator", p.Generator},
		{"generator_version", p.GeneratorVersion},
		{"source_library", p.SourceLibrary},
		{"source_library_version", p.SourceLibraryVersion},
		{"generated_at", p.GeneratedAt},
	} {
		if kv.val == "" {
			return fmt.Errorf("provenance.%s is empty", kv.key)
		}
	}
	if _, err := time.Parse(time.RFC3339, p.GeneratedAt); err != nil {
		return fmt.Errorf("provenance.generated_at is not RFC 3339")
	}
	return nil
}

// checkRows enforces the row rules; an error names the row index and the rule, never a value.
func checkRows(rows []contracts.SessionDay) error {
	last := map[string]string{}
	for i, r := range rows {
		if r.CalendarID == "" {
			return fmt.Errorf("row %d: empty calendar_id", i)
		}
		day := r.SessionDay.Format(dayLayout)
		if prev, ok := last[r.CalendarID]; ok && day <= prev {
			return fmt.Errorf("row %d: session_day is not strictly ascending within its calendar", i)
		}
		last[r.CalendarID] = day
		if r.IsSession {
			if !r.OpenAt.Before(r.CloseAt) {
				return fmt.Errorf("row %d: a session day needs open_at before close_at", i)
			}
			continue
		}
		if !r.OpenAt.Equal(r.CloseAt) {
			return fmt.Errorf("row %d: a day that is not a session needs open_at equal to close_at", i)
		}
		if r.EarlyClose {
			return fmt.Errorf("row %d: a day that is not a session cannot be an early close", i)
		}
	}
	return nil
}
