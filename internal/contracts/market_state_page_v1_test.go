package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const marketStatePageV1 = "page.market_state_page.v1.json"

func marketStateFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "contracts", "fixtures", "page.market_state_page.v1."+name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func msRows(m map[string]any) []any {
	return m["market"].(map[string]any)["rows"].([]any)
}

func msSectors(m map[string]any) map[string]any { return m["sectors"].(map[string]any) }

func TestMarketStatePageValid(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"ok", "gap"} {
		if !pageV1Valid(t, marketStatePageV1, marketStateFixture(t, name)) {
			t.Fatalf("fixture %s is invalid", name)
		}
	}
	inst := marketStateFixture(t, "ok")
	inst["extra"] = 1
	if pageV1Valid(t, marketStatePageV1, inst) {
		t.Fatal("an unknown page field validates")
	}
}

func TestMSPageRejectsBadRowId(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"nasdaq", "VIX", ""} {
		inst := marketStateFixture(t, "ok")
		msRows(inst)[0].(map[string]any)["id"] = id
		if pageV1Valid(t, marketStatePageV1, inst) {
			t.Fatalf("row id %q validates", id)
		}
	}
	inst := marketStateFixture(t, "ok")
	msRows(inst)[0].(map[string]any)["state"] = "calm"
	if pageV1Valid(t, marketStatePageV1, inst) {
		t.Fatal("a row with a state field validates")
	}
}

func TestMSPageRequiresWindowDay(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"window", "day", "history"} {
		inst := marketStateFixture(t, "ok")
		delete(msRows(inst)[0].(map[string]any), key)
		if pageV1Valid(t, marketStatePageV1, inst) {
			t.Fatalf("a row without %s validates", key)
		}
	}
	inst := marketStateFixture(t, "ok")
	row := msRows(inst)[0].(map[string]any)
	delete(row, "percentile")
	if !pageV1Valid(t, marketStatePageV1, inst) {
		t.Fatal("a row without percentile is invalid")
	}
}

func TestMSPageRequiresEndDayCaption(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"end_day", "caption", "bars_caption", "rotation_axes"} {
		inst := marketStateFixture(t, "ok")
		delete(msSectors(inst), key)
		if pageV1Valid(t, marketStatePageV1, inst) {
			t.Fatalf("a sector block without %s validates", key)
		}
	}
	inst := marketStateFixture(t, "ok")
	msSectors(inst)["end_day"] = pageV1Gap()
	if !pageV1Valid(t, marketStatePageV1, inst) {
		t.Fatal("an end_day that is a Gap is invalid")
	}
}

func TestMSHistoryRequiresCaption(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"caption", "aria_text", "unit", "points"} {
		inst := marketStateFixture(t, "ok")
		delete(msRows(inst)[0].(map[string]any)["history"].(map[string]any), key)
		if pageV1Valid(t, marketStatePageV1, inst) {
			t.Fatalf("a history without %s validates", key)
		}
	}
	inst := marketStateFixture(t, "ok")
	h := msRows(inst)[0].(map[string]any)["history"].(map[string]any)
	pt := h["points"].([]any)[0]
	pts := make([]any, 801)
	for i := range pts {
		pts[i] = pt
	}
	h["points"] = pts
	if pageV1Valid(t, marketStatePageV1, inst) {
		t.Fatal("a history of 801 points validates")
	}
}

func TestMarketStateRotationShape(t *testing.T) {
	t.Parallel()
	rows := msSectors(marketStateFixture(t, "ok"))["rows"].([]any)
	lengths := map[int]int{}
	for _, r := range rows {
		rot, ok := r.(map[string]any)["rotation"].([]any)
		if !ok {
			continue
		}
		lengths[len(rot)]++
		seen := map[[2]float64]bool{}
		for _, p := range rot {
			pm := p.(map[string]any)
			if !strings.HasPrefix(pm["text"].(string), pm["day"].(string)) {
				t.Fatalf("text %q does not start with its day %v", pm["text"], pm["day"])
			}
			xy := [2]float64{pm["x"].(float64), pm["y"].(float64)}
			if seen[xy] {
				t.Fatalf("value %v repeats", xy)
			}
			seen[xy] = true
		}
	}
	if lengths[5] == 0 || lengths[4] != 1 {
		t.Fatalf("rotation lengths %v, want 5-point rows and exactly one 4-point row", lengths)
	}
	inst := marketStateFixture(t, "ok")
	row := msSectors(inst)["rows"].([]any)[0].(map[string]any)
	rot := row["rotation"].([]any)
	row["rotation"] = append(rot, rot[0])
	if pageV1Valid(t, marketStatePageV1, inst) {
		t.Fatal("a rotation of 6 points validates")
	}
	row["rotation"] = []any{}
	if pageV1Valid(t, marketStatePageV1, inst) {
		t.Fatal("an empty rotation validates")
	}
	row["rotation"] = pageV1Gap()
	if !pageV1Valid(t, marketStatePageV1, inst) {
		t.Fatal("a rotation that is a Gap is invalid")
	}
}

func TestMSRequiresFourQuadrants(t *testing.T) {
	t.Parallel()
	axes := func(inst map[string]any) map[string]any { return msSectors(inst)["rotation_axes"].(map[string]any) }
	inst := marketStateFixture(t, "ok")
	q := axes(inst)["quadrants"].([]any)
	if len(q) != 4 {
		t.Fatalf("fixture has %d quadrants", len(q))
	}
	axes(inst)["quadrants"] = q[:3]
	if pageV1Valid(t, marketStatePageV1, inst) {
		t.Fatal("three quadrants validate")
	}
	inst = marketStateFixture(t, "ok")
	axes(inst)["quadrants"] = append(q, q[0])
	if pageV1Valid(t, marketStatePageV1, inst) {
		t.Fatal("five quadrants validate")
	}
	inst = marketStateFixture(t, "ok")
	q = axes(inst)["quadrants"].([]any)
	q[0].(map[string]any)["id"] = "neutral"
	if pageV1Valid(t, marketStatePageV1, inst) {
		t.Fatal("an unknown quadrant id validates")
	}
	inst = marketStateFixture(t, "ok")
	delete(axes(inst), "note")
	if pageV1Valid(t, marketStatePageV1, inst) {
		t.Fatal("axes without a note validate")
	}
}

func msIsGap(v any) bool {
	m, ok := v.(map[string]any)
	return ok && m["kind"] == "gap"
}

func checkMSMarketRow(t *testing.T, row map[string]any, id string) {
	t.Helper()
	if row["id"] != id {
		t.Fatalf("row id %v, want %s", row["id"], id)
	}
	if row["window"] == "" || row["day"] == nil {
		t.Fatalf("row %s lacks window or day", id)
	}
	if _, has := row["percentile"]; has != (id == "vix" || id == "hy_spread") {
		t.Fatalf("row %s percentile presence %v", id, has)
	}
	h := row["history"]
	if id == "vix_term" {
		if g, _ := h.(map[string]any); !msIsGap(h) || g["reason"] != "no_data" {
			t.Fatalf("vix_term history %v, want a no_data Gap", h)
		}
		return
	}
	hm := h.(map[string]any)
	if len(hm["points"].([]any)) < 3 || hm["caption"] == "" {
		t.Fatalf("row %s history has fewer than 3 points or no caption", id)
	}
}

func rotationDays(row map[string]any) string {
	days := []string{}
	for _, p := range row["rotation"].([]any) {
		days = append(days, p.(map[string]any)["day"].(string))
	}
	return strings.Join(days, ",")
}

// checkMSSectorRow checks one fund row and returns its kind: "gap", "full" or "short".
func checkMSSectorRow(t *testing.T, row map[string]any) string {
	t.Helper()
	if msIsGap(row["gap_21"]) {
		if row["rank_63"].(map[string]any)["reason"] != "missing_input" || !msIsGap(row["rotation"]) {
			t.Fatalf("gap row %v: rank %v rotation %v", row["symbol"], row["rank_63"], row["rotation"])
		}
		return "gap"
	}
	for _, k := range []string{"rank_21", "rank_63"} {
		if of := row[k].(map[string]any)["of"]; of != 10.0 {
			t.Fatalf("row %v %s of %v, want 10", row["symbol"], k, of)
		}
	}
	switch n := len(row["rotation"].([]any)); n {
	case 5:
		return "full"
	case 4:
		if got := rotationDays(row); got != "2026-10-16,2026-10-30,2026-11-06,2026-11-13" {
			t.Fatalf("4-point rotation days %v", got)
		}
		return "short"
	default:
		t.Fatalf("rotation of %d points", n)
	}
	return ""
}

func TestMSOkFixtureShape(t *testing.T) {
	t.Parallel()
	ok := marketStateFixture(t, "ok")
	rows := msRows(ok)
	wantIDs := []string{"sp500_vs_200d", "sp500_drawdown", "vix", "vix_term", "hy_spread"}
	if len(rows) != len(wantIDs) {
		t.Fatalf("%d market rows", len(rows))
	}
	for i, r := range rows {
		checkMSMarketRow(t, r.(map[string]any), wantIDs[i])
	}
	srows := msSectors(ok)["rows"].([]any)
	if len(srows) != 12 {
		t.Fatalf("%d sector rows", len(srows))
	}
	kinds := map[string]int{}
	for _, r := range srows {
		row := r.(map[string]any)
		if row["group"] == "industry" {
			if row["symbol"] != "SMH" || len(row["rotation"].([]any)) != 5 {
				t.Fatalf("industry row %v", row["symbol"])
			}
			continue
		}
		kinds[checkMSSectorRow(t, row)]++
	}
	if kinds["gap"] != 1 || kinds["short"] != 1 || kinds["full"] != 9 {
		t.Fatalf("row kinds %v, want 1 gap, 1 short and 9 full", kinds)
	}
}

func TestMSGapFixtureEveryFigureGap(t *testing.T) {
	t.Parallel()
	g := marketStateFixture(t, "gap")
	for _, r := range msRows(g) {
		row := r.(map[string]any)
		if !msIsGap(row["figure"]) || !msIsGap(row["day"]) || !msIsGap(row["history"]) {
			t.Fatalf("row %v has a figure that is not a Gap", row["id"])
		}
	}
	s := msSectors(g)
	if !msIsGap(s["end_day"]) {
		t.Fatal("end_day is not a Gap")
	}
	for _, r := range s["rows"].([]any) {
		row := r.(map[string]any)
		for _, k := range []string{"gap_21", "gap_63", "rank_21", "rank_63", "rotation"} {
			if !msIsGap(row[k]) {
				t.Fatalf("row %v %s is not a Gap", row["symbol"], k)
			}
		}
	}
}

func TestMSSectorGroupEnum(t *testing.T) {
	t.Parallel()
	inst := marketStateFixture(t, "ok")
	msSectors(inst)["rows"].([]any)[0].(map[string]any)["group"] = "proxy"
	if pageV1Valid(t, marketStatePageV1, inst) {
		t.Fatal("an unknown group validates")
	}
}
