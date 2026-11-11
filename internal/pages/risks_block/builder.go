// Package risks_block builds the risks block of report_page v2 from the alert rules. It
// evaluates the rules over the stored readings it is given, formats the results and
// computes nothing else. Every time is passed in; it reads no clock.
package risks_block

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	// maxItems and maxGaps are the maxItems of items and gaps in the schema.
	maxItems = 50
	maxGaps  = 20
	// maxDataDays is how many distinct data days data_times names; maxLength is the schema's
	// maxLength of the text fields.
	maxDataDays = 6
	maxLength   = 200

	timeLayout = "2006-01-02T15:04:05-07:00"
)

// Input is what Build needs. Read is the winner reader of the report page; Today is the day
// the rules are evaluated for; GeneratedAt is written in its own zone, the owner's.
type Input struct {
	Rules       compute.AlertRules
	Evaluate    compute.EvaluateAlertsFunc
	Read        compute.ReadingFunc
	Today       contracts.Day
	GeneratedAt time.Time
}

type gap struct {
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

type item struct {
	Kind    string `json:"kind"`
	IconKey string `json:"icon_key"`
	Level   string `json:"level"`
	Title   string `json:"title"`
	Detail  string `json:"detail"`
}

type block struct {
	Key       string `json:"key"`
	Title     string `json:"title"`
	Say       string `json:"say"`
	Sub       string `json:"sub"`
	DataTimes string `json:"data_times"`
	State     string `json:"state"`
	Gaps      []gap  `json:"gaps"`
	Items     any    `json:"items"`
}

// Build returns the JSON of RisksBlock for the stored readings: one item per fired rule and
// a gap per rule without data. An error of the reader or of the evaluation is an error of
// the call, never a gap.
func Build(ctx context.Context, in Input) ([]byte, error) {
	if in.Evaluate == nil || in.Read == nil {
		return nil, errors.New("risks_block: nil evaluate or read function")
	}
	if in.Today == "" || in.GeneratedAt.IsZero() {
		return nil, errors.New("risks_block: empty day or generated_at")
	}
	results, err := in.Evaluate(ctx, in.Rules, in.Read, in.Today)
	if err != nil {
		return nil, fmt.Errorf("risks_block: evaluate: %w", err)
	}
	items := []item{}
	gaps := []gap{}
	fired, noData := 0, 0
	newest := contracts.Day("")
	var dataDays []string
	for _, r := range results {
		if r.Day > newest {
			newest = r.Day
		}
		switch r.State {
		case "fired":
			it, err := firedItem(r)
			if err != nil {
				return nil, err
			}
			fired++
			if len(items) < maxItems {
				items = append(items, it)
			}
			if len(dataDays) == 0 || !contains(dataDays, string(r.DataDay)) {
				dataDays = append(dataDays, string(r.DataDay))
			}
		case "no_data":
			noData++
			if len(gaps) < maxGaps {
				gaps = append(gaps, gap{gapReason(r.Reason), r.Title + gapSuffix})
			}
		case "clear":
		default:
			return nil, fmt.Errorf("risks_block: rule %q has state %q", r.RuleID, r.State)
		}
	}
	b := block{
		Key: blockKey, Title: blockTitle, Sub: blockSub,
		DataTimes: dataTimes(newest, dataDays, in.GeneratedAt),
		State:     "ok", Gaps: []gap{}, Items: items,
	}
	if fired == 0 {
		b.Say = string(in.Today) + sayNone
	} else {
		b.Say = fmt.Sprintf("%s%s%d%s", in.Today, saySomePre, fired, saySomePost)
	}
	switch {
	case noData > 0 && noData == len(results):
		b.State = "gap"
		b.Gaps = gaps
		b.Items = gap{"no_data", itemsGapText}
	case noData > 0:
		b.State = "partial"
		b.Gaps = gaps
	}
	return json.Marshal(b)
}

func gapReason(reason string) string {
	switch reason {
	case "missing_input", "no_data_day":
		return "missing_input"
	case "stale_data":
		return "stale_input"
	}
	return "no_data"
}

func dataTimes(newest contracts.Day, dataDays []string, generatedAt time.Time) string {
	var sb strings.Builder
	sb.WriteString(newestReadingLabel)
	if newest == "" {
		sb.WriteString(noReadingWord)
	} else {
		sb.WriteString(string(newest))
	}
	if len(dataDays) > 0 {
		sb.WriteString(alertDataLabel)
		sb.WriteString(strings.Join(dataDays[:min(len(dataDays), maxDataDays)], ", "))
		if len(dataDays) > maxDataDays {
			sb.WriteString(moreWord)
		}
	}
	sb.WriteString(generatedLabel)
	sb.WriteString(generatedAt.Format(timeLayout))
	return sb.String()
}

func firedItem(r compute.AlertResult) (item, error) {
	if r.Level != "n" && r.Level != "w" && r.Level != "d" {
		return item{}, fmt.Errorf("risks_block: rule %q has level %q", r.RuleID, r.Level)
	}
	icon, ok := iconByFigure[r.Figure]
	if !ok {
		icon = iconVolatility
	}
	suffix := detailDataDayOpen + string(r.DataDay) + detailDataDayClose
	detail := []rune(r.Detail)
	if room := maxLength - len([]rune(suffix)); len(detail) > room {
		detail = detail[:max(room, 0)]
	}
	return item{kindMarketAlert, icon, r.Level, r.Title, string(detail) + suffix}, nil
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
