package settingsapi_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
)

type fakeParams struct{}

func (fakeParams) Specs() []settingsapi.RuleParamSpec {
	return []settingsapi.RuleParamSpec{{Key: "min_dte", Label: "Min DTE", Unit: "days", Kind: "int", Default: "7", Min: "0", Max: "90"}}
}

func (fakeParams) Normalize(key, text string) (string, error) {
	if key != "min_dte" {
		return "", errors.New("unknown key")
	}
	return strings.TrimSpace(text), nil
}

func (fakeParams) Resolve(stored map[string]string) map[string]string {
	out := map[string]string{"min_dte": "7"}
	for k, v := range stored {
		if _, ok := out[k]; ok && v != "" {
			out[k] = v
		}
	}
	return out
}

type fakeForm struct{ p settingsapi.RuleParams }

func (fakeForm) Token(now time.Time) settingsapi.FormToken {
	return settingsapi.FormToken{Expires: now.Add(time.Hour).Format(time.RFC3339), MAC: "mac"}
}

func (fakeForm) Check(r settingsapi.FormRequest, now time.Time) error {
	if r.Token.MAC != "mac" {
		return errors.New("bad token")
	}
	return nil
}

func (f fakeForm) Parse(values map[string][]string, shown []int64, cur settingsapi.FormState) (settingsapi.FormChange, error) {
	var ch settingsapi.FormChange
	for _, s := range f.p.Specs() {
		v, ok := values["param_"+s.Key]
		if !ok {
			continue
		}
		text, err := f.p.Normalize(s.Key, v[0])
		if err != nil {
			return settingsapi.FormChange{}, err
		}
		if text != cur.Params[s.Key] {
			ch.Params = append(ch.Params, settingsapi.FormField{Key: s.Key, Text: text})
		}
	}
	ch.Confirm = len(values["confirm"]) > 0 && values["confirm"][0] == "1"
	return ch, nil
}

func TestRuleParamsResolveStoredMap(t *testing.T) {
	t.Parallel()
	var ctor settingsapi.NewRuleParamsFunc = func() settingsapi.RuleParams { return fakeParams{} }
	p := ctor()
	got := p.Resolve(map[string]string{"min_dte": "14", "other": "x"})
	if got["min_dte"] != "14" || len(got) != 1 {
		t.Fatalf("Resolve = %v", got)
	}
	if got := p.Resolve(nil); got["min_dte"] != "7" {
		t.Fatalf("default = %v", got)
	}
}

func TestSettingsFormParsesPost(t *testing.T) {
	t.Parallel()
	var ctor settingsapi.NewSettingsFormFunc = func(_ []byte, _ time.Duration, p settingsapi.RuleParams) settingsapi.SettingsForm {
		return fakeForm{p: p}
	}
	now := time.Date(2026, 11, 9, 9, 0, 0, 0, time.UTC)
	f := ctor([]byte("k"), time.Hour, fakeParams{})
	tok := f.Token(now)
	if err := f.Check(settingsapi.FormRequest{Token: tok}, now); err != nil {
		t.Fatal(err)
	}
	if err := f.Check(settingsapi.FormRequest{}, now); err == nil {
		t.Fatal("empty token accepted")
	}
	cur := settingsapi.FormState{Params: map[string]string{"min_dte": "7"}, Exclusions: map[int64]bool{}}
	ch, err := f.Parse(map[string][]string{"param_min_dte": {" 14 "}, "confirm": {"1"}}, nil, cur)
	if err != nil {
		t.Fatal(err)
	}
	if len(ch.Params) != 1 || ch.Params[0] != (settingsapi.FormField{Key: "min_dte", Text: "14"}) || !ch.Confirm {
		t.Fatalf("change = %+v", ch)
	}
}

type fakeUploader struct{}

func (fakeUploader) CheckUpload(r settingsapi.FormRequest, maxBody int64, now time.Time) error {
	if !strings.HasPrefix(r.ContentType, "multipart/form-data") {
		return errors.New("content type refused")
	}
	if r.BodySize < 0 || r.BodySize > maxBody {
		return errors.New("body size refused")
	}
	return nil
}

func TestUploadCheckerAppliesCallerCap(t *testing.T) {
	t.Parallel()
	var u settingsapi.UploadChecker = fakeUploader{}
	now := time.Date(2026, 11, 9, 9, 0, 0, 0, time.UTC)
	r := settingsapi.FormRequest{ContentType: "multipart/form-data; boundary=x", BodySize: 100}
	if err := u.CheckUpload(r, 100, now); err != nil {
		t.Fatal(err)
	}
	if err := u.CheckUpload(r, 99, now); err == nil {
		t.Fatal("body over the cap accepted")
	}
	r.ContentType = "application/x-www-form-urlencoded"
	if err := u.CheckUpload(r, 100, now); err == nil {
		t.Fatal("wrong content type accepted")
	}
}
