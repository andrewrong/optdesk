package main

import (
	"reflect"
	"testing"
)

func TestNewDepsFilled(t *testing.T) {
	t.Parallel()
	v := reflect.ValueOf(newDeps())
	if v.NumField() != 14 {
		t.Fatalf("deps has %d fields, want 14", v.NumField())
	}
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).IsNil() {
			t.Errorf("field %s is nil", v.Type().Field(i).Name)
		}
	}
}
