package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStructureAttributeDictionariesIgnoreKeyOrder(t *testing.T) {
	for _, tc := range []struct {
		name, actual string
		differences  int
	}{
		{"same", `{"BBox":[51,133.539,524.45,269.689],"Nested":{"List":[{"B":2,"A":1}],"Owner":"Layout"},"O":"Layout","Placement":"Block"}`, 0},
		{"changed", `{"BBox":[51,133.539,524.45,270],"Nested":{"List":[{"B":2,"A":1}],"Owner":"Layout"},"O":"Layout","Placement":"Block"}`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expected := json.NewDecoder(strings.NewReader(`{"attributes":{"O":"Layout","Placement":"Block","Nested":{"Owner":"Layout","List":[{"A":1,"B":2}]},"BBox":[51,133.539,524.45,269.689]},"children":[]}`))
			actual := json.NewDecoder(strings.NewReader(`{"attributes":` + tc.actual + `,"children":[]}`))
			expected.UseNumber()
			actual.UseNumber()
			var differences []string
			if err := compareJSONStreamValue(expected, actual, "structure[0].children[16]", 1e-6, &differences); err != nil {
				t.Fatal(err)
			}
			if len(differences) != tc.differences {
				t.Fatalf("differences=%v, want %d", differences, tc.differences)
			}
		})
	}
}
