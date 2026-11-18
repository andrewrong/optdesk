package valueimport

import (
	"strings"

	"github.com/andrewrong/optdesk/internal/compute/valueimportapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const poolIn, poolRemoved = "in_pool", "removed"

type importer struct{}

// New returns the importer.
func New() valueimportapi.Importer { return importer{} }

// Plan reads the file and compares it with the stored state.
func (importer) Plan(in valueimportapi.Input) (valueimportapi.Plan, error) {
	f, err := parseFile(in.File)
	if err != nil {
		return valueimportapi.Plan{}, err
	}
	rows, err := marginRows(f.params)
	if err != nil {
		return valueimportapi.Plan{}, err
	}
	st := in.Stored
	plan := valueimportapi.Plan{
		FileSHA256: sha(in.File), AsOf: contracts.Day(f.asOf),
		R: planR(f, st), Margin: planMargin(f, rows, st), Unvalued: f.unvalued,
	}
	t := effectiveTables(plan.R, plan.Margin, st)
	profiles := map[string]valueimportapi.StoredProfile{}
	for _, p := range st.Profiles {
		profiles[p.Key] = p
	}
	current := map[string]valueimportapi.StoredCurrent{}
	for _, c := range st.Current {
		current[c.Key] = c
	}
	used := map[string]bool{}
	valued := map[string]bool{} // keys and names with a row in the file
	for _, c := range f.companies {
		row := planRow(f, c, in, plan, t, profiles, current)
		if row.Key != "" {
			used[row.Key] = true
		}
		valued[strings.ToLower(c.name)] = true
		if fl := strings.ToLower(strings.TrimSpace(c.flag)); fl != "" && fl != "ok" {
			plan.FlaggedCount++
		}
		plan.Rows = append(plan.Rows, row)
	}
	for _, e := range f.removed {
		name := e.name
		if name == "" {
			name = e.ticker
		}
		plan.RemovedNames = append(plan.RemovedNames, name)
		if row, ok := removedRow(e, in, profiles, used, valued); ok {
			used[row.Key] = true
			plan.Rows = append(plan.Rows, row)
		}
	}
	plan.NotInFile = notInFile(st, used)
	return plan, nil
}

// planRow maps and compares one company row.
func planRow(f *file, c company, in valueimportapi.Input, plan valueimportapi.Plan, t tables,
	profiles map[string]valueimportapi.StoredProfile, current map[string]valueimportapi.StoredCurrent) valueimportapi.RowPlan {
	row := valueimportapi.RowPlan{
		Ticker: c.ticker, Name: c.name, Currency: c.currency, V: c.v, P: c.p,
		VersionDate: contracts.Day(c.vVersion), ChangeMemo: changeMemo(c.note, c.qNote), ReportText: c.sourceReport,
		FileBuyTop: c.bHi, Pool: poolIn,
	}
	badPick := mapKey(&row, c, in)
	row.Symbol = symbolFor(row.RMarket, c.ticker)
	row.NeedsTick = row.Unmatched || !currencyOK(row.RMarket, c.currency)
	prof, hasProf := profiles[row.Key]
	cur, hasCur := current[row.Key]
	if hasProf && prof.Pool != "" {
		row.Pool = prof.Pool
	}
	row.NewProfile = row.Key != "" && !hasProf
	row.ReviewType = "first"
	if hasCur {
		row.ReviewType = "event"
	}
	if row.Key != "" {
		row.ReviewID = reviewID(plan.FileSHA256, row.Key)
	}
	if row.Action == valueimportapi.ActionRefused {
		return row
	}
	if badPick {
		row.Action, row.Reason = valueimportapi.ActionRefused, "the key picked is not valid or is already on the list"
		return row
	}
	if why := refusal(f, c, &row, plan.R, t, in.Calc); why != "" {
		row.Action, row.Reason = valueimportapi.ActionRefused, why
		return row
	}
	switch {
	case !hasCur:
		row.Action = valueimportapi.ActionInsert
	case decEq(cur.V, c.v) && decEq(cur.P, c.p) && (!hasProf || prof.RMarket == row.RMarket):
		row.Action = valueimportapi.ActionSame
		if !hasProf {
			row.Action = valueimportapi.ActionProfileOnly
		}
	case !laterThan(row.VersionDate, cur.VersionDate):
		row.Action = valueimportapi.ActionOlder
	default:
		row.Action = valueimportapi.ActionInsert
	}
	return row
}

// removedRow gives the profile row of a name of meta.removed: pool removed, no review. A name with
// a row in the file is never moved; a name with no list match and no profile is only listed.
func removedRow(e removedEntry, in valueimportapi.Input, profiles map[string]valueimportapi.StoredProfile,
	used, valued map[string]bool) (valueimportapi.RowPlan, bool) {
	if valued[strings.ToLower(e.name)] || valued[strings.ToLower(e.ticker)] {
		return valueimportapi.RowPlan{}, false
	}
	row := valueimportapi.RowPlan{Name: e.name, Ticker: e.ticker, Currency: strings.ToUpper(e.currency), Pool: poolRemoved}
	var prof valueimportapi.StoredProfile
	found := false
	if e.ticker != "" {
		if u, ok := match(in.Stored.Underlying, e.ticker, ""); ok {
			row.Key = u.Key
			row.RMarket, _ = listMarket(u.Market)
			prof, found = profiles[u.Key]
		}
	}
	if !found && row.Key == "" && e.name != "" {
		for _, p := range in.Stored.Profiles {
			if strings.EqualFold(p.Name, e.name) {
				prof, found, row.Key = p, true, p.Key
				break
			}
		}
	}
	if row.Key == "" || used[row.Key] {
		return valueimportapi.RowPlan{}, false
	}
	if found {
		row.Name, row.Symbol, row.Currency, row.RMarket = prof.Name, prof.Symbol, prof.Currency, prof.RMarket
		row.Action = valueimportapi.ActionProfileOnly
		if prof.Pool == poolRemoved {
			row.Action = valueimportapi.ActionSame
		}
		return row, true
	}
	if row.Name == "" {
		row.Name = e.ticker
	}
	row.Symbol = symbolFor(row.RMarket, e.ticker)
	row.NewProfile, row.Action = true, valueimportapi.ActionProfileOnly
	return row, true
}

// mapKey sets the key, the R market and the unmatched fields of a row; it reports a bad pick.
func mapKey(row *valueimportapi.RowPlan, c company, in valueimportapi.Input) (badPick bool) {
	if u, ok := match(in.Stored.Underlying, c.ticker, c.market); ok {
		row.Key = u.Key
		if rm, known := listMarket(u.Market); known {
			row.RMarket = rm
		} else {
			row.RMarket = fileRMarket(c.market)
			row.Action, row.Reason = valueimportapi.ActionRefused, "the list row has a market the import does not know"
		}
	} else {
		row.Unmatched = true
		row.ProposedKey = proposedKey(c.market, c.ticker)
		row.RMarket = fileRMarket(c.market)
		if pick, ok := in.Picks[c.ticker]; ok {
			if checkPick(pick, in.Stored.Underlying) {
				row.Key, row.RMarket = pick.Key, pick.RMarket
			} else {
				badPick = true
			}
		}
	}
	return badPick
}
