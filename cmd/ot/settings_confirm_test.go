package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/settingsform"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
)

// storedOwner is a fakeOwnerStore that holds stored param and exclusion rows.
type storedOwner struct {
	fakeOwnerStore
	params []ownerset.Param
	excl   []ownerset.Exclusion
}

func (s *storedOwner) Params(context.Context, time.Time) ([]ownerset.Param, error) {
	return s.params, nil
}

func (s *storedOwner) Exclusions(context.Context, time.Time) ([]ownerset.Exclusion, error) {
	return s.excl, nil
}

func storedSite(t *testing.T, st *storedOwner) http.Handler {
	t.Helper()
	a := settingsApp(t)
	form := settingsform.New(nil, ownerTokenTTL, a.deps.newRuleParams())
	rd := siteReaders{owner: st, catalog: demoOption{}, names: demoOption{}, edit: &ownerEdit{form: form, writer: st, token: postKey}}
	return siteHandler(a, fakeClock{postNow}, "", rd, echoHandler, http.NotFoundHandler())
}

type previewPayload struct {
	Confirm *struct {
		Form struct {
			PostPath string `json:"post_path"`
			Expires  string `json:"expires"`
			MAC      string `json:"mac"`
		} `json:"form"`
	} `json:"confirm"`
	Params []struct{ Key, Value string } `json:"params"`
	Rows   []struct {
		InstrumentID int64 `json:"instrument_id"`
		Excluded     bool  `json:"excluded"`
	} `json:"rows"`
}

// secondPost builds the confirm post from the payload only, as SettingsPage.tsx does.
func secondPost(t *testing.T, p previewPayload) *http.Request {
	t.Helper()
	if p.Confirm == nil {
		t.Fatal("no confirm in the payload")
	}
	v := url.Values{"expires": {p.Confirm.Form.Expires}, "mac": {p.Confirm.Form.MAC}, "confirm": {"1"}}
	for _, q := range p.Params {
		v.Set("param."+q.Key, q.Value)
	}
	for _, r := range p.Rows {
		v.Add("row", strconv.FormatInt(r.InstrumentID, 10))
	}
	for _, r := range p.Rows {
		if r.Excluded {
			v.Add("excluded", strconv.FormatInt(r.InstrumentID, 10))
		}
	}
	r := postRequest(t, v)
	r.URL.Path = p.Confirm.Form.PostPath
	return r
}

func TestPostSettingsConfirmRoundTripFromPayload(t *testing.T) {
	t.Parallel()
	a, b := strconv.FormatInt(demoOptionIDs[0], 10), strconv.FormatInt(demoOptionIDs[1], 10)
	cases := []struct {
		name  string
		store *storedOwner
		first url.Values
		want  ownerset.Change
		saved string
	}{
		{name: "integer param", first: url.Values{"param.min_dte": {"30"}},
			want:  ownerset.Change{Params: []ownerset.ParamValue{{Key: "min_dte", Value: "30"}}},
			saved: "1"},
		{name: "decimal param", first: url.Values{"param.max_spread": {"0.2"}},
			want:  ownerset.Change{Params: []ownerset.ParamValue{{Key: "max_spread", Value: "0.2"}}},
			saved: "1"},
		{name: "emptied param", store: &storedOwner{params: []ownerset.Param{{Key: "min_dte", Value: "30"}}},
			first: url.Values{"param.min_dte": {""}},
			want:  ownerset.Change{Params: []ownerset.ParamValue{{Key: "min_dte", Value: ""}}}, saved: "1"},
		{name: "row excluded", first: url.Values{"row": {a}, "excluded": {a}},
			want:  ownerset.Change{Exclusions: []ownerset.ExclusionValue{{InstrumentID: demoOptionIDs[0], Excluded: true}}},
			saved: "1"},
		{name: "row un-excluded", store: &storedOwner{excl: []ownerset.Exclusion{{InstrumentID: demoOptionIDs[0], Excluded: true}}},
			first: url.Values{"row": {a}},
			want:  ownerset.Change{Exclusions: []ownerset.ExclusionValue{{InstrumentID: demoOptionIDs[0]}}}, saved: "1"},
		{name: "two rows one excluded", first: url.Values{"row": {a, b}, "excluded": {b}},
			want:  ownerset.Change{Exclusions: []ownerset.ExclusionValue{{InstrumentID: demoOptionIDs[1], Excluded: true}}},
			saved: "1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			st := c.store
			if st == nil {
				st = &storedOwner{}
			}
			h := storedSite(t, st)
			rec := serve(h, postRequest(t, c.first))
			if rec.Code != http.StatusOK {
				t.Fatalf("first post: %d %.300q", rec.Code, rec.Body)
			}
			var p previewPayload
			if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
				t.Fatal(err)
			}
			rec = serve(h, secondPost(t, p))
			if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/settings?saved="+c.saved {
				t.Fatalf("second post: %d %q", rec.Code, rec.Header().Get("Location"))
			}
			if st.calls() != 1 || !reflect.DeepEqual(st.applied[0], c.want) {
				t.Fatalf("applied %+v, want one apply of %+v", st.applied, c.want)
			}
		})
	}
}

func TestPostSettingsPreviewShowsPostedValues(t *testing.T) {
	t.Parallel()
	st := &storedOwner{params: []ownerset.Param{{Key: "min_dte", Value: "21"}}}
	h := storedSite(t, st)
	rec := serve(h, postRequest(t, url.Values{"param.min_dte": {"30"}}))
	value := func(rec interface{ Bytes() []byte }) string {
		var p previewPayload
		if err := json.Unmarshal(rec.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		for _, q := range p.Params {
			if q.Key == "min_dte" {
				return q.Value
			}
		}
		t.Fatal("no min_dte param")
		return ""
	}
	if got := value(rec.Body); got != "30" {
		t.Fatalf("preview value %q, want 30", got)
	}
	if got := value(settingsGet(t, h, http.MethodGet, "/settings").Body); got != "21" {
		t.Fatalf("GET value %q, want 21", got)
	}
}
