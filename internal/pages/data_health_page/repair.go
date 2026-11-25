package data_health_page

import (
	"sort"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// The verdicts of a cell.
const (
	RepairNone           = "none"
	RepairButton         = "button"
	RepairNotRefetchable = "not_refetchable"
)

// The kinds of a Job.
const (
	KindAnyDay     = "any_day"
	KindTodayOnly  = "today_only"
	KindListedOnly = "listed_only"
)

const (
	slotLayout   = "15:04"
	dayLayout    = "2006-01-02"
	zoneAbbrev   = "MST"
	dueTextFirst = "due at "
)

// Job is a job of jobTable with the facts the rules read; cmd/ot fills it.
type Job struct {
	ID          string
	Kind        string // KindAnyDay, KindTodayOnly or KindListedOnly
	Zone        string // zone of "today" for a today_only job
	WhyText     string // why a listed_only job or a past day cannot be run
	Datasets    []string
	Calendar    string
	PresentOnly bool   // a past-day run stores today's source under the day (Decision 5)
	DueAt       string // "HH:MM" in Zone; "" without a slot (Decision 16)
}

// Form is the owner's edit token; nil means no owner edit.
type Form struct{ PostPath, Expires, MAC string }

// Apply sets Repair, WhyText and DueText of every cell by the one rule of Decisions 4, 5 and 16. job is nil for a
// dataset with no job; form is nil without owner edit, and then every cell is "none".
func Apply(cells []Cell, job *Job, form *Form, now time.Time, today contracts.Day) {
	for i := range cells {
		verdict(&cells[i], job, form, now, today)
	}
}

func verdict(c *Cell, job *Job, form *Form, now time.Time, today contracts.Day) {
	c.Repair, c.WhyText, c.DueText = RepairNone, "", ""
	if form == nil || c.State == "ok" || c.State == "no_value" {
		return
	}
	localDay, local, haveZone := jobToday(job, now, today)
	if c.State == "missing" || c.State == StateNotRun {
		if due, before := beforeSlot(c, job, localDay, local, haveZone); before {
			c.DueText = due
			return
		}
	}
	switch {
	case job == nil:
		c.Repair, c.WhyText = RepairNotRefetchable, lostText
	case job.Kind == KindListedOnly:
		c.Repair, c.WhyText = RepairNotRefetchable, job.WhyText
	case job.Kind == KindAnyDay && !job.PresentOnly:
		c.Repair = RepairButton
	case c.Day == localDay:
		c.Repair = RepairButton
	case job.PresentOnly || job.WhyText == "":
		c.Repair, c.WhyText = RepairNotRefetchable, lostText
	default:
		c.Repair, c.WhyText = RepairNotRefetchable, job.WhyText
	}
}

// jobToday is today's date in the zone of the job, and now in that zone; without a usable zone it is the given today.
func jobToday(job *Job, now time.Time, today contracts.Day) (contracts.Day, time.Time, bool) {
	if job == nil || job.Zone == "" {
		return today, now, false
	}
	loc, err := time.LoadLocation(job.Zone)
	if err != nil {
		return today, now, false
	}
	l := now.In(loc)
	return contracts.Day(l.Format(dayLayout)), l, true
}

// beforeSlot reports a cell of today whose job has a slot that Now has not reached, with its due text.
func beforeSlot(c *Cell, job *Job, localDay contracts.Day, local time.Time, haveZone bool) (string, bool) {
	if job == nil || job.DueAt == "" || !haveZone || c.Day != localDay {
		return "", false
	}
	slot, err := time.Parse(slotLayout, job.DueAt)
	if err != nil {
		return "", false
	}
	if local.Hour()*60+local.Minute() >= slot.Hour()*60+slot.Minute() {
		return "", false
	}
	return dueTextFirst + job.DueAt + " " + local.Format(zoneAbbrev), true
}

// GapDays returns the days of the cells in state not_run, missing or expired whose verdict is a button, oldest first.
// The page prints len of it as gap_days and the repair queues one request per day of it.
func GapDays(cells []Cell) []contracts.Day {
	days := []contracts.Day{}
	for _, c := range cells {
		if (c.State == StateNotRun || c.State == "missing" || c.State == "expired") && c.Repair == RepairButton {
			days = append(days, c.Day)
		}
	}
	sort.Slice(days, func(i, j int) bool { return days[i] < days[j] })
	return days
}

// RepairAll is "button" only when there are gap days and the form is present, otherwise "none".
func RepairAll(gaps []contracts.Day, form *Form) string {
	if form != nil && len(gaps) > 0 {
		return RepairButton
	}
	return RepairNone
}
