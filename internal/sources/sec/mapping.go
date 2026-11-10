package sec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// kept lists the concepts of Decision 6 by taxonomy; a concept not listed is dropped.
var kept = map[string]map[string]bool{
	"us-gaap": setOf(
		"NetCashProvidedByUsedInOperatingActivities",
		"NetCashProvidedByUsedInOperatingActivitiesContinuingOperations",
		"PaymentsToAcquirePropertyPlantAndEquipment",
		"PaymentsToAcquireProductiveAssets",
		"PaymentsToAcquireOtherPropertyPlantAndEquipment",
		"PaymentsForCapitalImprovements",
		"ShareBasedCompensation",
		"AllocatedShareBasedCompensationExpense",
		"WeightedAverageNumberOfDilutedSharesOutstanding",
		"WeightedAverageNumberOfShareOutstandingBasicAndDiluted",
		"LongTermDebt",
		"LongTermDebtNoncurrent",
		"Revenues",
		"RevenueFromContractWithCustomerExcludingAssessedTax",
		"RevenueFromContractWithCustomerIncludingAssessedTax",
		"SalesRevenueNet",
		"SalesRevenueGoodsNet",
		"NetIncomeLoss",
		"StockholdersEquity",
		"GrossProfit",
		"OperatingIncomeLoss",
		"IncomeLossFromContinuingOperationsBeforeIncomeTaxesExtraordinaryItemsNoncontrollingInterest",
		"IncomeTaxExpenseBenefit",
		"PropertyPlantAndEquipmentNet",
		"AccountsReceivableNetCurrent",
		"InventoryNet",
		"AccountsPayableCurrent",
		"Assets",
	),
	"dei": setOf("EntityCommonStockSharesOutstanding"),
}

func setOf(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

type companyFacts struct {
	Facts map[string]map[string]struct {
		Units map[string][]struct {
			Start *string     `json:"start"`
			End   string      `json:"end"`
			Val   json.Number `json:"val"`
			Accn  string      `json:"accn"`
			Form  string      `json:"form"`
			Filed string      `json:"filed"`
		} `json:"units"`
	} `json:"facts"`
}

func sha(body []byte) string {
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:])
}

func parseDay(s string) (types.SerializableDate, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return types.SerializableDate{}, err
	}
	return types.SerializableDate{Time: t}, nil
}

// currencyOf returns the unit when it is a three-letter money code, else empty.
func currencyOf(unit string) string {
	if len(unit) != 3 || unit != strings.ToUpper(unit) {
		return ""
	}
	return unit
}

// mapCompanyFacts maps the kept concepts of a companyfacts document whose filing day is within
// [from, to] (an empty bound is open). Facts come out in a fixed order.
func mapCompanyFacts(subject string, body []byte, retrieved time.Time, from, to contracts.Day) ([]contracts.FundamentalFact, error) {
	var doc companyFacts
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("sec: companyfacts document is not valid")
	}
	digest := sha(body)
	var out []contracts.FundamentalFact
	for _, tax := range sortedKeys(doc.Facts) {
		keep := kept[tax]
		for _, concept := range sortedKeys(doc.Facts[tax]) {
			if !keep[concept] {
				continue
			}
			units := doc.Facts[tax][concept].Units
			for _, unit := range sortedKeys(units) {
				for _, e := range units[unit] {
					if e.Accn == "" || e.Val == "" || e.Filed == "" {
						continue
					}
					if (from != "" && e.Filed < string(from)) || (to != "" && e.Filed > string(to)) {
						continue
					}
					end, err1 := parseDay(e.End)
					filed, err2 := parseDay(e.Filed)
					if err1 != nil || err2 != nil {
						return nil, fmt.Errorf("sec: companyfacts document has a bad date")
					}
					f := contracts.FundamentalFact{
						Source: "sec", Subject: subject,
						Taxonomy: tax, Concept: concept, Unit: unit, Currency: currencyOf(unit),
						PeriodEnd: end, Value: e.Val.String(), Form: e.Form, DocID: e.Accn,
						Filed: filed, RetrievedAt: retrieved, RawSha256: digest,
					}
					if e.Start != nil {
						s, err := parseDay(*e.Start)
						if err != nil {
							return nil, fmt.Errorf("sec: companyfacts document has a bad date")
						}
						f.PeriodStart = &s
					}
					out = append(out, f)
				}
			}
		}
	}
	return out, nil
}

// mapSIC keeps the SIC code of a submissions document as one fact dated the retrieval day; a document
// with no SIC gives none.
func mapSIC(subject string, body []byte, retrieved time.Time) ([]contracts.FundamentalFact, error) {
	var doc struct {
		SIC json.RawMessage `json:"sic"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("sec: submissions document is not valid")
	}
	sic := strings.Trim(strings.TrimSpace(string(doc.SIC)), `"`)
	if sic == "" || sic == "null" {
		return nil, nil
	}
	today, err := parseDay(retrieved.UTC().Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	return []contracts.FundamentalFact{{
		Source: "sec", Subject: subject,
		Taxonomy: "sec", Concept: "StandardIndustrialClassification", Unit: "sic",
		PeriodEnd: today, Value: sic, Form: "submissions",
		DocID: "submissions-" + strings.TrimPrefix(subject, "cik:"),
		Filed: today, RetrievedAt: retrieved, RawSha256: sha(body),
	}}, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
