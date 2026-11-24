package optionopen

import (
	"reflect"
	"testing"
)

func TestPlanManifestParamsEqualDefaultPlanParams(t *testing.T) {
	t.Parallel()
	m, err := parseManifest(planManifest)
	if err != nil {
		t.Fatal(err)
	}
	// Every key of [params] is written in the file, so a default cannot stand in for a missing line.
	tv := reflect.ValueOf(m.Params)
	for i := 0; i < tv.NumField(); i++ {
		if f := tv.Field(i); (f.Kind() == reflect.Pointer || f.Kind() == reflect.Slice) && f.IsNil() {
			t.Errorf("job_plan.toml [params] lacks %s", tv.Type().Field(i).Tag.Get("toml"))
		}
	}
	got, want := reflect.ValueOf(m.params), reflect.ValueOf(DefaultPlanParams())
	for i := 0; i < want.NumField(); i++ {
		if !reflect.DeepEqual(got.Field(i).Interface(), want.Field(i).Interface()) {
			t.Errorf("%s: job_plan.toml has %v, DefaultPlanParams has %v", want.Type().Field(i).Name,
				got.Field(i).Interface(), want.Field(i).Interface())
		}
	}
	if m.params.CeilTotal != 2050 || m.params.CeilAroundSpot != 0 || m.params.CeilPuts != 0 || m.params.CeilCalls != 0 || m.params.CeilPerStock != 0 {
		t.Errorf("ceilings of job_plan.toml: %+v", m.params)
	}
}

func TestJPPlanManifestKeepsItsCeilingsAndNoTotal(t *testing.T) {
	t.Parallel()
	m, err := parseManifest(planManifestJP)
	if err != nil {
		t.Fatal(err)
	}
	if m.params.CeilTotal != 0 || m.params.CeilAroundSpot != 200 || m.params.CeilPuts != 250 || m.params.CeilCalls != 150 {
		t.Errorf("JP ceilings %+v", m.params)
	}
}
