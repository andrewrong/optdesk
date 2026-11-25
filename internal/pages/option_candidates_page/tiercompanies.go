package option_candidates_page

// tierCompany is one company of a tier: its name, its row count and the ids of its rows in that tier.
type tierCompany struct {
	Company string   `json:"company"`
	Count   int      `json:"count"`
	RowIDs  []string `json:"row_ids"`
}

// companiesOfTier groups the rows of a tier by company, one entry per company in the order of its first row.
// names maps a row's company key to the text shown; a key it lacks is shown as the key.
func companiesOfTier(rows []tierRow, names map[string]string) []tierCompany {
	out := []tierCompany{}
	at := map[string]int{}
	for _, r := range rows {
		i, ok := at[r.Company]
		if !ok {
			name := names[r.Company]
			if name == "" {
				name = r.Company
			}
			out = append(out, tierCompany{Company: name, RowIDs: []string{}})
			i = len(out) - 1
			at[r.Company] = i
		}
		out[i].Count++
		out[i].RowIDs = append(out[i].RowIDs, r.ID)
	}
	return out
}
